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
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	omnitelpb "github.com/Omnition/omnition-opentelemetry-service/exporter/omnitelk/gen"
)

func TestConnectCancellation(t *testing.T) {
	c := NewClient(zap.NewNop())

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

func TestConnect(t *testing.T) {
	// Find a local address for delivery.
	endpoint := GetAvailableLocalAddress()
	server := mockServer{}

	// Run a server
	go runServer(&server, endpoint)
	defer server.Stop()

	// Create a client
	client := NewClient(zap.NewNop())
	cancelCh := make(chan interface{})

	// Connect to the server
	err := client.Connect(ConnectionOptions{
		Endpoint: endpoint,
	}, cancelCh)

	// Make sure connection succeeded.
	assert.Nil(t, err)
}

func TestGetConfig(t *testing.T) {
	_, server, _ := setupClientServer(t, 0, &clientSink{})
	server.Stop()
}

// byPartitionKey implements sort.Interface for []*omnitelpb.EncodedRecord based on
// the PartitionKey field.
type byPartitionKey []*omnitelpb.EncodedRecord

func (a byPartitionKey) Len() int      { return len(a) }
func (a byPartitionKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byPartitionKey) Less(i, j int) bool {
	return a[i].PartitionKey < a[j].PartitionKey
}

func TestSendRequest(t *testing.T) {

	// Try with different stream counts.
	for streamCount := uint(1); streamCount < 20; streamCount = streamCount * 2 {

		func() {
			clientSink := &clientSink{}
			client, server, config := setupClientServer(t, streamCount, clientSink)
			defer server.Stop()

			const recordCount = 1000
			for i := 0; i < recordCount; i++ {
				// Create a record and send it.
				record := &omnitelpb.EncodedRecord{
					SpanCount:         uint64(i),
					PartitionKey:      fmt.Sprintf("%05d", i),
					UncompressedBytes: uint64(i * 10),
				}
				shard := config.ShardDefinitions[rand.Intn(len(config.ShardDefinitions))]
				client.Send(record, shard)
			}

			// Wait until it arrives to the server.
			WaitFor(t, func() bool {
				serverRecords := server.sink.getRecords()
				return len(serverRecords) == recordCount
			})

			// Check it arrived intact to server.
			serverRecords := server.sink.getRecords()

			// Need to sort the records because they may arrive out of order.
			sort.Sort(byPartitionKey(serverRecords))

			require.True(t, len(serverRecords) == recordCount)

			for i := 0; i < recordCount; i++ {
				record := &omnitelpb.EncodedRecord{
					SpanCount:         uint64(i),
					PartitionKey:      fmt.Sprintf("%05d", i),
					UncompressedBytes: uint64(i * 10),
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
				record := &omnitelpb.EncodedRecord{
					SpanCount:         uint64(i),
					PartitionKey:      fmt.Sprintf("%05d", i),
					UncompressedBytes: uint64(i * 10),
				}
				assert.EqualValues(t, record, responseToRecords[i])
			}

			// Check that all responses were success.
			responses := clientSink.getResponses()
			for i := 0; i < len(responses); i++ {
				response := responses[i]
				assert.True(t, response.Id != 0)
				assert.EqualValues(t, omnitelpb.ExportResponse_SUCCESS, response.ResultCode)
				assert.Nil(t, response.ShardingConfig)
			}
		}()
	}
}

// TODO: add tests for server closing connection, server closing stream,
// server returning non SUCCESS responses.

// clientSink is used in the tests to store responses that the client receives from
// the server.
type clientSink struct {
	mutex sync.Mutex

	responseToRecords []*omnitelpb.EncodedRecord
	responses         []*omnitelpb.ExportResponse

	failedRecords [][]*omnitelpb.EncodedRecord
	failureCodes  []FailureCode
}

func (cs *clientSink) onSendResponse(responseToRecords []*omnitelpb.EncodedRecord, response *omnitelpb.ExportResponse) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.responseToRecords = append(cs.responseToRecords, responseToRecords...)
	cs.responses = append(cs.responses, response)
}

func (cs *clientSink) onSendFail(failedRecords []*omnitelpb.EncodedRecord, code FailureCode) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.failedRecords = append(cs.failedRecords, failedRecords)
	cs.failureCodes = append(cs.failureCodes, code)
}

func (cs *clientSink) getResponseToRecords() []*omnitelpb.EncodedRecord {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	return cs.responseToRecords[:]
}

func (cs *clientSink) getResponses() []*omnitelpb.ExportResponse {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	return cs.responses[:]
}

func setupClientServer(t *testing.T, streamCount uint, callback *clientSink) (*Client, *mockServer, *omnitelpb.ShardingConfig) {
	// Find a local address for delivery.
	endpoint := GetAvailableLocalAddress()
	server := &mockServer{
		config: createTestConfig(),
	}

	// Run a server
	go runServer(server, endpoint)

	// Create a client
	client := NewClient(zap.NewNop())
	cancelCh := make(chan interface{})

	// Connect to the server
	err := client.Connect(ConnectionOptions{
		StreamCount:              streamCount,
		Endpoint:                 endpoint,
		StreamReopenRequestCount: 100,
		OnSendResponse:           callback.onSendResponse,
		OnSendFail:               callback.onSendFail,
	}, cancelCh)

	// Make sure connection succeeded.
	require.Nil(t, err)

	// Get the initial config.
	config, err := client.GetShardingConfig()
	require.Nil(t, err)
	assert.EqualValues(t, server.config, config)

	return client, server, config
}
