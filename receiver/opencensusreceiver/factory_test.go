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

package opencensusreceiver

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
}

func TestCreateReceiver(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()

	config := cfg.(*Config)
	config.Endpoint = getAvailableLocalAddress(t)

	tReceiver, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)
	assert.NotNil(t, tReceiver)
	assert.Nil(t, err)

	mReceiver, err := factory.CreateMetricsReceiver(zap.NewNop(), cfg, nil)
	assert.NotNil(t, mReceiver)
	assert.Nil(t, err)
}

func TestCreateTraceReceiver(t *testing.T) {
	factory := Factory{}
	endpoint := getAvailableLocalAddress(t)
	defaultReceiverSettings := configmodels.ReceiverSettings{
		TypeVal:  typeStr,
		NameVal:  typeStr,
		Endpoint: endpoint,
	}
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "default",
			cfg: &Config{
				ReceiverSettings: defaultReceiverSettings,
			},
		},
		{
			name: "invalid_port",
			cfg: &Config{
				ReceiverSettings: configmodels.ReceiverSettings{
					TypeVal:  typeStr,
					NameVal:  typeStr,
					Endpoint: "127.0.0.1:112233",
				},
			},
			wantErr: true,
		},
		{
			name: "max-msg-size-and-concurrent-connections",
			cfg: &Config{
				ReceiverSettings:     defaultReceiverSettings,
				MaxRecvMsgSizeMiB:    32,
				MaxConcurrentStreams: 16,
			},
		},
	}
	ctx := context.Background()
	logger := zap.NewNop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(exportertest.SinkTraceExporter)
			tr, err := factory.CreateTraceReceiver(ctx, logger, tt.cfg, sink)
			if (err != nil) != tt.wantErr {
				t.Errorf("factory.CreateTraceReceiver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tr != nil {
				mh := component.NewMockHost()
				if err := tr.Start(mh); err == nil {
					tr.Shutdown()
				} else {
					t.Fatalf("StartTraceReception() error = %v", err)
				}
			}
		})
	}
}

func getAvailableLocalAddress(t *testing.T) string {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to get a free local port: %v", err)
	}
	// There is a possible race if something else takes this same port before
	// the test uses it, however, that is unlikely in practice.
	defer ln.Close()
	return ln.Addr().String()
}
