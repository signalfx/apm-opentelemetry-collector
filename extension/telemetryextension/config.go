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

package telemetryextension

import (
	"time"

	"github.com/open-telemetry/opentelemetry-service/config/configmodels"
)

// Config has the configuration for the extension enabling the telemetry
// extension. It is used to send telemetry from the
type Config struct {
	configmodels.ExtensionSettings `mapstructure:",squash"`

	// Endpoint specifies the endpoint to send the telemetry to.
	// This field is required.
	Endpoint string `mapstructure:"endpoint"`

	// Headers specifies the headers to set on requests sent to the
	Headers map[string]string `mapstructure:"headers"`

	// ScrapeInterval specifies how often to scrape the service for telemetry.
	ScrapeInterval time.Duration `mapstructure:"scrape_interval"`

	// SourceUrl defines the local endpoint to retrieve metrics from.
	// It is usually `http://localhost:<metrics-port>/metrics` where
	// `metrics-port` is a command line argument to the application.
	// If not defined, the default value is `http://localhost:8888/metrics`
	SourceURL string `mapstructure:"source_url"`
}
