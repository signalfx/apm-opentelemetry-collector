// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	api_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/Omnition/omnition-opentelemetry-service/processor/kubernetes/observability"
)

const (
	podNodeField            = "spec.nodeName"
	ignoreAnnotation string = "opentelemetry.io/k8s-processor/ignore"

	tagClusterName    = "k8s.cluster.name"
	tagNamespaceName  = "k8s.namespace.name"
	tagPodName        = "k8s.pod.name"
	tagNodeName       = "k8s.node.name"
	tagDeploymentName = "k8s.deployment.name"
	tagStartTime      = "k8s.startTime"
)

var (
	// TODO: move this to config with default values
	podNameIgnorePatterns = []*regexp.Regexp{
		regexp.MustCompile(`jaeger-agent`),
		regexp.MustCompile(`jaeger-collector`),
	}
	podDeleteGracePeriod = time.Second * 120
)

// FieldFilter ...
type FieldFilter struct {
	Key   string
	Value string
	Op    selection.Operator
}

// Filters ...
type Filters struct {
	Node      string
	Namespace string
	Fields    []FieldFilter
	Labels    []FieldFilter
}

// FieldExtractionRule ...
type FieldExtractionRule struct {
	Name  string
	Key   string
	Regex *regexp.Regexp
}

// ExtractionRules ...
type ExtractionRules struct {
	Deployment bool
	Namespace  bool
	PodName    bool
	Node       bool
	Cluster    bool
	StartTime  bool

	// Annotations []string
	// Labels []string
	Annotations []FieldExtractionRule
	Labels      []FieldExtractionRule
}

// Pod represents a kubernetes pod
type Pod struct {
	Name       string
	Address    string
	Attributes map[string]string
	Ignore     bool
	StartTime  *metav1.Time

	DeletedAt time.Time
}

type deleteRequest struct {
	ip   string
	name string
	ts   time.Time
}

// Client is the main interface provided by this package to a kubernetes cluster
type Client struct {
	m               sync.RWMutex
	deleteMut       sync.Mutex
	kc              *kubernetes.Clientset
	deploymentRegex *regexp.Regexp
	deleteQueue     []deleteRequest

	Namespace string
	Pods      map[string]*Pod
	Rules     ExtractionRules
	Filters   Filters
}

// New initializes a new k8s Client
func New(namespace string, rules ExtractionRules, filters Filters) (*Client, error) {
	// Extract deployment name from the pod name. Pod name is created using
	// format: [deployment-name]-[Random-String-For-ReplicaSet]-[Random-String-For-Pod]
	dRegex, err := regexp.Compile(`^(.*)-([0-9a-zA-Z]*)-([0-9a-zA-Z]*)$`)
	if err != nil {
		return nil, err
	}
	c := &Client{Namespace: namespace, Rules: rules, Filters: filters, deploymentRegex: dRegex}
	go c.deleteLoop(time.Second * 30)
	err = c.start()
	return c, err
}

func (c *Client) watch() error {
	labelSelector := labels.Everything()
	{
		for _, f := range c.Filters.Labels {
			r, err := labels.NewRequirement(f.Key, f.Op, []string{f.Value})
			if err != nil {
				return err
			}
			labelSelector = labelSelector.Add(*r)
		}
	}

	var fieldSelector fields.Selector
	{
		var selectors []fields.Selector
		for _, f := range c.Filters.Fields {
			switch f.Op {
			case selection.Equals:
				selectors = append(selectors, fields.OneTermEqualSelector(f.Key, f.Value))
			case selection.NotEquals:
				selectors = append(selectors, fields.OneTermNotEqualSelector(f.Key, f.Value))
			default:
				return fmt.Errorf("field filters don't support operator: '%s'", f.Op)
			}
		}

		if c.Filters.Node != "" {
			selectors = append(selectors, fields.OneTermEqualSelector(podNodeField, c.Filters.Node))
		}
		fieldSelector = fields.AndSelectors(selectors...)
	}

	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				opts.LabelSelector = labelSelector.String()
				opts.FieldSelector = fieldSelector.String()
				return c.kc.CoreV1().Pods(c.Filters.Namespace).List(opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				opts.LabelSelector = labelSelector.String()
				opts.FieldSelector = fieldSelector.String()
				return c.kc.CoreV1().Pods(c.Filters.Namespace).Watch(opts)
			},
		},
		&api_v1.Pod{},
		5*time.Minute,
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			observability.RecordPodAdded()
			if pod, ok := obj.(*api_v1.Pod); ok {
				c.addOrUpdatePod(pod)
			}
		},
		UpdateFunc: func(old, obj interface{}) {
			observability.RecordPodUpdated()
			if pod, ok := obj.(*api_v1.Pod); ok {
				// TODO: update or remove based on whether container is ready/unready?
				c.addOrUpdatePod(pod)
			}
		},
		DeleteFunc: func(obj interface{}) {
			observability.RecordPodDeleted()
			if pod, ok := obj.(*api_v1.Pod); ok {
				c.forgetPod(pod)
			}
		},
	})
	stopCh := make(<-chan struct{})
	go informer.Run(stopCh)
	return nil
}

func (c *Client) deleteLoop(interval time.Duration) {
	for {
		<-time.After(interval)
		// lock delete mutex
		var cutoff int
		now := time.Now()
		c.deleteMut.Lock()
		for i, d := range c.deleteQueue {
			if d.ts.Add(podDeleteGracePeriod).After(now) {
				break
			}
			cutoff = i + 1
		}
		toDelete := c.deleteQueue[:cutoff]
		c.deleteQueue = c.deleteQueue[cutoff:]
		c.deleteMut.Unlock()

		c.m.Lock()
		for _, d := range toDelete {
			if p, ok := c.Pods[d.ip]; ok {
				if p.Name == d.name {
					delete(c.Pods, d.ip)
				}
			}
		}
		c.m.Unlock()
	}
}

// GetPodByIP takes an IP address and returns the pod the IP address is associated with
func (c *Client) GetPodByIP(ip string) (*Pod, bool) {
	c.m.RLock()
	pod, ok := c.Pods[ip]
	c.m.RUnlock()
	if ok {
		if pod.Ignore {
			return nil, false
		}
		return pod, ok
	}
	observability.RecordIPLookupMiss()
	return nil, false
}

func (c *Client) start() error {
	c.Pods = map[string]*Pod{}
	kc, err := kubeClientset()
	if err != nil {
		return err
	}
	c.kc = kc
	return c.watch()
}

func (c *Client) extractPodAttributes(pod *api_v1.Pod) map[string]string {
	tags := map[string]string{}
	if c.Rules.PodName {
		tags[tagPodName] = pod.Name
	}

	if c.Rules.Namespace {
		tags[tagNamespaceName] = pod.GetNamespace()
	}

	if c.Rules.StartTime {
		ts := pod.GetCreationTimestamp()
		if !ts.IsZero() {
			tags["k8s.startTime"] = ts.String()
		}
	}

	if c.Rules.Deployment {
		parts := c.deploymentRegex.FindStringSubmatch(pod.Name)
		if len(parts) == 4 {
			tags[tagDeploymentName] = parts[1]
		}
	}

	if c.Rules.Node {
		tags[tagNodeName] = pod.Spec.NodeName
	}

	if c.Rules.Cluster {
		clusterName := pod.GetClusterName()
		if clusterName != "" {
			tags[tagClusterName] = clusterName
		}
	}

	for _, r := range c.Rules.Labels {
		if v, ok := pod.Labels[r.Key]; ok {
			tags[r.Name] = c.extractField(v, r)
		}
	}

	for _, r := range c.Rules.Annotations {
		if v, ok := pod.Annotations[r.Key]; ok {
			tags[r.Name] = c.extractField(v, r)
		}
	}
	return tags
}

func (c *Client) extractField(v string, r FieldExtractionRule) string {
	value := v
	if r.Regex != nil {
		matches := r.Regex.FindStringSubmatch(v)
		if len(matches) == 2 {
			value = matches[1]
		}
	}
	return value
}

func (c *Client) addOrUpdatePod(pod *api_v1.Pod) {
	if pod.Status.PodIP == "" {
		return
	}

	c.m.Lock()
	defer c.m.Unlock()
	// compare initial scheduled timestamp for existing pod and new pod with same IP
	// and only replace old pod if scheduled time of new pod is newer? This should fix
	// the case where scheduler has assigned the same IP to a new pod but update event for
	// the old pod came in later
	if p, ok := c.Pods[pod.Status.PodIP]; ok {
		if p.StartTime != nil && pod.Status.StartTime.Before(p.StartTime) {
			return
		}
	}
	newPod := &Pod{
		Name:      pod.Name,
		Address:   pod.Status.PodIP,
		StartTime: pod.Status.StartTime,
	}

	if c.shouldIgnorePod(pod) {
		newPod.Ignore = true
	} else {
		newPod.Attributes = c.extractPodAttributes(pod)
	}
	c.Pods[pod.Status.PodIP] = newPod
}

func (c *Client) forgetPod(pod *api_v1.Pod) {
	if pod.Status.PodIP == "" {
		return
	}
	c.m.RLock()
	p, ok := c.GetPodByIP(pod.Status.PodIP)
	c.m.RUnlock()

	if ok && p.Name == pod.Name {
		c.deleteMut.Lock()
		c.deleteQueue = append(c.deleteQueue, deleteRequest{
			ip:   pod.Status.PodIP,
			name: pod.Name,
			ts:   time.Now(),
		})
		c.deleteMut.Unlock()
	}
}

func (c *Client) shouldIgnorePod(pod *api_v1.Pod) bool {
	// Host network mode is not supported right now with IP based
	// tagging as all pods in host network get same IP addresses
	if pod.Spec.HostNetwork {
		return true
	}

	// Check if user requested the pod to be ignored through annotations
	if v, ok := pod.Annotations[ignoreAnnotation]; ok {
		if v == "true" {
			return true
		}
	}

	// Check well known names that should be ignored
	for _, rexp := range podNameIgnorePatterns {
		if rexp.MatchString(pod.Name) {
			return true
		}
	}

	return false
}
