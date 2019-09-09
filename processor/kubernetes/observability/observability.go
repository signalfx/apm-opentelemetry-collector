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

package observability

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

func init() {
	view.Register(AllViews...)
}

var (
	mPodsUpdated = stats.Int64("otelsvc/k8s/pod_updated", "Number of pod update events received", "1")
	mPodsAdded   = stats.Int64("otelsvc/k8s/pod_added", "Number of pod add events received", "1")
	mPodsDeleted = stats.Int64("otelsvc/k8s/pod_deleted", "Number of pod delete events received", "1")

	mIPLookupMiss = stats.Int64("otelsvc/k8s/ip_lookup_miss", "Number of times pod by IP lookup failed.", "1")
	mPodsMismatch = stats.Int64("otelsvc/k8s/pod_mismatch", "Number of times k8s processor detected the wrong pod", "1")
)

// ViewPodsUpdated ...
var ViewPodsUpdated = &view.View{
	Name:        mPodsUpdated.Name(),
	Description: mPodsUpdated.Description(),
	Measure:     mPodsUpdated,
	Aggregation: view.Sum(),
}

// ViewPodsAdded ...
var ViewPodsAdded = &view.View{
	Name:        mPodsAdded.Name(),
	Description: mPodsAdded.Description(),
	Measure:     mPodsAdded,
	Aggregation: view.Sum(),
}

// ViewPodsDeleted ...
var ViewPodsDeleted = &view.View{
	Name:        mPodsDeleted.Name(),
	Description: mPodsDeleted.Description(),
	Measure:     mPodsDeleted,
	Aggregation: view.Sum(),
}

// ViewIPLookupMiss ...
var ViewIPLookupMiss = &view.View{
	Name:        mIPLookupMiss.Name(),
	Description: mIPLookupMiss.Description(),
	Measure:     mIPLookupMiss,
	Aggregation: view.Sum(),
}

// ViewPodsMismatch ...
var ViewPodsMismatch = &view.View{
	Name:        mPodsMismatch.Name(),
	Description: mPodsMismatch.Description(),
	Measure:     mPodsMismatch,
	Aggregation: view.Sum(),
}

// AllViews has the views for the metrics provided by the agent.
var AllViews = []*view.View{
	ViewPodsUpdated,
	ViewPodsAdded,
	ViewPodsDeleted,
	ViewIPLookupMiss,
	ViewPodsMismatch,
}

// RecordPodUpdated ...
func RecordPodUpdated() {
	stats.Record(context.Background(), mPodsUpdated.M(int64(1)))
}

// RecordPodAdded ...
func RecordPodAdded() {
	stats.Record(context.Background(), mPodsAdded.M(int64(1)))
}

// RecordPodDeleted ...
func RecordPodDeleted() {
	stats.Record(context.Background(), mPodsDeleted.M(int64(1)))
}

// RecordIPLookupMiss ...
func RecordIPLookupMiss() {
	stats.Record(context.Background(), mIPLookupMiss.M(int64(1)))
}

// RecordPodsMismatch ...
func RecordPodsMismatch() {
	stats.Record(context.Background(), mPodsMismatch.M(int64(1)))
}
