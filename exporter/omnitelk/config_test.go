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
	"path"
	"testing"

	"github.com/open-telemetry/opentelemetry-service/config"
	"github.com/open-telemetry/opentelemetry-service/config/configmodels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Exporters[factory.Type()] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "default.yaml"), factories,
	)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters["omnitelk"]

	assert.Equal(t, e,
		&Config{
			ExporterSettings: configmodels.ExporterSettings{
				TypeVal: "omnitelk",
				NameVal: "omnitelk",
			},
			Streams: 1,
		},
	)
}

func TestConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Exporters[factory.Type()] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters["omnitelk"]

	assert.Equal(t, e,
		&Config{
			ExporterSettings: configmodels.ExporterSettings{
				TypeVal: "omnitelk",
				NameVal: "omnitelk",
			},
			Endpoint: "0.0.0.0:1234",
			Streams:  456,
		},
	)
}
