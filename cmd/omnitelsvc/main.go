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

// Program omnitelsvc is the Omnition Telemetry Service built on top of
// OpenTelemetry Service.
package main

import (
	"log"
	"os"
	"strconv"

	"github.com/open-telemetry/opentelemetry-service/service"

	"github.com/Omnition/omnition-opentelemetry-service/extension/telemetryextension"
)

func main() {
	handleErr := func(err error) {
		if err != nil {
			log.Fatalf("Failed to run the service: %v", err)
		}
	}

	// Unfortunately, this is required to retrieve the command line arguments to the service.
	// The command line arguments aren't accessible via Viper to the extensions.
	// If the flags package is used, then it breaks the flags inherited from OpenTelemetry-Collector.
	// This is a temporary work around until config parsing exposes the command line arguments
	// in viper to the extensions.
	setOfArgs := os.Args[1:]
	argLen := len(setOfArgs)
	for i, arg := range setOfArgs {
		// Only set the metrics port global variable to be used within telemetry,
		// if the next argument is of type UINT and it doesn't cause an index of out bounds error.
		if arg == "--metrics-port" && i+1 < argLen {
			// Ensure that the next argument is of UINT type as what is expected in the flags.
			if _, err := strconv.ParseUint(setOfArgs[i+1], 10, 32); err == nil {
				telemetryextension.MetricPort = setOfArgs[i+1]
			}
		}

		// Only set the config  global variable to be used within telemetry,
		// if it doesn't cause an index of out bounds error.
		if arg == "--config" && i+1 < argLen {
			telemetryextension.ConfigFile = setOfArgs[i+1]
		}
	}

	factories, err := components()
	handleErr(err)

	svc := service.New(factories)
	err = svc.StartUnified()
	handleErr(err)
}
