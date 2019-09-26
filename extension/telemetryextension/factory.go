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
	"github.com/open-telemetry/opentelemetry-service/config/configmodels"
	"github.com/open-telemetry/opentelemetry-service/extension"
	"go.uber.org/zap"
)

const (
	// typeStr is the value of "type" key in configuration.
	typeStr = "telemetry"
)

var _ (extension.Factory) = (*Factory)(nil)

// Factory is the factory of the telemetry extension.
type Factory struct {
}

// Type gets the type of the config created by this factory.
func (f *Factory) Type() string {
	return typeStr
}

// CreateDefaultConfig creates the default configuration for the extension.
func (f *Factory) CreateDefaultConfig() configmodels.Extension {
	return &Config{
		ExtensionSettings: configmodels.ExtensionSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		SourceURL: "http://localhost:8888/metrics",
	}
}

// CreateExtension creates a service extension based on the given config.
func (f *Factory) CreateExtension(logger *zap.Logger, cfg configmodels.Extension) (extension.ServiceExtension, error) {
	return nil, nil

}
