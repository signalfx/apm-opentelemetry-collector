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
	"path"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-service/config"
	"github.com/open-telemetry/opentelemetry-service/config/configmodels"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {

	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Extensions[typeStr] = factory
	config, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	assert.Nil(t, err)
	assert.NotNil(t, config)

	e0 := config.Extensions["telemetry"]
	assert.Equal(t, e0, &Config{
		ExtensionSettings: configmodels.ExtensionSettings{
			NameVal: typeStr,
			TypeVal: typeStr,
		},
		Endpoint: "https://api.test.com/stuff",
	})

	e1 := config.Extensions["telemetry/1"]
	assert.Equal(t, e1, &Config{
		ExtensionSettings: configmodels.ExtensionSettings{
			NameVal: "telemetry/1",
			TypeVal: typeStr,
		},
		Endpoint: "https://api.test2.com/stuff",
		Headers: map[string]string{
			"x-auth-header": "12345",
		},
		ScrapeInterval: time.Duration(5 * time.Second),
	})
}
