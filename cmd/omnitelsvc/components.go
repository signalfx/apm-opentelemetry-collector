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

package main

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver"
	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/jaeger/jaegergrpcexporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/jaeger/jaegerthrifthttpexporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/loggingexporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/zipkinexporter"
	"github.com/open-telemetry/opentelemetry-collector/extension"
	"github.com/open-telemetry/opentelemetry-collector/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector/extension/pprofextension"
	"github.com/open-telemetry/opentelemetry-collector/extension/zpagesextension"
	"github.com/open-telemetry/opentelemetry-collector/oterr"
	"github.com/open-telemetry/opentelemetry-collector/processor"
	"github.com/open-telemetry/opentelemetry-collector/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector/processor/batchprocessor"
	"github.com/open-telemetry/opentelemetry-collector/processor/queuedprocessor"
	"github.com/open-telemetry/opentelemetry-collector/processor/samplingprocessor/probabilisticsamplerprocessor"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"github.com/open-telemetry/opentelemetry-collector/receiver/jaegerreceiver"
	"github.com/open-telemetry/opentelemetry-collector/receiver/zipkinreceiver"

	"github.com/signalfx/apm-opentelemetry-collector/exporter/kinesis"
	"github.com/signalfx/apm-opentelemetry-collector/exporter/omnishard"
	"github.com/signalfx/apm-opentelemetry-collector/exporter/opencensusexporter"
	signalfxexporter "github.com/signalfx/apm-opentelemetry-collector/exporter/signalfx"
	"github.com/signalfx/apm-opentelemetry-collector/extension/telemetryextension"
	"github.com/signalfx/apm-opentelemetry-collector/processor/k8sprocessor"
	"github.com/signalfx/apm-opentelemetry-collector/processor/memorylimiter"
	"github.com/signalfx/apm-opentelemetry-collector/receiver/memorymonitor"
	"github.com/signalfx/apm-opentelemetry-collector/receiver/opencensusreceiver"
)

func components() (config.Factories, error) {
	errs := []error{}
	receivers, err := receiver.Build(
		&jaegerreceiver.Factory{},
		&zipkinreceiver.Factory{},
		&zipkinscribereceiver.Factory{},
		&opencensusreceiver.Factory{},
		&memorymonitor.Factory{},
		&sapmreceiver.Factory{},
	)
	if err != nil {
		errs = append(errs, err)
	}

	exporters, err := exporter.Build(
		&opencensusexporter.Factory{},
		&prometheusexporter.Factory{},
		&loggingexporter.Factory{},
		&jaegergrpcexporter.Factory{},
		&jaegerthrifthttpexporter.Factory{},
		&zipkinexporter.Factory{},
		&kinesis.Factory{},
		&omnishard.Factory{},
		&signalfxexporter.Factory{},
		&sapmexporter.Factory{},
	)
	if err != nil {
		errs = append(errs, err)
	}

	processors, err := processor.Build(
		&attributesprocessor.Factory{},
		&k8sprocessor.Factory{},
		&queuedprocessor.Factory{},
		&batchprocessor.Factory{},
		&memorylimiter.Factory{},
		&probabilisticsamplerprocessor.Factory{},
	)
	if err != nil {
		errs = append(errs, err)
	}

	extensions, err := extension.Build(
		&healthcheckextension.Factory{},
		&pprofextension.Factory{},
		&telemetryextension.Factory{},
		&zpagesextension.Factory{},
	)
	if err != nil {
		errs = append(errs, err)
	}
	factories := config.Factories{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
		Extensions: extensions,
	}
	return factories, oterr.CombineErrors(errs)
}
