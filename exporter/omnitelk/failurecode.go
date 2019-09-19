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

// FailureCode describes encoding or sending failures.
type FailureCode int

const (
	_ FailureCode = iota // skip 0 value.

	// FailedEncoderStopped indicates encoding attempt when encoder is stopped.
	FailedEncoderStopped

	// FailedNotRetryable indicates that encoding or sending span data failed and
	// it should not be retried because the problem is fatal (e.g. bad data that
	// cannot be marshaled).
	FailedNotRetryable

	// FailedShouldRetry indicates that encoding or sending span data failed but
	// that should be retried because the error is transient.
	FailedShouldRetry
)
