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
	"bytes"
	"compress/gzip"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	jaeger "github.com/jaegertracing/jaeger/model"
	encodemodel "github.com/omnition/opencensus-go-exporter-kinesis/models/gen"
	"go.uber.org/zap"

	omnitelpb "github.com/Omnition/omnition-opentelemetry-service/exporter/omnitelk/gen"
)

const avgBatchSizeInSpans = 1000

var compressedMagicByte = [8]byte{111, 109, 58, 106, 115, 112, 108, 122}

// A function that accepts encoded records, the original spans that were encoded
// and the config that was used for encoding.
type recordConsumeFunc func(record *omnitelpb.EncodedRecord, originalSpans []*jaeger.Span, shard *shardInMemConfig)

// A function that accepts spans that failed processing and the failure code.
type spanProcessFailFunc func(failedSpans []*jaeger.Span, code EncoderErrorCode)

// shardEncoder encodes spans belonging to a single shard into a sequence
// of EncodedRecords. Spans are batched according to maxRecordSize and flushInterval
// settings. Encoding results are reported via onRecordReady and onSpanProcessFail
// callbacks asynchronously.
type shardEncoder struct {
	logger *zap.Logger

	// Shard configuration.
	shard shardInMemConfig

	// Hooks to call on operations.
	hooks *telemetryHooks

	// Maximum size of one record in total uncompressed bytes of encoded spans.
	maxRecordSize uint

	// MaxAllowedSizePerSpan limits the size of an individual span (in uncompressed
	// encoded bytes)
	maxAllowedSizePerSpan uint

	// Interval of flushing the batch if doesn't reach maxRecordSize.
	flushInterval time.Duration

	// Called when encoded record is ready. May be called synchronously from Encode()
	// or asynchronously from a different goroutine.
	onRecordReady recordConsumeFunc

	// Called when encoding of spans fails. Also called if encoder is stopped while still
	// having pending spans to encode. All pending spans are returned via this func.
	// May be called synchronously from Encode() or asynchronously from a different goroutine.
	onSpanProcessFail spanProcessFailFunc

	// Accumulated encoded spans that are not yet flushed out.
	spans *encodemodel.SpanList

	// Total accumulated size of encoded spans in uncompressed bytes.
	accumulatedSize uint

	// Gzip writer to compressed encoded spans.
	gzipWriter *gzip.Writer

	// Mutex to protect spans, accumulatedSize and gzipWriter fields which are used
	// concurrently.
	sync.RWMutex

	// Done signal, used to interrupt internal goroutine.
	done chan interface{}

	// Stopped flag, is set to non-zero simultaneously with done signal.
	stopped uint32
}

// newShardEncoder creates a new shardEncoder.
func newShardEncoder(
	logger *zap.Logger,
	shard shardInMemConfig,
	hooks *telemetryHooks,
	maxRecordSize uint,
	maxAllowedSizePerSpan uint,
	batchFlushInterval time.Duration,
	onRecordReady recordConsumeFunc,
	onSpanProcessFail spanProcessFailFunc,
) *shardEncoder {
	return &shardEncoder{
		logger:                logger,
		shard:                 shard,
		hooks:                 hooks,
		maxRecordSize:         maxRecordSize,
		maxAllowedSizePerSpan: maxAllowedSizePerSpan,
		flushInterval:         batchFlushInterval,
		onRecordReady:         onRecordReady,
		onSpanProcessFail:     onSpanProcessFail,
		done:                  make(chan interface{}),
		gzipWriter:            gzip.NewWriter(&bytes.Buffer{}),
		spans:                 &encodemodel.SpanList{Spans: make([]*jaeger.Span, 0, avgBatchSizeInSpans)},
		accumulatedSize:       0,
	}
}

// Start the encoder. Call once after newShardEncoder().
func (se *shardEncoder) Start() {
	go se.flushPeriodically()
}

// Stop the encoder. Can be called after Start() is called. Call at the end to
// cleanup when encoder is no longer needed. Safe to call concurrently with Encode()
// and while encoding is in progress. Can be called once only.
// If there are pending spans they will be returned via onSpanProcessFail() callback.
// This means that either spans will be encoded and will be emitted via onRecordReady
// or will be returned via onSpanProcessFail, so no spans will be lost even during stopping.
func (se *shardEncoder) Stop() {
	// Signal stop if not already stopped.
	if atomic.CompareAndSwapUint32(&se.stopped, 0, 1) {
		close(se.done)
	}
}

// Encode a span. Can be called after Start() is called. If accumulated size of spans
// exceeds maxRecordSize it will flush the batch and call onRecordReady.
// Returns encoded uncompressed size of the span.
func (se *shardEncoder) Encode(span *jaeger.Span) uint {
	// We are marshaling the span twice. Once to calculate the size and then again
	// to actually encode it into a record.
	// TODO: See if we can marshal only once and keep marshaled span in the accumulated
	// batch. We will have to arrange the bytes exactly as protobuf marshaller would
	// encode a SpanList object.

	size, err := se.tryEncodeSpan(span)
	if err != nil {
		return 0
	}

	// We know the size, let's batch it.
	se.accumulateEncodedSpan(span, size)

	return uint(size)
}

// Flush accumulated spans. Encodes and emits via onSpanProcessFail on success or
// via onSpanProcessFail on error.
func (se *shardEncoder) Flush() {
	// Lock the encode. Note that we Unlock before returning or before calling
	// onSpanProcessFail or onRecordReady callbacks to hold the lock for as short
	// as possible.
	se.Lock()

	numSpans := len(se.spans.Spans)
	if numSpans == 0 {
		// Nothing to do.
		se.Unlock()
		return
	}

	if se.isStopped() {
		// We are stopped. Return all pending spans via onSpanProcessFail callback.
		spans := se.spans.Spans
		se.clearAccumulated()
		se.Unlock()
		se.onSpanProcessFail(spans, ErrEncoderStopped)
		return
	}

	encoded, err := proto.Marshal(se.spans)
	if err != nil {
		// Can't marshal. Return all pending spans via onSpanProcessFail callback.
		spans := se.spans.Spans
		se.clearAccumulated()
		se.Unlock()
		se.onSpanProcessFail(spans, ErrEncodingFailed)
		se.logger.Error("failed to marshal: ", zap.Error(err))
		return
	}

	compressed, err := se.compress(encoded)
	if err != nil {
		// Can't compress. Return all pending spans via onSpanProcessFail callback.
		spans := se.spans.Spans
		se.clearAccumulated()
		se.Unlock()
		se.onSpanProcessFail(spans, ErrEncodingFailed)
		se.logger.Error("failed to compress: ", zap.Error(err))
		return
	}

	// Encoded and compressed successfully. Emit via onRecordReady callback.
	record := &omnitelpb.EncodedRecord{
		Data:              compressed,
		PartitionKey:      se.spans.Spans[0].TraceID.String(),
		SpanCount:         int64(numSpans),
		UncompressedBytes: int64(len(encoded)),
	}
	originalSpans := se.spans.Spans

	se.clearAccumulated()
	se.Unlock()

	se.onRecordReady(record, originalSpans, &se.shard)

	// Record stats.
	se.hooks.OnCompressed(record.UncompressedBytes, int64(len(record.Data)))
	se.hooks.OnSpanListFlushed(record.SpanCount, int64(len(record.Data)))
}

// tryEncodeSpan encodes the span and ensures encoded size is within maxAllowedSizePerSpan.
// If the encoded size is bigger it will try to remove tags or logs to fit the span.
// Returns the encoded uncompressed size in bytes on success. Returns error if encoding
// fails or cannot fit within maxAllowedSizePerSpan.
func (se *shardEncoder) tryEncodeSpan(span *jaeger.Span) (size int, err error) {
	// Marshal to calculate the size.
	encoded, err := proto.Marshal(span)
	if err != nil {
		se.logger.Error("failed to marshal", zap.Error(err))
		se.onSpanProcessFail([]*jaeger.Span{span}, ErrEncodingFailed)
		return 0, err
	}
	size = len(encoded)
	if uint(size) > se.maxAllowedSizePerSpan {
		// Span is too big.
		se.hooks.OnXLSpanTruncated(size)

		// Try to cut it a bit, replace the tags.
		span.Tags = []jaeger.KeyValue{
			{Key: "omnition.truncated", VBool: true, VType: jaeger.ValueType_BOOL},
			{Key: "omnition.truncated.reason", VStr: "unsupported size", VType: jaeger.ValueType_STRING},
			{Key: "omnition.truncated.size", VInt64: int64(size), VType: jaeger.ValueType_INT64},
		}
		// And remove the logs.
		span.Logs = []jaeger.Log{}

		// Now try to encode again.
		encoded, err = proto.Marshal(span)
		if err != nil {
			// Cannot encode, drop it.
			se.logger.Error("failed to marshal span", zap.Error(err))
			se.onSpanProcessFail([]*jaeger.Span{span}, ErrEncodingFailed)
			return 0, err
		}
		size = len(encoded)

		// Check the size again.
		if uint(size) > se.maxAllowedSizePerSpan {
			// Still too big, drop it.
			se.logger.Error("failed to reduce span size", zap.Error(err))
			se.onSpanProcessFail([]*jaeger.Span{span}, ErrEncodingFailed)
			return 0, errors.New("spans is too big")
		}
	}
	return size, nil
}

func (se *shardEncoder) accumulateEncodedSpan(span *jaeger.Span, size int) {
	// Acquire lock to shared data.
	se.Lock()

	if se.isStopped() {
		se.Unlock()
		// Return the span, we are stopped.
		se.onSpanProcessFail([]*jaeger.Span{span}, ErrEncoderStopped)
		return
	}

	// Append to the batch.
	se.spans.Spans = append(se.spans.Spans, span)
	se.accumulatedSize += uint(size)

	// If needed flush the queue.
	needFlush := se.accumulatedSize >= se.maxRecordSize
	se.Unlock()

	if needFlush {
		se.Flush()
	}
}

func (se *shardEncoder) clearAccumulated() {
	se.spans.Spans = []*jaeger.Span{}
	se.accumulatedSize = 0
}

func (se *shardEncoder) isStopped() bool {
	return atomic.LoadUint32(&se.stopped) > 0
}

// compress is unsafe for concurrent usage. caller must protect calls with mutexes.
func (se *shardEncoder) compress(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(compressedMagicByte[:])
	se.gzipWriter.Reset(&buf)

	_, err := se.gzipWriter.Write(in)
	if err != nil {
		return nil, err
	}

	if err := se.gzipWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (se *shardEncoder) flushPeriodically() {
	// TODO: Start timer after last flush instead of periodic.
	ticker := time.NewTicker(se.flushInterval)
	for {
		select {
		case <-ticker.C:
			se.Flush()

		case <-se.done:
			se.Flush()
			return
		}
	}
}
