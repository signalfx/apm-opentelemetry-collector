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

package kubernetes

import (
	"fmt"
	"os"
	"regexp"

	"github.com/Omnition/omnition-opentelemetry-service/processor/kubernetes/kube"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	filterOPEquals       = "equals"
	filterOPNotEquals    = "not-equals"
	filterOPExists       = "exists"
	filterOPDoesNotExist = "exist"
)

// Option ...
type Option func(*kubernetesprocessor) error

// WithPassthrough ...
func WithPassthrough() Option {
	return func(p *kubernetesprocessor) error {
		p.passthroughMode = true
		return nil
	}
}

// WithExtractMetadata ...
func WithExtractMetadata(fields ...string) Option {
	return func(p *kubernetesprocessor) error {
		for _, field := range fields {
			switch field {
			case "namespace":
				p.rules.Namespace = true
			case "podName":
				p.rules.PodName = true
			case "startTime":
				p.rules.StartTime = true
			case "deployment":
				p.rules.Deployment = true
			case "cluster":
				p.rules.Cluster = true
			case "node":
				p.rules.Node = true
			default:
				fmt.Printf("\"%s\" is not a supported metadata field", field)
			}
		}
		return nil
	}
}

/*
// WithExtractLabels ...
func WithExtractLabels(labels ...string) Option {
	return func(p *kubernetesprocessor) error {
		p.rules.Labels = labels
		return nil
	}
}
*/

// WithExtractLabels ...
func WithExtractLabels(labels ...FieldExtractConfig) Option {
	return func(p *kubernetesprocessor) error {
		labels, err := extractFieldRules("label", labels...)
		if err != nil {
			return err
		}
		p.rules.Labels = labels
		return nil
	}
}

// WithExtractAnnotations ...
func WithExtractAnnotations(annotations ...FieldExtractConfig) Option {
	return func(p *kubernetesprocessor) error {
		annotations, err := extractFieldRules("annotation", annotations...)
		if err != nil {
			return err
		}
		p.rules.Annotations = annotations
		return nil
	}
}

func extractFieldRules(fieldType string, fields ...FieldExtractConfig) ([]kube.FieldExtractionRule, error) {
	rules := []kube.FieldExtractionRule{}
	for _, a := range fields {
		name := a.Name
		if name == "" {
			name = fmt.Sprintf("k8s.%s.%s", fieldType, a.Key)
		}

		var r *regexp.Regexp
		if a.Regex != "" {
			var err error
			r, err = regexp.Compile(a.Regex)
			if err != nil {
				return rules, err
			}
			names := r.SubexpNames()
			if len(names) != 2 || names[1] != "value" {
				return rules, fmt.Errorf("regex must contain exactly one named submatch (value)")
			}
		}

		rules = append(rules, kube.FieldExtractionRule{
			Name: name, Key: a.Key, Regex: r,
		})
	}
	return rules, nil
}

// WithFilterNode ...
func WithFilterNode(node, nodeFromEnvVar string) Option {
	return func(p *kubernetesprocessor) error {
		if nodeFromEnvVar != "" {
			p.filters.Node = os.Getenv(nodeFromEnvVar)
			return nil
		}
		p.filters.Node = node
		return nil
	}
}

// WithFilterNamespace ...
func WithFilterNamespace(ns string) Option {
	return func(p *kubernetesprocessor) error {
		p.filters.Namespace = ns
		return nil
	}
}

// WithFilterLabels ...
func WithFilterLabels(filters ...FilterConfig) Option {
	return func(p *kubernetesprocessor) error {
		labels := []kube.FieldFilter{}
		for _, f := range filters {
			if f.Op == "" {
				f.Op = filterOPEquals
			}

			var op selection.Operator
			switch f.Op {
			case filterOPEquals:
				op = selection.Equals
			case filterOPNotEquals:
				op = selection.NotEquals
			case filterOPExists:
				op = selection.Exists
			case filterOPDoesNotExist:
				op = selection.DoesNotExist
			default:
				return fmt.Errorf("'%s' is not a valid label filter operation for key=%s, value=%s", f.Op, f.Key, f.Value)
			}
			labels = append(labels, kube.FieldFilter{
				Key:   f.Key,
				Value: f.Value,
				Op:    op,
			})
		}
		p.filters.Labels = labels
		return nil
	}
}

// WithFilterFields ...
func WithFilterFields(filters ...FilterConfig) Option {
	return func(p *kubernetesprocessor) error {
		fields := []kube.FieldFilter{}
		for _, f := range filters {
			if f.Op == "" {
				f.Op = filterOPEquals
			}

			var op selection.Operator
			switch f.Op {
			case filterOPEquals:
				op = selection.Equals
			case filterOPNotEquals:
				op = selection.NotEquals
			default:
				return fmt.Errorf("'%s' is not a valid field filter operation for key=%s, value=%s", f.Op, f.Key, f.Value)
			}
			fields = append(fields, kube.FieldFilter{
				Key:   f.Key,
				Value: f.Value,
				Op:    op,
			})
		}
		p.filters.Fields = fields
		return nil
	}
}
