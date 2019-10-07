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

	jaeger "github.com/jaegertracing/jaeger/model"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	omnishardpb "github.com/Omnition/omnition-opentelemetry-collector/exporter/omnishard/gen"
)

// ClientUnary can connect to a server and send an ExportRequest. It uses multiple
// concurrent unary calls to increase throughput.
type ClientUnary struct {
	// gRPC client.
	client omnishardpb.OmniShardClient

	options ConnectionOptions

	// Cached context that is the basis for other contexts used by the client.
	baseCtx context.Context

	// Requests that are pending to be sent.
	requestsToSend chan requestToSend

	done chan struct{}

	logger *zap.Logger
}

// requestToSend is the data type we keep in pendingAckMap and pendingAckList
// and in the requestsToSend queues.
type requestToSend struct {
	// Ready to send request containing spans encoded into a EncodedRecord.
	exportRequest *omnishardpb.ExportRequest

	// originalSpans represents original spans that were encoded into record.
	originalSpans []*jaeger.Span
}

var _ client = (*ClientUnary)(nil)

// NewClientUnary creates a new ClientUnary with specified options. Call Connect() after this.
func NewClientUnary(logger *zap.Logger) *ClientUnary {
	return &ClientUnary{
		done:   make(chan struct{}),
		logger: logger,
	}
}

// Connect to the server endpoint using specified number of concurrent streams.
// Connect must block until it succeeds or fails (and return error in that case).
// If caller needs to interrupt a blocked Connect call the caller must close
// cancelCh, in that case Connect should return as soon as possible.
func (c *ClientUnary) Connect(options ConnectionOptions, cancelCh chan interface{}) error {
	c.options = options

	// Set up a connection to the server. We will use blocking mode with cancellation
	// options.

	// Create the base context for RPC messages.
	c.baseCtx = context.Background()
	if len(options.Headers) > 0 {
		c.baseCtx = metadata.NewOutgoingContext(c.baseCtx, metadata.New(options.Headers))
	}

	// Create a cancellable context.
	ctx, cancelFunc := context.WithCancel(c.baseCtx)
	defer cancelFunc()

	// Cancel if cancelCh signal is raised.
	go func() {
		<-cancelCh
		cancelFunc()
	}()

	// Build the gRPC dial options.
	grpcOptions := []grpc.DialOption{
		grpc.WithBlock(),
	}
	if !options.UseSecure {
		grpcOptions = append(grpcOptions, grpc.WithInsecure())
	}

	// Now connect. This will block until connected or until cancelFunc is called.
	conn, err := grpc.DialContext(ctx, options.Endpoint, grpcOptions...)
	if err != nil {
		return err
	}

	// Connection successful, create gRPC client.
	c.client = omnishardpb.NewOmniShardClient(conn)

	// Create queue of requests to send.
	c.requestsToSend = make(chan requestToSend, c.options.SendConcurrency)

	for i := uint(0); i < options.SendConcurrency; i++ {
		go c.processSendRequests()
	}

	return nil
}

// Shutdown the client. After this call Send() should not be called anymore.
// Any requests that are not sent yet will not be sent. The responses to already
// sent requests may continue arriving after Shutdown() call returns.
func (c *ClientUnary) Shutdown() {
	close(c.done)
}

// GetShardingConfig returns a sharding config from the server. May be called
// only after Connect succeeds.
func (c *ClientUnary) GetShardingConfig() (*omnishardpb.ShardingConfig, error) {
	return c.client.GetShardingConfig(c.baseCtx, &omnishardpb.ConfigRequest{})
}

// Send an encoded record to the server. The record must be encoded for the shard
// that is passed as the second parameter (record's partition key must be in the
// has key range of the shard). The call may block if the sending queue is full.
// This function will block if it wants to apply backpressure otherwise it may
// return as soon as the record is queued for delivery.
// The result of sending will be reported via OnSendResponse or OnSendFail
// callbacks.
// originalSpans represents original spans that are encoded into record.
// It is required that these 2 fields match each other.
func (c *ClientUnary) Send(record *omnishardpb.EncodedRecord, originalSpans []*jaeger.Span, shard *omnishardpb.ShardDefinition) {

	exportRequest := &omnishardpb.ExportRequest{
		Record: record,
		Shard:  shard,
	}

	// Make sure we have only up to c.streamCount Send calls in progress
	// concurrently.
	c.requestsToSend <- requestToSend{
		exportRequest: exportRequest,
		originalSpans: originalSpans,
	}
}

func (c *ClientUnary) processSendRequests() {
	for {
		select {
		case <-c.done:
			return

		case request := <-c.requestsToSend:
			c.sendRequest(request)
		}
	}
}

func (c *ClientUnary) sendRequest(pr requestToSend) {
	// Send the batch via stream.
	exportRequest := pr.exportRequest

	response, err := c.client.Export(c.baseCtx, exportRequest)
	if err != nil {
		// Check if this is a throttling response from the server.
		st := status.Convert(err)
		for _, detail := range st.Details() {
			switch t := detail.(type) {
			case *errdetails.RetryInfo:
				if t.RetryDelay.Seconds > 0 || t.RetryDelay.Nanos > 0 {
					// TODO: Wait before retrying.
					c.options.OnSendFail(exportRequest.Record, pr.originalSpans, ErrFailedRetryable)
					return
				}
			}
		}

		// Some other error, probably cannot serialize because we have bad data. Drop the data.
		c.logger.Error("Cannot send request", zap.Error(err))
		c.options.OnSendFail(exportRequest.Record, pr.originalSpans, ErrFailedNotRetryable)
	} else {
		c.options.OnSendResponse(pr.exportRequest.Record, pr.originalSpans, response)
	}
}
