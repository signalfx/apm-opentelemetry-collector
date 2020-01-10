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
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"math/rand"
	"testing"
	"time"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/gogo/protobuf/proto"
	jaeger "github.com/jaegertracing/jaeger/model"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configgrpc"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	tracetranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	omnishard "github.com/signalfx/apm-opentelemetry-collector/exporter/omnishard/gen"
	encodemodel "github.com/signalfx/opencensus-go-exporter-kinesis/models/gen"
)

func TestNewExporter(t *testing.T) {
	exp, server := setupExporter(t, 1)
	defer server.Stop()

	err := exp.Shutdown()
	assert.Nil(t, err)
}

func TestExporterStartAndShutdown(t *testing.T) {
	exp, server := setupExporter(t, 1)
	defer server.Stop()

	exp.Start(component.NewMockHost())
	err := exp.Shutdown()
	assert.Nil(t, err)
}

func TestExporterSend(t *testing.T) {
	exp, server := setupExporter(t, 1)
	defer server.Stop()
	defer func() {
		err := exp.Shutdown()
		assert.Nil(t, err)
	}()

	exp.Start(component.NewMockHost())

	// Send some batches via Exporter.
	sentPartitionKeys := make(map[string]bool)
	sentSpanIDs := make(map[string]bool)
	sentSpanCount := sendSpans(t, exp, 10, sentPartitionKeys, sentSpanIDs)

	// Wait for them to be received and validated.
	waitReceiveSpans(t, server, sentSpanCount, sentPartitionKeys, sentSpanIDs)
}

func TestExporterSendWithReconfig(t *testing.T) {
	exp, server := setupExporter(t, 1)
	defer server.Stop()
	defer func() {
		err := exp.Shutdown()
		assert.Nil(t, err)
	}()

	exp.Start(component.NewMockHost())

	totalSentSpanCount := 0
	sentPartitionKeys := make(map[string]bool)
	sentSpanIDs := make(map[string]bool)
	for i := 0; i < 100; i++ {

		// Change server config to trigger SHARD_CONFIG_MISTMATCH situation.
		newConfig := createTestShardConfig(rand.Intn(40) + 1)
		server.SetConfig(newConfig)

		// Send some span batches via Exporter.
		sentSpanCount := sendSpans(t, exp, rand.Intn(4)+1, sentPartitionKeys, sentSpanIDs)
		totalSentSpanCount += sentSpanCount
	}

	waitReceiveSpans(t, server, totalSentSpanCount, sentPartitionKeys, sentSpanIDs)
}

func TestExporterSendWithServerError(t *testing.T) {
	exp, server := setupExporter(t, 1)
	defer server.Stop()
	server.RandomServerError = true

	defer func() {
		err := exp.Shutdown()
		assert.Nil(t, err)
	}()

	exp.Start(component.NewMockHost())

	// Send some batches via Exporter.
	sentPartitionKeys := make(map[string]bool)
	sentSpanIDs := make(map[string]bool)

	sentSpanCount := sendSpans(t, exp, 100, sentPartitionKeys, sentSpanIDs)

	// Wait for them to be received and validated.
	waitReceiveSpans(t, server, sentSpanCount, sentPartitionKeys, sentSpanIDs)
}

func setupExporter(t *testing.T, streamCount uint) (*Exporter, *mockServer) {
	// Find a local address for delivery.
	endpoint := GetAvailableLocalAddress()

	cfg := &Config{
		GRPCSettings: configgrpc.GRPCSettings{
			Endpoint:  endpoint,
			UseSecure: false,
		},
		SendConcurrency:    streamCount,
		BatchFlushInterval: 100 * time.Millisecond,
	}

	logger, _ := zap.NewProduction()

	client := NewClientUnary(logger)

	// Run a server
	server := newMockServer()
	server.SetConfig(createTestShardConfig(4))

	go runServer(server, endpoint, nil)

	exp, err := NewExporter(cfg, logger, client)

	assert.Nil(t, err)
	require.NotNil(t, exp)

	return exp, server
}

const traceIDByteCount = 16
const spanIDByteCount = 8

// traceIDToPartitionKey converts OC trace ID to a PartitionKey using the same logic
// that is used by shard encoders.
func traceIDToPartitionKey(traceID []byte) string {
	var jaegerTraceID jaeger.TraceID
	traceIDHigh, traceIDLow, err := tracetranslator.BytesToUInt64TraceID(traceID)
	if err != nil {
		return ""
	}
	if traceIDLow == 0 && traceIDHigh == 0 {
		return ""
	}
	jaegerTraceID = jaeger.TraceID{
		Low:  traceIDLow,
		High: traceIDHigh,
	}

	return jaegerTraceID.String()
}

func jaegerSpanIDtoOC(spanID jaeger.SpanID) []byte {
	return tracetranslator.UInt64ToByteSpanID(uint64(spanID))
}

func createTraceData(spanCount int, createdSpanIDs map[string]bool) consumerdata.TraceData {

	td := consumerdata.TraceData{}

	// Create a batch of spans, with random trace ids.
	for i := 0; i < spanCount; i++ {
		span := &tracepb.Span{
			TraceId: []byte(GenRandByteString(traceIDByteCount)),
			SpanId:  []byte(GenRandByteString(spanIDByteCount)),
		}
		td.Spans = append(td.Spans, span)
		createdSpanIDs[string(span.SpanId)] = true
	}

	return td
}

func decodeRecord(t *testing.T, record *omnishard.EncodedRecord) []*jaeger.Span {
	// Verify heading magic bytes.
	buf := bytes.NewBuffer(record.Data)
	magicBytes := [len(compressedMagicByte)]byte{}
	n, err := buf.Read(magicBytes[:])
	assert.Nil(t, err)
	assert.EqualValues(t, len(compressedMagicByte), n)
	assert.EqualValues(t, magicBytes, compressedMagicByte)

	// Decompress the data following header.
	gzipReader, err := gzip.NewReader(buf)
	assert.Nil(t, err)
	assert.NotNil(t, gzipReader)
	defer gzipReader.Close()

	decBuf := bytes.NewBuffer(nil)

	_, err = io.Copy(decBuf, gzipReader)
	assert.Nil(t, err)

	decompressed := decBuf.Bytes()

	// Unmarshal decompressed data into SpanList.
	var spans encodemodel.SpanList
	err = proto.Unmarshal(decompressed, &spans)
	assert.Nil(t, err)

	return spans.Spans
}

func sendSpans(
	t *testing.T,
	exp *Exporter,
	batchCount int,
	allPartitionKeys map[string]bool,
	sentSpanIDs map[string]bool,
) (sentSpanCount int) {
	sentSpanCount = 0

	for i := 0; i < batchCount; i++ {
		spansInBatch := rand.Intn(100) + 1
		sentSpanCount += spansInBatch
		td := createTraceData(spansInBatch, sentSpanIDs)
		err := exp.ConsumeTraceData(context.Background(), td)

		// Remember all spans sent in appropriate buckets for later verification
		// upon receipt.
		for _, span := range td.Spans {
			// Remember the partition key.
			allPartitionKeys[traceIDToPartitionKey(span.TraceId)] = true
		}

		require.Nil(t, err)
	}
	return
}

func waitReceiveSpans(
	t *testing.T,
	server *mockServer,
	sentSpanCount int,
	sentPartitionKeys map[string]bool,
	sentSpanIDs map[string]bool,
) {
	// Wait until all spans arrive at the server in the form of records.
	var receivedSpanCount int
	WaitForN(t, func() bool {
		receivedSpanCount = 0
		records := server.Sink.GetRecords()
		for _, r := range records {
			receivedSpanCount += int(r.SpanCount)
		}

		// Number of spans sent must match number of spans received.
		return receivedSpanCount == sentSpanCount
	}, time.Second*10)

	decodedSpanCount := 0

	records := server.Sink.GetRecords()
	decodedSpanIDs := make(map[string]bool)

	for _, record := range records {
		// Verify that all received records have partition keys that are
		// present in the partition keys sent.
		exists := sentPartitionKeys[record.PartitionKey]
		assert.True(t, exists, "cannot find trace id "+record.PartitionKey)
		spans := decodeRecord(t, record)

		// Verify that received spans have IDs that we sent.
		for _, span := range spans {
			spanID := string(jaegerSpanIDtoOC(span.SpanID))
			assert.False(t, decodedSpanIDs[spanID], "spanID %x was decoded more than once", spanID)
			if !decodedSpanIDs[spanID] {
				decodedSpanCount++
				decodedSpanIDs[spanID] = true
				assert.True(t, sentSpanIDs[spanID], "spanID %x was decoded but was not sent", spanID)
			}

			delete(sentSpanIDs, spanID)
		}
	}

	// Verify that all sent spans are decoded.
	assert.EqualValues(t, 0, len(sentSpanIDs), "%d spans were sent but not decoded", len(sentSpanIDs))

	// Ensure that received, decoded and sent counters match.
	assert.EqualValues(t, sentSpanCount, receivedSpanCount, "sent and received span count mismatch")
	assert.EqualValues(t, sentSpanCount, decodedSpanCount, "sent and decoded span count mismatch")
}
