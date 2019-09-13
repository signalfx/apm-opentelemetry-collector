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
	"container/list"
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	omnitelpb "github.com/Omnition/omnition-opentelemetry-service/exporter/omnitelk/gen"
)

// Client can connect to a server and send an ExportRequest. It uses multiple streams
// for sending concurrently.
type Client struct {
	// gRPC client.
	client omnitelpb.OmnitelKClient

	options ConnectionOptions

	// The streams themselves.
	clientStreams []*clientStream

	// Requests that are pending to be sent.
	requestsToSend chan *omnitelpb.ExportRequest

	logger *zap.Logger
}

var _ client = (*Client)(nil)

// clientStream represent a single gRPC stream in a currently connected Client.
type clientStream struct {
	// Parent Client.
	client *Client

	// gRPC stream.
	stream omnitelpb.OmnitelK_ExportClient

	// Timestamp when stream was opened.
	lastStreamOpen time.Time

	// Map of requests pending acknowledgement by server. Key is request id,
	// value is of pendingRequest type.
	pendingAckMap map[uint64]*list.Element

	// Ordered list of pending acknowledgements. Used for processing timeouts.
	pendingAckList *list.List

	// Mutex to protect the above 2 fields.
	pendingAckMutex sync.Mutex

	// If of the next request that we will send.
	nextID uint64

	// Count of requests sent since last stream opening.
	requestsSentSinceStreamOpen uint32

	// Requests that are pending to be sent.
	requestsToSend chan *omnitelpb.ExportRequest
}

// pendingRequest is the data type we keep in pendingAckMap and pendingAckList.
type pendingRequest struct {
	// The request that we sent.
	request *omnitelpb.ExportRequest

	// The deadline for the response to arrive.
	deadline time.Time
}

// NewClient creates a new Client with specified options. Call Connect() after this.
func NewClient(logger *zap.Logger) *Client {
	return &Client{logger: logger}
}

// Connect to the server endpoint using specified number of concurrent streams.
// Connect must block until it succeeds or fails (and return error in that case).
// If caller needs to interrupt a blocked Connect call the caller must close
// cancelCh, in that case Connect should return as soon as possible.
func (c *Client) Connect(options ConnectionOptions, cancelCh chan interface{}) error {
	if options.StreamReopenRequestCount == 0 {
		options.StreamReopenRequestCount = defStreamReopenRequestCount
	}

	if options.StreamReopenPeriod == 0 {
		options.StreamReopenPeriod = defStreamReopenPeriod
	}

	c.options = options

	// Set up a connection to the server. We will use blocking mode with cancellation
	// options.

	// First create a cancellable context.
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// Cancel if cancelCh signal is raised.
	go func() {
		<-cancelCh
		cancelFunc()
	}()

	// Now connect. This will block until connected or until cancelFunc is called.
	conn, err := grpc.DialContext(ctx, options.Endpoint, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}

	// Connection successful, create gRPC client.
	c.client = omnitelpb.NewOmnitelKClient(conn)

	if c.options.StreamCount < 1 {
		// At least one stream is required.
		c.options.StreamCount = 1
	}

	// Create queue of requests to send.
	c.requestsToSend = make(chan *omnitelpb.ExportRequest, c.options.StreamCount)

	// Create streams.
	for i := uint(0); i < c.options.StreamCount; i++ {
		stream, err := newClientStream(c)
		if err != nil {
			return err
		}
		c.clientStreams = append(c.clientStreams, stream)
	}
	return nil
}

// GetShardingConfig returns a sharding config from the server. May be called
// only after Connect succeeds.
func (c *Client) GetShardingConfig() (*omnitelpb.ShardingConfig, error) {
	return c.client.GetShardingConfig(context.Background(), &omnitelpb.ConfigRequest{})
}

// Send an encoded record to the server. The record must be encoded for the shard
// that is passed as the second parameter (record's partition key must be in the
// has key range of the shard). The call may block if the sending queue is full.
// This function will block if it wants to apply backpressure otherwise it may
// return as soon as the record is queued for delivery.
// The result of sending will be reported via OnSendResponse or OnSendFail
// callbacks.
func (c *Client) Send(record *omnitelpb.EncodedRecord, shard *omnitelpb.ShardDefinition) {
	request := &omnitelpb.ExportRequest{
		Records: []*omnitelpb.EncodedRecord{record},
		Shard:   shard,
	}

	if c.options.StreamCount == 1 {
		// One stream, it is faster to send request directly, bypassing the queue.
		c.clientStreams[0].sendRequest(request)
		return
	}

	// Make sure we have only up to c.streamCount Send calls in progress
	// concurrently.
	c.requestsToSend <- request
}

func newClientStream(client *Client) (*clientStream, error) {
	c := clientStream{}

	c.client = client
	c.requestsToSend = client.requestsToSend

	// Create ack pending data structures.
	c.pendingAckMap = make(map[uint64]*list.Element)
	c.pendingAckList = list.New()

	// Open the stream.
	if err := c.openStream(); err != nil {
		return nil, err
	}

	// Begin sending requests.
	go c.processSendRequests()

	// Begin processing request timeouts on the background.
	go c.processTimeouts()

	return &c, nil
}

func (c *clientStream) openStream() error {
	var err error
	c.stream, err = c.client.client.Export(context.Background())
	if err != nil {
		return err
	}
	c.lastStreamOpen = time.Now()
	atomic.StoreUint32(&c.requestsSentSinceStreamOpen, 0)

	go c.readStream(c.stream)

	return nil
}

func (c *clientStream) readStream(stream omnitelpb.OmnitelK_ExportClient) {
	for {
		response, err := stream.Recv()
		if err == io.EOF {
			// TODO: when can this happen and what do we need to do?
			// Most likely this happens when server closes the stream. We will need
			// to reconnect.
			return
		}
		if err != nil {
			// TODO: when can this happen and what do we need to do?
			// Most likely this happens when server closes the stream and returns
			// an error to RP call. We will need to reconnect.
			// This can probably also happen when we close the stream, so need
			// to be careful not to reconnect in that case.
			return
		}

		c.pendingAckMutex.Lock()
		elem, ok := c.pendingAckMap[response.Id]
		if !ok {
			// Response id is not found in pending requests. Should not normally happen,
			// indicates server misbehavior.
		} else {
			delete(c.pendingAckMap, response.Id)
			c.pendingAckList.Remove(elem)
		}
		c.pendingAckMutex.Unlock()

		pr := elem.Value.(pendingRequest)
		c.client.options.OnSendResponse(pr.request.Records, response)
	}
}

// processTimeouts checks pending requests and fails old ones.
func (c *clientStream) processTimeouts() {
	// Check once per second.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for now := range ticker.C {
		for {
			c.pendingAckMutex.Lock()

			// Get the oldest pending request.
			elem := c.pendingAckList.Back()
			if elem == nil {
				// No pending requests, nothing to do.
				c.pendingAckMutex.Unlock()
				break
			}

			pr := elem.Value.(pendingRequest)

			// Check request deadline.
			if pr.deadline.Before(now) {
				// We are past deadline, need to fail the request.
				c.pendingAckList.Remove(elem)
				delete(c.pendingAckMap, pr.request.Id)
				c.pendingAckMutex.Unlock()
				c.client.options.OnSendFail(pr.request.Records, FailedNotRetryable)
			} else {
				c.pendingAckMutex.Unlock()
				break
			}
		}
	}
}

func (c *clientStream) processSendRequests() {
	for request := range c.requestsToSend {
		c.sendRequest(request)
	}
}

func (c *clientStream) sendRequest(
	request *omnitelpb.ExportRequest,
) {
	// Send the batch via stream.
	request.Id = atomic.AddUint64(&c.nextID, 1)

	pr := pendingRequest{request: request, deadline: time.Now().Add(30 * time.Second)}

	// Add the ID to pendingAckMap map
	c.pendingAckMutex.Lock()
	elem := c.pendingAckList.PushFront(pr)
	c.pendingAckMap[request.Id] = elem
	c.pendingAckMutex.Unlock()

	if err := c.stream.Send(request); err != nil {
		// TODO: refactor reopening logic to happen with retries.
		if err == io.EOF {
			// Server closed the stream or disconnected. Try reopening the stream once.
			time.Sleep(1 * time.Second)
			if err = c.openStream(); err != nil {
				c.client.logger.Error("Error opening stream", zap.Error(err))
			}
			// We need to re-send all requests that are not yet acknowledged.
			c.resendPending()
		} else {
			c.client.logger.Error("Cannot send request", zap.Error(err))
			c.client.options.OnSendFail(request.Records, FailedNotRetryable)
		}
	}

	requestsSent := atomic.AddUint32(&c.requestsSentSinceStreamOpen, 1)

	// Check if time to re-open the stream.
	if requestsSent >= c.client.options.StreamReopenRequestCount ||
		time.Since(c.lastStreamOpen) >= c.client.options.StreamReopenPeriod {
		// Close and reopen the stream.
		c.lastStreamOpen = time.Now()
		err := c.stream.CloseSend()
		if err != nil {
			c.client.logger.Error("Cannot close stream", zap.Error(err))
		}
		if err = c.openStream(); err != nil {
			c.client.logger.Error("Cannot open stream", zap.Error(err))
		}
		// TODO: refactor reopening logic to happen with retries.
	}
}

// resendPending re-sends requests which are in the pending map. This is required
// when we re-connect after connection failure.
func (c *clientStream) resendPending() {
	// TODO: this function is not re-entrant. If somehow we end up calling it while
	// it is still in progress we will send duplicates of items in the pendingAckMap.
	// Refactor this to avoid that problem.

	var requests []*omnitelpb.ExportRequest
	// Make a copy of the pending requests list.
	c.pendingAckMutex.Lock()
	for _, request := range c.pendingAckMap {
		requests = append(requests, request.Value.(*omnitelpb.ExportRequest))
	}
	c.pendingAckMutex.Unlock()

	// Mutex is unlocked asap, now we can send the requests.
	for _, request := range requests {
		if err := c.stream.Send(request); err != nil {
			c.client.options.OnSendFail(request.Records, FailedNotRetryable)
		}
	}
}
