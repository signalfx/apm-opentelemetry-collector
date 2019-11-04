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
	"runtime"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/shirou/gopsutil/mem"
	"go.uber.org/zap"

	"github.com/Omnition/omnition-opentelemetry-collector/ptypes/ptime"
)

type timeSeries struct {
	metric *metricspb.MetricDescriptor
	value  *metricspb.TimeSeries
}

// Monitor encapsulates the core logic of scraping and exporting memory metrics of the host
type Monitor struct {
	logger    *zap.Logger
	resource  *resourcepb.Resource
	startTime time.Time
	stopCh    chan struct{}

	consumer consumer.MetricsConsumer
}

// start starts the memory monitor and scrapes metrics after time interval provided in a loop
func (m *Monitor) start(interval time.Duration) {
	m.stopCh = make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.scrapeAndEmit()
			}
		}
	}()
}

// stop stops the memory monitor
func (m *Monitor) stop() {
	close(m.stopCh)
}

func (m *Monitor) scrapeAndEmit() {
	ctx := context.Background()

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		loggerFunc := m.logger.Error
		if err == context.DeadlineExceeded {
			loggerFunc = m.logger.Debug
		}
		loggerFunc("unable to collect memory time info", zap.Error(err))
		return
	}

	metrics := make([]*timeSeries, 0, len(allMetrics))
	metrics = append(
		metrics,
		&timeSeries{
			metricMemUtilization,
			m.getDoubleTimeSeries(memInfo.UsedPercent),
		},
		&timeSeries{
			metricMemUsed,
			m.getInt64TimeSeries(memInfo.Used),
		},
	)

	// windows only
	if runtime.GOOS == windowsOS {
		metrics = m.appendWindowsMetrics(metrics, memInfo)
	}

	// linux + darwin only
	if runtime.GOOS != windowsOS {
		metrics = m.appendNonWindowsMetrics(metrics, memInfo)
	}

	// darwin only
	if runtime.GOOS == darwinOS {
		metrics = m.appendDarwinMetrics(metrics, memInfo)
	}

	// linux only
	if runtime.GOOS == linuxOS {
		metrics = m.appendLinuxMetrics(metrics, memInfo)
	}

	m.consumer.ConsumeMetricsData(ctx, consumerdata.MetricsData{Metrics: m.toPBMetrics(metrics)})
}

func (m *Monitor) appendWindowsMetrics(metrics []*timeSeries, memInfo *mem.VirtualMemoryStat) []*timeSeries {
	return append(metrics, &timeSeries{
		metricMemAvailable,
		m.getInt64TimeSeries(memInfo.Available),
	})
}

func (m *Monitor) appendNonWindowsMetrics(metrics []*timeSeries, memInfo *mem.VirtualMemoryStat) []*timeSeries {
	return append(metrics, &timeSeries{
		metricMemFree,
		m.getInt64TimeSeries(memInfo.Free),
	})
}

func (m *Monitor) appendDarwinMetrics(metrics []*timeSeries, memInfo *mem.VirtualMemoryStat) []*timeSeries {
	return append(
		metrics,
		&timeSeries{
			metricMemActive,
			m.getInt64TimeSeries(memInfo.Active),
		},
		&timeSeries{
			metricMemInactive,
			m.getInt64TimeSeries(memInfo.Inactive),
		},
		&timeSeries{
			metricMemWired,
			m.getInt64TimeSeries(memInfo.Wired),
		},
	)
}

func (m *Monitor) appendLinuxMetrics(metrics []*timeSeries, memInfo *mem.VirtualMemoryStat) []*timeSeries {
	return append(
		metrics,
		&timeSeries{
			metricMemBuffered,
			m.getInt64TimeSeries(memInfo.Buffers),
		},
		&timeSeries{
			metricMemCached,
			m.getInt64TimeSeries(memInfo.Cached - memInfo.SReclaimable),
		},
		&timeSeries{
			metricMemSlabRecl,
			m.getInt64TimeSeries(memInfo.SReclaimable),
		},
		&timeSeries{
			metricMemSlabUnrecl,
			m.getInt64TimeSeries(memInfo.Slab - memInfo.SReclaimable),
		},
	)
}

func (m *Monitor) getInt64TimeSeries(val uint64) *metricspb.TimeSeries {
	return m.getTimeSeries(&metricspb.Point{
		Timestamp: ptime.TimeToTimestamp(time.Now()),
		Value:     &metricspb.Point_Int64Value{Int64Value: int64(val)},
	})
}

func (m *Monitor) getDoubleTimeSeries(val float64) *metricspb.TimeSeries {
	return m.getTimeSeries(&metricspb.Point{
		Timestamp: ptime.TimeToTimestamp(time.Now()),
		Value:     &metricspb.Point_DoubleValue{DoubleValue: val},
	})
}

func (m *Monitor) getTimeSeries(p *metricspb.Point) *metricspb.TimeSeries {
	return &metricspb.TimeSeries{
		StartTimestamp: ptime.TimeToTimestamp(m.startTime),
		Points:         []*metricspb.Point{p},
		LabelValues:    nil,
	}
}

func (m *Monitor) toPBMetrics(tsMetrics []*timeSeries) []*metricspb.Metric {
	pbMetrics := make([]*metricspb.Metric, 0, len(tsMetrics))
	for _, ts := range tsMetrics {
		pbMetrics = append(pbMetrics, &metricspb.Metric{
			MetricDescriptor: ts.metric,
			Resource:         m.resource,
			Timeseries:       []*metricspb.TimeSeries{ts.value},
		})
	}
	return pbMetrics
}
