// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memorymonitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"contrib.go.opencensus.io/resource/auto"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"go.uber.org/zap"
)

var _ receiver.MetricsReceiver = (*Receiver)(nil)

// Receiver is used to scrape and export memory metrics from the local host.
type Receiver struct {
	mu        sync.Mutex
	stopOnce  sync.Once
	startOnce sync.Once

	nextConsumer   consumer.MetricsConsumer
	logger         *zap.Logger
	monitor        *Monitor
	scrapeInterval time.Duration
}

const metricsSource string = "memory"

// MetricsSource returns the name of the metrics data source.
func (r *Receiver) MetricsSource() string {
	return metricsSource
}

// Start scrapes and exports memory metrics from the local host.
func (r *Receiver) Start(host component.Host) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var retErr error
	r.startOnce.Do(func() {
		rsc, err := r.detectResource()
		if err != nil {
			retErr = err
			return
		}
		r.monitor = &Monitor{
			logger:    r.logger,
			resource:  rsc,
			startTime: time.Now(),
			consumer:  r.nextConsumer,
		}
		r.monitor.start(r.scrapeInterval)
	})
	return retErr
}

// Shutdown stops and cancels the underlying memory metrics scrapers.
func (r *Receiver) Shutdown() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopOnce.Do(func() {
		r.monitor.stop()
	})
	return nil
}

func (r *Receiver) detectResource() (*resourcepb.Resource, error) {
	res, err := auto.Detect(context.Background())
	if err != nil {
		return nil, fmt.Errorf("resource detection failed, err:%v", err)
	}
	if res == nil {
		return nil, nil
	}
	rsc := &resourcepb.Resource{
		Type:   res.Type,
		Labels: make(map[string]string, len(res.Labels)),
	}
	for k, v := range res.Labels {
		rsc.Labels[k] = v
	}
	return rsc, nil
}
