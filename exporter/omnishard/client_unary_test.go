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
	"fmt"
	"math/rand"
	"sort"
	"testing"

	jaeger "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	omnishardpb "github.com/signalfx/apm-opentelemetry-collector/exporter/omnishard/gen"
)

func TestClientUnaryConnectCancellation(t *testing.T) {
	c := NewClientUnary(zap.NewNop())

	cancelCh := make(chan interface{})
	done := make(chan interface{})

	var err error
	go func() {
		err = c.Connect(ConnectionOptions{
			Endpoint: "localhost:0",
		}, cancelCh)

		close(done)
	}()

	// Signal cancellation.
	close(cancelCh)

	// Wait until cancelled.
	<-done

	// Check Connect returned an error.
	assert.NotNil(t, err)
}

func TestClientUnaryConnect(t *testing.T) {
	// Find a local address for delivery.
	endpoint := GetAvailableLocalAddress()
	server := newMockServer()

	// Run a server
	go runServer(server, endpoint, nil)
	defer server.Stop()

	// Create a client
	client := NewClientUnary(zap.NewNop())
	cancelCh := make(chan interface{})

	// Connect to the server
	err := client.Connect(ConnectionOptions{
		Endpoint:  endpoint,
		UseSecure: false,
	}, cancelCh)

	// Make sure connection succeeded.
	assert.Nil(t, err)
}

func TestClientUnaryGetConfig(t *testing.T) {
	_, server, _ := setupClientUnaryServer(t, 0, &clientSink{})
	server.Stop()
}

func TestClientUnarySendRequest(t *testing.T) {

	// Try with different stream counts.
	for streamCount := uint(1); streamCount < 20; streamCount = streamCount * 2 {
		func() {
			clientSink := &clientSink{}
			client, server, config := setupClientUnaryServer(t, streamCount, clientSink)
			defer server.Stop()

			const recordCount = 1000
			for i := 0; i < recordCount; i++ {
				// Create a record and send it.
				record := &omnishardpb.EncodedRecord{
					SpanCount:         int64(i),
					PartitionKey:      fmt.Sprintf("%05d", i),
					UncompressedBytes: int64(i * 10),
				}
				originalSpans := []*jaeger.Span{{}} // fake span, doesn't matter in this test.
				shard := config.ShardDefinitions[rand.Intn(len(config.ShardDefinitions))]
				client.Send(record, originalSpans, shard)
			}

			// Wait until it arrives to the server.
			WaitFor(t, func() bool {
				serverRecords := server.Sink.GetRecords()
				return len(serverRecords) == recordCount
			})

			// Check it arrived intact to server.
			serverRecords := server.Sink.GetRecords()

			// Need to sort the records because they may arrive out of order.
			sort.Sort(byPartitionKey(serverRecords))

			require.True(t, len(serverRecords) == recordCount)

			for i := 0; i < recordCount; i++ {
				record := &omnishardpb.EncodedRecord{
					SpanCount:         int64(i),
					PartitionKey:      fmt.Sprintf("%05d", i),
					UncompressedBytes: int64(i * 10),
				}
				assert.EqualValues(t, record, serverRecords[i])
			}

			// Wait for client to get responses from the server.
			WaitFor(t, func() bool {
				return len(clientSink.getResponseToRecords()) == recordCount
			})

			// Verify that client got correct response for all records.
			responseToRecords := clientSink.getResponseToRecords()

			// Need to sort the records because they may arrive out of order.
			sort.Sort(byPartitionKey(responseToRecords))

			for i := 0; i < recordCount; i++ {
				record := &omnishardpb.EncodedRecord{
					SpanCount:         int64(i),
					PartitionKey:      fmt.Sprintf("%05d", i),
					UncompressedBytes: int64(i * 10),
				}
				assert.EqualValues(t, record, responseToRecords[i])
			}

			// Check that all responses were success.
			responses := clientSink.getResponses()
			for i := 0; i < len(responses); i++ {
				response := responses[i]
				assert.EqualValues(t, omnishardpb.ExportResponse_SUCCESS, response.ResultCode)
				assert.Nil(t, response.ShardingConfig)
			}
		}()
	}
}

// TODO: add tests for server closing connection, server closing stream,
// server returning non SUCCESS responses.

func setupClientUnaryServer(t *testing.T, sendConcurrency uint, callback *clientSink) (*ClientUnary, *mockServer, *omnishardpb.ShardingConfig) {
	// Find a local address for delivery.
	endpoint := GetAvailableLocalAddress()
	server := newMockServer()
	server.SetConfig(createTestShardConfig(4))

	headers := map[string]string{
		"hdr-test-key": "hdr-test-value",
	}

	// Run a server
	go runServer(server, endpoint, headers)

	// Create a client
	client := NewClientUnary(zap.NewNop())
	cancelCh := make(chan interface{})

	// Connect to the server
	err := client.Connect(ConnectionOptions{
		SendConcurrency: sendConcurrency,
		Endpoint:        endpoint,
		UseSecure:       false,
		Headers: map[string]string{
			"hdr-test-key": "hdr-test-value",
		},
		OnSendResponse: callback.onSendResponse,
		OnSendFail:     callback.onSendFail,
	}, cancelCh)

	// Make sure connection succeeded.
	require.Nil(t, err)

	// Get the initial config.
	config, err := client.GetShardingConfig()
	require.Nil(t, err)

	// Normalize received hash keys: nil values should be converted to empty byte slices
	// because that is how createTestShardConfig() generates them, but sending via
	// gRPC/ProtoBuf loses this information. Here we convert nil back to empty slice
	// so that EqualValues works correctly.
	for _, shard := range config.ShardDefinitions {
		if shard.StartingHashKey == nil {
			shard.StartingHashKey = []byte{}
		}
		if shard.EndingHashKey == nil {
			shard.EndingHashKey = []byte{}
		}
	}

	assert.EqualValues(t, server.GetConfig(), config)

	return client, server, config
}
