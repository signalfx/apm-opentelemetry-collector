// Copyright 2019 Omnition Inc.
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

package omnishard

import (
	"context"
	"sync"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

type tagCache struct {
	sync.RWMutex
	tags map[tag.Key]map[string]tag.Mutator
}

type telemetryHooks struct {
	exporterName string
	commonTags   []tag.Mutator
	tagCache     tagCache
}

func newTelemetryHooks(name, shardID string) *telemetryHooks {
	tags := []tag.Mutator{
		tag.Upsert(tagExporterName, name),
	}
	if shardID != "" {
		tags = append(tags, tag.Upsert(tagShardID, shardID))
	}
	return &telemetryHooks{
		exporterName: name,
		commonTags:   tags,
		tagCache: tagCache{
			tags: map[tag.Key]map[string]tag.Mutator{},
		},
	}
}

func (h *telemetryHooks) OnSpanEnqueued() {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statEnqueuedSpans.M(1),
	)
}

func (h *telemetryHooks) OnSpanDequeued() {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statDequeuedSpans.M(1),
	)
}

func (h *telemetryHooks) OnXLSpanTruncated(size int) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statXLSpansBytes.M(int64(size)),
		statXLSpans.M(1),
	)
}

func (h *telemetryHooks) OnSpanListFlushed(spans, bytes int64) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statFlushedSpans.M(spans),
		statFlushedSpansBytes.M(bytes),
	)
}

func (h *telemetryHooks) OnCompressed(original, compressed int64) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statCompressFactor.M(original/compressed),
	)
}

func (h *telemetryHooks) OnDropSpans(spans int64) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statDroppedSpans.M(spans),
	)
}
