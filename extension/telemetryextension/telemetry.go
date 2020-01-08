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
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector/extension"
	"go.uber.org/zap"

	"github.com/signalfx/apm-opentelemetry-collector/internal/version"
)

const (
	contentEncodingHeader = "Content-Encoding"
	contentEncodingGZIP   = "gzip"
	instanceGitHashHeader = "X-Instance-GitSha"
	instanceUUIDHeader    = "X-Instance-Uuid"
	instanceVersionHeader = "X-Instance-Version"
	telemetryDataHeader   = "X-Telemetry-Data-Type"
	telemetryDataLogTag   = "telemetryDataType"
)

var (
	defaultMinIntervalToPostConfig = 180 * time.Second
	defaultScrapeInterval          = 10 * time.Second
	defaultRequestTimeout          = 5 * time.Second
)

type telemetryExtension struct {
	scrapper        *http.Client
	scrapeRq        *http.Request
	dispatcher      *http.Client
	logger          *zap.Logger
	sourceURL       string
	goEnvBytes      []byte
	configBytes     []byte
	lastCfgPostTime time.Time
	uuidStr         string
	cfg             Config
	done            chan struct{}
}

var _ (extension.ServiceExtension) = (*telemetryExtension)(nil)

func newTelemetryExtension(config Config, logger *zap.Logger) (*telemetryExtension, error) {

	// Unfortunately, this is required to retrieve the command line arguments to the service.
	// The command line arguments aren't accessible via Viper to the extensions.
	// If the flags package is used, then it breaks the flags inherited from OpenTelemetry-Collector.
	// This is a temporary work around until config parsing exposes the command line arguments
	// in viper to the extensions.
	metricsPort := ""
	configFile := ""
	setOfArgs := os.Args[1:]
	argLen := len(setOfArgs)
	for i, arg := range setOfArgs {
		// Only set the metrics port global variable to be used within telemetry,
		// if the next argument is of type UINT and it doesn't cause an index of out bounds error.
		if arg == "--metrics-port" && i+1 < argLen {
			// Ensure that the next argument is of UINT type as what is expected in the flags.
			if _, err := strconv.ParseUint(setOfArgs[i+1], 10, 32); err == nil {
				metricsPort = setOfArgs[i+1]
			}
		}

		// Only set the config  global variable to be used within telemetry,
		// if it doesn't cause an index of out bounds error.
		if arg == "--config" && i+1 < argLen {
			configFile = setOfArgs[i+1]
		}
	}

	// The config file is required for configuring the collector.
	if configFile == "" {
		return nil, errors.New("missing configuration file for service")
	}

	// Ensures the endpoint passed in through the configuration is valid.
	_, err := http.NewRequest("POST", config.Endpoint, nil)
	if err != nil {
		return nil, err
	}

	// The default value is pulled from
	// https://github.com/open-telemetry/opentelemetry-collector/blob/master/service/telemetry.go#L53
	if metricsPort == "" {
		metricsPort = "8888"
	}
	sourceURL := fmt.Sprintf("http://localhost:%s/metrics", metricsPort)

	// Ensures the source URL created is valid..
	scrapeRq, err := http.NewRequest("GET", sourceURL, nil)
	if err != nil {
		return nil, err
	}

	// Set the expected headers.
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	uuidStr := uuid.String()
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}
	config.Headers[instanceGitHashHeader] = version.GitHash
	config.Headers[instanceUUIDHeader] = uuidStr
	config.Headers[instanceVersionHeader] = version.Version
	config.Headers[contentEncodingHeader] = contentEncodingGZIP

	// The Go Environment values are only processed at collector start so there
	// is no need to refresh this value.
	goEnvBytes, err := gzipBytes(captureGoEnvBytes())
	if err != nil {
		return nil, err
	}

	// Similar to Go Environment values, the config is only processed
	// at collector start so there is no need to refresh the value.
	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	svcConfigGzipBytes, err := gzipBytes(yamlFile)
	if err != nil {
		return nil, err
	}
	te := &telemetryExtension{
		scrapper: &http.Client{
			Timeout: defaultRequestTimeout,
		},
		scrapeRq: scrapeRq,
		dispatcher: &http.Client{
			Timeout: defaultRequestTimeout,
		},
		cfg:         config,
		goEnvBytes:  goEnvBytes,
		configBytes: svcConfigGzipBytes,
		logger:      logger,
		uuidStr:     uuidStr,
		done:        make(chan struct{}),
		sourceURL:   sourceURL,
	}

	return te, nil
}

func (te *telemetryExtension) Start(host extension.Host) error {
	te.logger.Info(fmt.Sprintf("Starting telemetry extension with endpoint: %q at scrape interval %q from source %q", te.cfg.Endpoint, te.cfg.ScrapeInterval.String(), te.sourceURL))
	ticker := time.NewTicker(te.cfg.ScrapeInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				te.collectAndPostTelemetry()
			case <-te.done:
				return
			}
		}
	}()

	return nil
}

func (te *telemetryExtension) Shutdown() error {
	close(te.done)

	return nil
}

func (te *telemetryExtension) dispatch(body []byte, telemetryData string) {
	reader := bytes.NewReader(body)
	postReq, err := http.NewRequest("POST", te.cfg.Endpoint, reader)
	if err != nil {
		te.logger.Warn("telemetry-dispatcher: failed to create POST request", zap.Error(err), zap.String(telemetryDataLogTag, telemetryData), zap.String("uuid", te.uuidStr))
		return
	}
	for k, v := range te.cfg.Headers {
		postReq.Header.Add(k, v)
	}
	postReq.Header.Add(telemetryDataHeader, telemetryData)

	resp, err := te.dispatcher.Do(postReq)
	if err != nil {
		te.logger.Warn("telemetry-dispatcher: dispatch error", zap.Error(err), zap.String(telemetryDataLogTag, telemetryData), zap.String("uuid", te.uuidStr))
		return
	}
	te.logger.Debug("telemetry-dispatcher: dispatch success", zap.String("status", resp.Status), zap.String(telemetryDataLogTag, telemetryData), zap.String("uuid", te.uuidStr))
}

func (te *telemetryExtension) collectAndPostTelemetry() {
	te.getAndPostMetrics()
	if time.Since(te.lastCfgPostTime) > defaultMinIntervalToPostConfig {
		// This data doesn't change after re-start but the cost of sending it is low and
		// makes this independent of failures on the backend.
		te.dispatch(te.goEnvBytes, "goenv")
		te.dispatch(te.configBytes, "config")
		te.lastCfgPostTime = time.Now()
	}
}

func (te *telemetryExtension) getAndPostMetrics() {
	resp, err := te.scrapper.Do(te.scrapeRq)
	if err != nil {
		te.logger.Warn("prom-dispatcher: request error", zap.Error(err), zap.String("uuid", te.uuidStr))
		return
	}
	te.logger.Debug("prom-dispatcher: request success", zap.String("status", resp.Status), zap.String("uuid", te.uuidStr))

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		te.logger.Warn("prom-dispatcher: failed to read GET body", zap.Error(err), zap.String("uuid", te.uuidStr))
		return
	}

	body, err = gzipBytes(body)
	if err != nil {
		te.logger.Warn("prom-dispatcher: failed to gzip body", zap.Error(err), zap.String("uuid", te.uuidStr))
		return
	}

	te.dispatch(body, "metrics")
}

func gzipBytes(buf []byte) ([]byte, error) {
	var gzbuf bytes.Buffer
	zw := gzip.NewWriter(&gzbuf)
	_, err := zw.Write(buf)
	if err != nil {
		zw.Close()
		return nil, err
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}

	return gzbuf.Bytes(), nil
}

func captureGoEnvBytes() []byte {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("GOARCH=%s\n", runtime.GOARCH))
	buf.WriteString(fmt.Sprintf("GODEBUG=%s\n", os.Getenv("GODEBUG")))
	buf.WriteString(fmt.Sprintf("GOGC=%s\n", os.Getenv("GOGC")))
	buf.WriteString(fmt.Sprintf("GOMAXPROCS=%s\n", os.Getenv("GOMAXPROCS")))
	buf.WriteString(fmt.Sprintf("GOOS=%s\n", runtime.GOOS))
	buf.WriteString(fmt.Sprintf("GOTRACEBACK=%s\n", os.Getenv("GOTRACEBACK")))

	return buf.Bytes()
}
