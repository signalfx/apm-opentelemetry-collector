// Copyright 2019 OpenTelemetry Authors
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

package kubernetes

import (
	"github.com/open-telemetry/opentelemetry-service/config/configmodels"
)

// FieldExtractConfig ...
type FieldExtractConfig struct {
	Name  string `mapstructure:"name,omitempty"`
	Key   string `mapstructure:"key,omitempty"`
	Regex string `mapstructure:"regex,omitempty"`
}

// FilterConfig ...
type FilterConfig struct {
	Key   string `mapstructure:"key,omitempty"`
	Value string `mapstructure:"value,omitempty"`
	Op    string `mapstructure:"op,omitempty"`
}

// Config defines configuration for Attributes processor.
type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	// Passthrough mode only annotates resources with the pod IP and
	// does not try to extra any other metadata. It does not need require
	// access to the K8S cluster API. Agent/Collector must receive spans
	// directly from services to be able to correctly detect the pod IPs.
	Passthrough bool `mapstructure:"passthrough,omitempty"`

	// Extract section controls extraction of data from pods
	Extract struct {
		Metadata []string `mapstructure:"metadata,omitempty"`
		// Annotations []string             `mapstructure:"annotations,omitempty"`
		Annotations []FieldExtractConfig `mapstructure:"annotations,omitempty"`
		Labels      []FieldExtractConfig `mapstructure:"labels,omitempty"`
		// Labels      []string             `mapstructure:"labels,omitempty"`
		// Fields      []FieldExtractConfig `mapstructure:"fields,omitempty"`
	} `mapstructure:"extract,omitempty"`

	Filter struct {
		Node           string         `mapstructure:"node,omitempty"`
		NodeFromEnvVar string         `mapstructure:"node_from_env_var,omitempty"`
		Namespace      string         `mapstructure:"namespace,omitempty"`
		Fields         []FilterConfig `mapstructure:"fields,omitempty"`
		Labels         []FilterConfig `mapstructure:"labels,omitempty"`
	}
}
