// Copyright 2019 Omnition Authors
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

package signalfxexporter

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/signalfx/golib/sfxclient"
	"go.uber.org/zap"

	signalfxtranslator "github.com/Omnition/omnition-opentelemetry-collector/translator/metric/signalfx"
)

type sfxMetricExporter struct {
	logger *zap.Logger
	client *sfxclient.HTTPSink
}

// NewExporter returns a new SignalFX metrics exporter.
func NewExporter(logger *zap.Logger, cfg *Config) (exporter.MetricsExporter, error) {
	exp := &sfxMetricExporter{
		logger: logger,
		client: sfxclient.NewHTTPSink(),
	}
	exp.client.AuthToken = cfg.AuthToken
	exp.client.Client.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	exp.client.DatapointEndpoint = fmt.Sprintf("https://ingest.%s.signalfx.com/v2/datapoint", cfg.Realm)
	return exp, nil
}

// Start is a noop at present.
func (sfx *sfxMetricExporter) Start(host exporter.Host) error {
	sfx.logger.Warn(
		"!!! SignalFx exporter is a major work in progress and still pre-alpha. DO NOT USE IN PRODUCTION",
	)
	return nil
}

// Stop is a noop at present.
func (sfx *sfxMetricExporter) Shutdown() error {
	return nil
}

// ConsumeMetricsData takes an OpenCensus MetricsData object and exports it to the SignalFx backend.
func (sfx *sfxMetricExporter) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	// convert to data points
	dps, err := signalfxtranslator.OCProtoToSignalFx(sfx.logger, md)
	if err != nil {
		return err
	}
	err = sfx.client.AddDatapoints(ctx, dps)
	if err != nil {
		sfx.logger.Error("error exporting to SignalFx", zap.Error(err))
	}
	return nil
}
