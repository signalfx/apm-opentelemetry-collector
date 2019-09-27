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
	"time"

	"github.com/open-telemetry/opentelemetry-service/config/configmodels"
)

// Config contains the main configuration options for the OmnitelK exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"`

	// Endpoint of the server to connect to.
	Endpoint string `mapstructure:"endpoint"`

	// Disables transport security for the client connection. Useful for private
	// networks or simple testing. By default security is enabled.
	DisableSecurity bool `mapstructure:"disable_security"`

	// The headers associated with the export requests.
	Headers map[string]string `mapstructure:"headers"`

	// Number of concurrent requests to use for sending ExportRequests.
	// Default value is 20. Higher values may be necessary to achieve good throughput.
	// TODO: run perf test and set a recommendation.
	SendConcurrency uint `mapstructure:"send_concurrency"`

	// Number of workers that will feed spans to shard encoders concurrently.
	// Minimum and default value is 1.
	NumWorkers uint `mapstructure:"num_workers"`

	// MaxRecordSize is the maximum size of one encoded uncompressed record in bytes.
	// When the record reaches this size, it gets sent to the server.
	MaxRecordSize uint `mapstructure:"max_record_size"`

	// BatchFlushInterval defines how often to flush batches of spans if they don't
	// reach the MaxRecordSize. The default value is defBatchFlushInterval.
	BatchFlushInterval time.Duration `mapstructure:"batch_flush_interval"`

	// MaxAllowedSizePerSpan limits the size of an individual span (in uncompressed encoded
	// bytes). If the encoded size is larger than this the encode will attempt to
	// drop some fields from the span and re-encode (see MaxAllowedSizePerSpan usage
	// in the code below for more details on what is dropped). If that doesn't help the
	// span will be dropped altogether. The default value is defMaxAllowedSizePerSpan.
	MaxAllowedSizePerSpan uint `mapstructure:"max_allowed_size_per_span"`
}
