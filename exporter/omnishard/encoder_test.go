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
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"testing"
	"time"

	jaeger "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	omnishardpb "github.com/Omnition/omnition-opentelemetry-collector/exporter/omnishard/gen"
)

var testConfig = createTestShardConfig(4)

func createTestShardConfig(shardCount int) *omnishardpb.ShardingConfig {
	cfg := &omnishardpb.ShardingConfig{}

	// Create shards, with evenly spaced start and end hash keys covering entire
	// hash key space from minHashKey to maxHashKey.

	prevHashKey := big.NewInt(0)
	prevHashKey.Set(minHashKey)

	var hashKey *big.Int
	for i := 1; i <= shardCount; i++ {
		// hashKey = maxHashKey*i/shardCount
		hashKey = big.NewInt(0)
		hashKey.Set(maxHashKey)
		hashKey.Mul(hashKey, big.NewInt(int64(i)))
		hashKey.Div(hashKey, big.NewInt(int64(shardCount)))

		shard := &omnishardpb.ShardDefinition{
			ShardId:         "shard#" + strconv.Itoa(i),
			StartingHashKey: prevHashKey.Bytes(),
			EndingHashKey:   hashKey.Bytes(),
		}

		cfg.ShardDefinitions = append(cfg.ShardDefinitions, shard)

		// prevHashKey = hashKey+1
		prevHashKey.Set(hashKey)
		prevHashKey.Add(prevHashKey, big.NewInt(1))
	}
	if hashKey.Cmp(maxHashKey) != 0 {
		log.Fatal("Last hash key must be the end of the hashing space.")
	}

	return cfg
}

func createEncoder(t *testing.T, flushInterval time.Duration, sink *encoderSink) *encoder {
	o := &Options{
		BatchFlushInterval: flushInterval,
		NumWorkers:         4,
	}

	e, err := newEncoder(
		o,
		zap.NewNop(),
		sink.onReady,
		sink.onFail,
	)
	require.Nil(t, err)

	e.SetConfig(testConfig)

	return e
}

func createSpanBatch() []*jaeger.Span {
	// Create a batch with a single span, with random trace id.
	span := &jaeger.Span{
		TraceID: jaeger.NewTraceID(rand.Uint64(), rand.Uint64()),
	}
	return []*jaeger.Span{span}
}

func TestNewEncoder(t *testing.T) {
	e := createEncoder(t, time.Millisecond, &encoderSink{})

	// Wait until config is applied.
	var err error
	var se *shardEncoder
	WaitFor(t, func() bool {
		se, err = e.getShardEncoder("abc")
		return se != nil
	})

	assert.Nil(t, err)

	// "abc" partition key should be mapped to shard at index 2 according to the mapping
	// function that we have currently (obtained from a test run, not calculated manually).
	shardID := 2
	assert.EqualValues(t, testConfig.ShardDefinitions[shardID].ShardId, se.shard.shardID)

	e.Stop(time.Second)
}

func TestEncodeSpan(t *testing.T) {
	for i := 0; i < 10; i++ {
		sink := &encoderSink{}
		e := createEncoder(t, time.Millisecond, sink)

		e.EncodeSpans(createSpanBatch())

		WaitForN(t, func() bool {
			sink.mutex.Lock()
			defer sink.mutex.Unlock()
			return sink.successSpanCount == 1
		}, time.Millisecond*500, "encoding is successful")

		assert.EqualValues(t, 1, sink.encodedRecords[0].SpanCount)
		assert.EqualValues(t, 0, sink.failSpanCount)
		assert.EqualValues(t, 0, len(sink.failedSpans))

		e.Stop(time.Second)
	}
}

func TestEncodeStopStart(t *testing.T) {

	for i := 0; i < 10; i++ {
		sink := &encoderSink{}
		e := createEncoder(t, time.Millisecond, sink)

		spans := createSpanBatch()
		e.EncodeSpans(spans)

		// Try to randomly sleep from 0 to twice the flush interval to try to race the encoding
		// which is supposed to happen after flush internal.
		time.Sleep(time.Duration(int64(float64(time.Millisecond.Nanoseconds()) * 2 * rand.Float64())))

		e.Stop(time.Second)

		WaitForN(t, func() bool {
			sink.mutex.Lock()
			defer sink.mutex.Unlock()
			return sink.successSpanCount > 0 || sink.failSpanCount > 0
		}, time.Millisecond*500, "encoding is success or failure")

		// Span must be either successfully encoded or failed.
		assert.EqualValues(t, 1, sink.successSpanCount+sink.failSpanCount)

		if sink.failSpanCount > 0 {
			assert.EqualValues(t, sink.failSpanCount, len(sink.failedSpans))
			assert.Equal(t, spans, sink.getFailedSpans())
		}
	}
}

func TestEncodeDrainOnStop(t *testing.T) {

	for i := 0; i < 10; i++ {
		sink := &encoderSink{}

		// Use large flash interval to avoid flushing.
		e := createEncoder(t, time.Second*10, sink)

		// Encode a batch of spans.
		const spanCount = 100
		for j := 0; j < spanCount; j++ {
			e.EncodeSpans(createSpanBatch())
		}

		// Stop with waiting for encoding to finish.
		e.Stop(time.Second)

		// Wait for all spans to be reported as failed.
		WaitForN(t, func() bool {
			sink.mutex.Lock()
			defer sink.mutex.Unlock()
			return sink.successSpanCount == spanCount
		}, time.Millisecond*500, "encoding is success or failure")

		// Span must be all failed.
		assert.EqualValues(t, spanCount, sink.successSpanCount)
		assert.EqualValues(t, 0, sink.failSpanCount)
		assert.EqualValues(t, 0, len(sink.failedSpans))
	}
}

func TestEncoderSetConfigEncodeRace(t *testing.T) {
	for i := 0; i < 10; i++ {
		sink := &encoderSink{}

		// Use large flash interval to avoid flushing.
		e := createEncoder(t, time.Second*10, sink)

		for j := 0; j < 10; j++ {

			// Encode a batch of spans.
			spanCount := rand.Intn(100) + 1
			for k := 0; k < spanCount; k++ {
				e.EncodeSpans(createSpanBatch())
			}

			e.SetConfig(testConfig)
		}

		e.Stop(time.Second)
	}
}

func TestEncodeAfterStop(t *testing.T) {
	sink := &encoderSink{}

	e := createEncoder(t, time.Second*10, sink)
	e.Stop(time.Second)

	// Encode a batch of spans.
	err := e.EncodeSpans(createSpanBatch())

	// It must fail.
	assert.NotNil(t, err)

	// Nothing should be encoded or reported as failed.
	assert.EqualValues(t, 0, sink.successSpanCount)
	assert.EqualValues(t, 0, len(sink.encodedRecords))
	assert.EqualValues(t, 0, sink.failSpanCount)
	assert.EqualValues(t, 0, len(sink.failedSpans))
}

func TestEncodeLargeSpanFail(t *testing.T) {
	sink := &encoderSink{}

	e := createEncoder(t, time.Second*10, sink)

	// Create a batch of one span.
	spans := []*jaeger.Span{
		{
			TraceID: jaeger.NewTraceID(rand.Uint64(), rand.Uint64()),
			// Very large operation name to ensure encoding fails.
			OperationName: GenRandByteString(defMaxAllowedSizePerSpan + 1),
		},
	}

	// Begin encoding it.
	err := e.EncodeSpans(spans)

	// It must be accepted for encoding.
	assert.Nil(t, err)

	e.Stop(time.Second)

	// Wait for encoding to fail.
	WaitForN(t, func() bool {
		sink.mutex.Lock()
		defer sink.mutex.Unlock()
		return sink.failSpanCount == 1
	}, time.Millisecond*500, "encoding is failure")

	assert.EqualValues(t, 0, sink.successSpanCount)
	assert.EqualValues(t, 0, len(sink.encodedRecords))
	assert.EqualValues(t, spans, sink.failedSpans)
}

func TestEncodeLargeSpanCut(t *testing.T) {
	sink := &encoderSink{}

	e := createEncoder(t, time.Second*10, sink)

	// Create a batch of one span.
	spans := []*jaeger.Span{
		{
			TraceID: jaeger.NewTraceID(rand.Uint64(), rand.Uint64()),
			Logs: []jaeger.Log{
				{
					Fields: []jaeger.KeyValue{
						{
							Key:   "test",
							VType: jaeger.ValueType_STRING,
							// Very large log to ensure encoding hits the limit.
							VStr: GenRandByteString(defMaxAllowedSizePerSpan + 1),
						},
					},
				},
			},
		},
	}

	// Begin encoding it.
	err := e.EncodeSpans(spans)

	// It must be accepted for encoding.
	assert.Nil(t, err)

	e.Stop(time.Second)

	// Wait for encoding to succeed.
	WaitForN(t, func() bool {
		sink.mutex.Lock()
		defer sink.mutex.Unlock()
		return sink.successSpanCount == 1
	}, time.Millisecond*500, "encoding is success")

	assert.EqualValues(t, 1, sink.successSpanCount)
	assert.EqualValues(t, 1, len(sink.encodedRecords))
	assert.EqualValues(t, 0, sink.failSpanCount)
}

func TestEncodeSpanBeforeConfig(t *testing.T) {
	for i := 0; i < 10; i++ {
		sink := &encoderSink{}

		o := &Options{
			BatchFlushInterval: time.Millisecond,
			NumWorkers:         4,
		}

		e, err := newEncoder(
			o,
			zap.NewNop(),
			sink.onReady,
			sink.onFail,
		)
		require.Nil(t, err)

		// Encode spans.
		e.EncodeSpans(createSpanBatch())

		// Set config afterwards. This should still work since it is supported.
		e.SetConfig(testConfig)

		WaitForN(t, func() bool {
			sink.mutex.Lock()
			defer sink.mutex.Unlock()
			return sink.successSpanCount == 1
		}, time.Millisecond*500, "encoding is successful")

		assert.EqualValues(t, 1, sink.encodedRecords[0].SpanCount)
		assert.EqualValues(t, 0, sink.failSpanCount)
		assert.EqualValues(t, 0, len(sink.failedSpans))

		e.Stop(time.Second)
	}
}
