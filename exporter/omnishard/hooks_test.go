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
	"testing"

	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats/view"

	omnishardpb "github.com/Omnition/omnition-opentelemetry-collector/exporter/omnishard/gen"
)

func TestAllTelemetryHooks(t *testing.T) {
	views := metricViews()
	require.NotNil(t, views)
	require.Greater(t, len(views), 0)

	err := view.Register(views...)
	require.NoError(t, err)
	defer view.Unregister(views...)

	th := newTelemetryHooks("test", "00")
	require.NotNil(t, th)

	th.OnSpansEnqueued(10)
	th.OnSpansDequeued(5)

	th.OnXLSpanTruncated(5 * 1024 * 1024)

	th.OnCompressed(1000, 100)
	th.OnSpanListFlushed(10, 1024)

	record := &omnishardpb.EncodedRecord{
		SpanCount:         int64(10),
		PartitionKey:      "01",
		UncompressedBytes: int64(1024),
	}

	th.OnDropSpans(RetryQueueFull, 10, record)
	th.OnDropSpans(FatalEncodingError, 10, nil)
	th.OnDropSpans(ExportResponseNotRetryable, 10, record)
	th.OnDropSpans(SendErrNotRetryable, 10, record)

	th.OnSendResponse(record, &omnishardpb.ExportResponse{
		ResultCode: omnishardpb.ExportResponse_SUCCESS,
	})
	th.OnSendResponse(record, &omnishardpb.ExportResponse{
		ResultCode: omnishardpb.ExportResponse_FAILED_NOT_RETRYABLE,
	})
	th.OnSendResponse(record, &omnishardpb.ExportResponse{
		ResultCode: omnishardpb.ExportResponse_FAILED_RETRYABLE,
	})
	th.OnSendResponse(record, &omnishardpb.ExportResponse{
		ResultCode: omnishardpb.ExportResponse_SHARD_CONFIG_MISTMATCH,
	})
}
