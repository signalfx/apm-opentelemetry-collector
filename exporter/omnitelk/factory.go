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

	"github.com/open-telemetry/opentelemetry-service/config/configerror"
	"github.com/open-telemetry/opentelemetry-service/config/configmodels"
	"github.com/open-telemetry/opentelemetry-service/exporter"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr = "omnitelk"

	// Default values for config options.
	defStreams                  = 1
	defStreamReopenPeriod       = 30 * time.Second
	defStreamReopenRequestCount = 1000
)

// Factory is the factory for OmnitelK exporter.
type Factory struct {
}

// Type gets the type of the Exporter config created by this factory.
func (f *Factory) Type() string {
	return typeStr
}

// CreateDefaultConfig creates the default configuration for exporter.
func (f *Factory) CreateDefaultConfig() configmodels.Exporter {
	return &Config{
		Streams:                  defStreams,
		StreamReopenPeriod:       defStreamReopenPeriod,
		StreamReopenRequestCount: defStreamReopenRequestCount,
	}
}

// CreateTraceExporter initializes and returns a new trace exporter
func (f *Factory) CreateTraceExporter(logger *zap.Logger, cfg configmodels.Exporter) (exporter.TraceExporter, error) {
	config := cfg.(*Config)
	if config.Streams < 1 {
		config.Streams = 1
	}

	// TODO: Create an exporter like this when Exporter and ClientImpl are ready.
	// e := NewExporter(config, logger, NewClient(config.Endpoint, config.Streams, logger))
	// e.Start()
	// return e, nil

	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsExporter creates a metrics exporter based on this config.
func (f *Factory) CreateMetricsExporter(logger *zap.Logger, cfg configmodels.Exporter) (exporter.MetricsExporter, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}
