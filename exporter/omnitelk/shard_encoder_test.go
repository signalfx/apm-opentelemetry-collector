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

package omnitelk

import (
	"math"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	omnitelk "github.com/Omnition/omnition-opentelemetry-service/exporter/omnitelk/gen"
	jaeger "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func createShardEncoder(
	maxSize uint,
	flushInterval time.Duration,
	sink *encoderSink,
) *shardEncoder {
	shardID := "abc123"

	shard := shardInMemConfig{
		shardID:         shardID,
		startingHashKey: *big.NewInt(0),
		endingHashKey:   *big.NewInt(1000),
	}

	hooks := newTelemetryHooks("test", shardID)

	se := newShardEncoder(
		zap.NewNop(),
		shard,
		hooks,
		maxSize,
		math.MaxInt32,
		flushInterval,
		sink.onReady,
		sink.onFail,
	)

	return se
}

func TestShardEncoderSinglePut(t *testing.T) {
	sink := &encoderSink{}

	se := createShardEncoder(
		1,                  // small max size, we only have one span.
		1*time.Millisecond, // timeout does not matter since we force one span per batch.
		sink,
	)

	se.Start()

	// Create a span and put it to encoder.
	span := &jaeger.Span{}
	se.Encode(span)

	// It should be immediately encoded since we have very small maxRecordSize.
	sink.mutex.Lock()
	defer sink.mutex.Unlock()

	assert.EqualValues(t, 1, sink.successSpanCount)
	assert.EqualValues(t, 1, sink.encodedRecords[0].SpanCount)
	assert.EqualValues(t, 0, sink.failSpanCount)
}

func TestShardEncoderMultiplePutOneBatch(t *testing.T) {
	sink := &encoderSink{}

	se := createShardEncoder(
		100000000,           // large max to enable batching
		10*time.Millisecond, // give enough time for batch to accumulate,
		sink,
	)

	se.Start()

	// Encode a bunch of spans. The should all fit in one batch.
	traceID := jaeger.NewTraceID(123, 456)
	const spanCount = 100
	var uncompressedBytes uint64
	for i := 0; i < spanCount; i++ {
		span := &jaeger.Span{
			TraceID: traceID,
		}
		size := se.Encode(span)
		uncompressedBytes += uint64(size)
	}

	// There should not be any batches yet, since we Encode the spans quickly and
	// flush interval is large enough that the batch is not yet created.
	assert.EqualValues(t, 0, sink.getSuccessSpanCount())

	// Now wait for a batch to be flushed out.
	WaitFor(t,
		func() bool { return sink.getSuccessSpanCount() == spanCount },
		"Encode to result in onRecordReady")

	// Check that all spans were encoded into one batch.
	encodedRecords := sink.getEncodedRecords()
	assert.EqualValues(t, 1, len(encodedRecords))
	assert.EqualValues(t, spanCount, encodedRecords[0].SpanCount)
	assert.EqualValues(t, 0, sink.getFailSpanCount())

	// Check that partition key is the trace id.
	assert.Equal(t, traceID.String(), encodedRecords[0].PartitionKey)

	// Ensure all spans are fit into record.
	assert.True(t, encodedRecords[0].UncompressedBytes > uncompressedBytes)
}

func TestShardEncoderMultiplePutOneSpanPerBatch(t *testing.T) {
	sink := &encoderSink{}

	se := createShardEncoder(
		1,                  // small max to force one span per batch
		1*time.Millisecond, // short flush interval, we only have one span per batch.
		sink,
	)

	se.Start()

	traceID := jaeger.NewTraceID(123, 456)
	var uncompressedBytes uint64
	const spanCount = 100
	for i := 0; i < spanCount; i++ {
		span := &jaeger.Span{
			TraceID: traceID,
		}
		size := se.Encode(span)
		uncompressedBytes = uint64(size)
	}

	WaitFor(t,
		func() bool { return sink.getSuccessSpanCount() == spanCount },
		"Encode to result in onRecordReady")

	encodedRecords := sink.getEncodedRecords()

	assert.EqualValues(t, 0, sink.getFailSpanCount())
	assert.EqualValues(t, spanCount, len(encodedRecords))

	for _, er := range encodedRecords {
		// Check that we have one span per batch.
		assert.EqualValues(t, 1, er.SpanCount)

		// Check that partition key is the trace id.
		assert.Equal(t, traceID.String(), er.PartitionKey)

		// Ensure the span is fit into record.
		assert.True(t, er.UncompressedBytes > uncompressedBytes)
	}
}

func TestShardEncoderStop(t *testing.T) {
	sink := &encoderSink{}

	se := createShardEncoder(
		1,                  // small max to force one span per batch.
		1*time.Millisecond, // timeout does not matter since we force one span per batch.
		sink,
	)

	// Start the encoder.
	se.Start()

	// Encode one span.
	traceID := jaeger.NewTraceID(123, 456)
	span := &jaeger.Span{
		TraceID: traceID,
	}
	se.Encode(span)

	// Ensure it is encoded.
	assert.EqualValues(t, 1, sink.getSuccessSpanCount())
	assert.EqualValues(t, 0, sink.getFailSpanCount())

	// Stop the encoder.
	se.Stop()

	// Encode another span.
	se.Encode(span)

	// Ensure it is not encoded.
	assert.EqualValues(t, 1, sink.getSuccessSpanCount())
	assert.EqualValues(t, 1, sink.getFailSpanCount())
}

func TestShardEncoderStopConcurrently(t *testing.T) {
	sink := &encoderSink{}

	se := createShardEncoder(
		10000000,           // large max to enable batching.
		1*time.Microsecond, // small timeout to try to make Stop and Encode race.
		sink,
	)

	// Start the encoder.
	se.Start()

	// Encode some spans.
	traceID := jaeger.NewTraceID(123, 456)
	const spanCount = 1000
	for i := 0; i < spanCount; i++ {
		span := &jaeger.Span{
			TraceID: traceID,
		}
		se.Encode(span)
	}

	// Ensure it is not yet encoded.
	assert.EqualValues(t, 0, sink.getFailSpanCount())

	// Stop the encoder. This should likely race with batch flushing, which is what we
	// want to hit.
	se.Stop()

	// Encode more spans to make sure some fail.
	for i := 0; i < spanCount; i++ {
		span := &jaeger.Span{
			TraceID: traceID,
		}
		se.Encode(span)
	}

	// Wait until failure is reported.
	WaitFor(t, func() bool {
		return sink.getFailSpanCount()+sink.getSuccessSpanCount() == 2*spanCount
	})
	assert.True(t, sink.getFailSpanCount() >= spanCount)
	assert.True(t, len(sink.getFailedSpans()) == sink.getFailSpanCount())
}

func TestShardEncoderCannotMarshal(t *testing.T) {
	var failSpanCount uint32
	var failureCode FailureCode

	shardID := "abc123"

	shard := shardInMemConfig{
		shardID:         shardID,
		startingHashKey: *big.NewInt(0),
		endingHashKey:   *big.NewInt(1000),
	}

	hooks := newTelemetryHooks("test", shardID)

	se := newShardEncoder(
		zap.NewNop(),
		shard,
		hooks,
		1, // small max to disable batching.
		math.MaxInt32,
		1*time.Microsecond, // timeout doesn't matter.
		func(record *omnitelk.EncodedRecord, shard *shardInMemConfig) {
		},
		func(failedSpans []*jaeger.Span, code FailureCode) {
			failureCode = code
			atomic.AddUint32(&failSpanCount, uint32(len(failedSpans)))
		},
	)

	// Start the encoder.
	se.Start()

	// Encode a spans
	span := &jaeger.Span{
		// Invalid date so that encoding fails. This is using the fact that minimum
		// valid date for gogoproto encoding is 0001-01-01.
		StartTime: time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC),
	}
	se.Encode(span)

	// Ensure encoding failed.
	assert.EqualValues(t, 1, atomic.LoadUint32(&failSpanCount))
	assert.EqualValues(t, FailedNotRetryable, failureCode)
}
