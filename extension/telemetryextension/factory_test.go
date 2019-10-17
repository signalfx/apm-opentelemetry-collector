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
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/config/configcheck"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/stretchr/testify/assert"
)

func TestFactory_Type(t *testing.T) {
	factory := Factory{}
	assert.Equal(t, typeStr, factory.Type())
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := Factory{}
	config := factory.CreateDefaultConfig()
	assert.Equal(t, config, &Config{
		ExtensionSettings: configmodels.ExtensionSettings{
			NameVal: typeStr,
			TypeVal: typeStr,
		},
	})
	assert.NoError(t, configcheck.ValidateConfig(config))
}
