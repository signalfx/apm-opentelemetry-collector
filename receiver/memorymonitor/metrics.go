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

package memorymonitor

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

const (
	metricUnitBytes      = "By"
	metricUnitPercentage = "%"
)

// common
var (
	// common
	metricMemUtilization = &metricspb.MetricDescriptor{
		Name:        "memory.utilization",
		Description: "Percent of memory in use on this host. This metric reports with plugin dimension set to signalfx-metadata.",
		Unit:        metricUnitPercentage,
		Type:        metricspb.MetricDescriptor_GAUGE_DOUBLE,
		LabelKeys:   nil,
	}

	metricMemUsed = &metricspb.MetricDescriptor{
		Name:        "memory.used",
		Description: "Bytes of memory in use by the system.",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	// windows
	metricMemAvailable = &metricspb.MetricDescriptor{
		Name:        "memory.available",
		Description: "(Windows Only) Bytes of memory available for use.",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	// linux
	metricMemFree = &metricspb.MetricDescriptor{
		Name:        "memory.free",
		Description: "(Linux Only) Bytes of memory available for use.",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	// darwin
	metricMemActive = &metricspb.MetricDescriptor{
		Name:        "memory.active",
		Description: "(Darwin Only) Active memory.",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	metricMemInactive = &metricspb.MetricDescriptor{
		Name:        "memory.inactive",
		Description: "(Darwin Only) Inactive memory.",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	metricMemWired = &metricspb.MetricDescriptor{
		Name:        "memory.wired",
		Description: "(Darwin Only) Wired memory.",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	// linux
	metricMemBuffered = &metricspb.MetricDescriptor{
		Name:        "memory.buffered",
		Description: "(Linux Only) Bytes of memory used for buffering I/O.",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	metricMemCached = &metricspb.MetricDescriptor{
		Name:        "memory.cached",
		Description: "(Linux Only) Bytes of memory used for disk caching.",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	metricMemSlabRecl = &metricspb.MetricDescriptor{
		Name:        "memory.slab_reclaimable",
		Description: "(Linux Only) Bytes of memory used for SLAB-allocation of kernel objects that can be reclaimed",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}

	metricMemSlabUnrecl = &metricspb.MetricDescriptor{
		Name:        "memory.slab_unreclaimable",
		Description: "(Linux Only) Bytes of memory used for SLAB-allocation of kernel objects that cannot be reclaimed",
		Unit:        metricUnitBytes,
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys:   nil,
	}
)

var allMetrics = []*metricspb.MetricDescriptor{
	metricMemUtilization,
	metricMemUsed,
	metricMemAvailable,
	metricMemFree,
	metricMemActive,
	metricMemInactive,
	metricMemWired,
	metricMemBuffered,
	metricMemCached,
	metricMemSlabRecl,
	metricMemSlabUnrecl,
}
