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
	jaeger "github.com/jaegertracing/jaeger/model"

	omnitelpb "github.com/Omnition/omnition-opentelemetry-service/exporter/omnishard/gen"
)

// client allows to connect to a server, get sharding config and send encoded data.
type client interface {
	// Connect to the server endpoint using specified number of concurrent streams.
	// Connect must block until it succeeds or fails (and return error in that case).
	// If caller needs to interrupt a blocked Connect call the caller must close
	// "cancelCh" channel, in that case Connect should return as soon as possible.
	Connect(options ConnectionOptions, cancelCh chan interface{}) error

	// GetShardingConfig returns a sharding config from the server. May be called
	// only after Connect succeeds.
	GetShardingConfig() (*omnitelpb.ShardingConfig, error)

	// Send an encoded record to the server. The record must be encoded for the shard
	// that is passed as a parameter (record's partition key must be in the hash
	// key range of the shard).
	// This function will block if it wants to apply backpressure otherwise it may
	// return as soon as the record is queued for delivery.
	// The result of sending will be reported via OnSendResponse or OnSendFail
	// callbacks.
	// originalSpans represents original spans that are encoded into record.
	// It is required that these 2 fields match each other.
	Send(record *omnitelpb.EncodedRecord, originalSpans []*jaeger.Span, shard *omnitelpb.ShardDefinition)

	// Shutdown the client. After this call Send() should not be called anymore.
	// Any requests that are not sent yet will not be sent. The responses to already
	// sent requests may continue arriving after Shutdown() call returns.
	Shutdown()
}

// ConnectionOptions to use for the client.
type ConnectionOptions struct {
	// Server's address and port.
	Endpoint string

	// Controls if the default transport security is going to be disabled.
	DisableSecurity bool

	// Headers associated with the export requests.
	Headers map[string]string

	// Number of concurrent requests to use for sending ExportRequests.
	SendConcurrency uint

	// Callback called when a response is received regarding previously sent records.
	// Called asynchronously sometime after Send() successfully sends the record.
	// originalSpans represents original spans that were encoded into record and
	// to which the response was received.
	OnSendResponse func(
		responseToRecords *omnitelpb.EncodedRecord,
		originalSpans []*jaeger.Span,
		response *omnitelpb.ExportResponse,
	)

	// Callback called if the records cannot be sent for whatever reason (e.g. the
	// records cannot be serialized).
	OnSendFail func(
		failedRecords *omnitelpb.EncodedRecord,
		failedSpans []*jaeger.Span,
		code SendErrorCode,
	)
}

// SendErrorCode describes sending errors.
type SendErrorCode int

const (
	_ SendErrorCode = iota // skip 0 value.

	// ErrFailedRetryable indicates that Send() operation must be retried.
	ErrFailedRetryable

	// ErrFailedNotRetryable indicates that Send span data failed and
	// it should not be retried because the problem is fatal (e.g. bad data that
	// cannot be marshaled).
	ErrFailedNotRetryable
)
