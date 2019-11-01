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

// DataDropCode describes reason why data was dropped.
type DataDropCode int

const (
	_ DataDropCode = iota // skip 0 value.

	// RetryQueueFull means that data needed to be dropped because the retry
	// queue was full.
	RetryQueueFull

	// FatalEncodingError indicates that data was dropped because there was an error
	// when trying to encode the data.
	FatalEncodingError

	// ExportResponseNotRetryable the response for the send message from the
	// server was ExportResponse_FAILED_NOT_RETRYABLE.
	ExportResponseNotRetryable

	// SendErrNotRetryable means that the send operation failed at the transport
	// level and it was not retryable.
	SendErrNotRetryable
)
