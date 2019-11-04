// Copyright 2019, Omnition Authors
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

package signalfx

import (
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/signalfx/golib/datapoint"
	"go.uber.org/zap"
)

// OCProtoToSignalFx converts a OpenCensus metrics to SignalFx Datapoints.
func OCProtoToSignalFx(logger *zap.Logger, md consumerdata.MetricsData) ([]*datapoint.Datapoint, error) {
	// Slice with minimum required capacity
	dps := make([]*datapoint.Datapoint, 0, len(md.Metrics))
	for _, metric := range md.Metrics {
		dps = translateAndAppend(logger, dps, md.Resource, md.Node, metric)
	}
	return dps, nil
}

func translateAndAppend(
	logger *zap.Logger,
	dps []*datapoint.Datapoint,
	resource *resourcepb.Resource,
	node *commonpb.Node,
	metric *metricspb.Metric,
) []*datapoint.Datapoint {
	descriptor := metric.GetMetricDescriptor()
	if descriptor == nil {
		logger.Error("metric received with nil descriptor")
		return dps
	}
	for _, ts := range metric.Timeseries {
		for _, pt := range ts.Points {
			dp := &datapoint.Datapoint{
				Metric: descriptor.GetName(),
			}

			// set timestamp
			pts := pt.GetTimestamp()
			if pts != nil {
				tstamp, err := ptypes.TimestampFromProto(pts)
				if err == nil {
					dp.Timestamp = tstamp
				}
			}

			// set value
			switch descriptor.Type {
			case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DOUBLE:
				dp.Value = datapoint.NewFloatValue(pt.GetDoubleValue())
			case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_GAUGE_INT64:
				dp.Value = datapoint.NewIntValue(pt.GetInt64Value())
			}

			// set type
			switch descriptor.Type {
			case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_CUMULATIVE_INT64:
				dp.MetricType = datapoint.Counter
			case metricspb.MetricDescriptor_GAUGE_DOUBLE, metricspb.MetricDescriptor_GAUGE_INT64:
				dp.MetricType = datapoint.Gauge
			}

			dp.Dimensions = map[string]string{
				"translator": "otel",
			}

			dps = append(dps, dp)
		}
	}
	return dps
}
