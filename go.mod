module github.com/Omnition/omnition-opentelemetry-service

go 1.12

require (
	contrib.go.opencensus.io/exporter/ocagent v0.6.0
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/census-instrumentation/opencensus-proto v0.2.2
	github.com/client9/misspell v0.3.4
	github.com/gogo/protobuf v1.3.0
	github.com/golang/protobuf v1.3.2
	github.com/google/addlicense v0.0.0-20190510175307-22550fa7c1b0
	github.com/grpc-ecosystem/grpc-gateway v1.11.1
	github.com/jaegertracing/jaeger v1.9.0
	github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/omnition/gogoproto-rewriter v0.0.0-20190723134119-239e2d24817f
	github.com/omnition/opencensus-go-exporter-kinesis v0.3.2
	github.com/open-telemetry/opentelemetry-service v0.0.2-0.20190909135303-35ecb0299990
	github.com/rs/cors v1.6.0
	github.com/soheilhy/cmux v0.1.4
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.1
	go.uber.org/zap v1.10.0
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7
	golang.org/x/tools v0.0.0-20190906203814-12febf440ab1
	google.golang.org/grpc v1.23.0
	honnef.co/go/tools v0.0.1-2019.2.3
)

replace contrib.go.opencensus.io/exporter/ocagent => github.com/omnition/opencensus-go-exporter-ocagent v0.6.0-omnition

replace github.com/census-instrumentation/opencensus-proto => github.com/omnition/opencensus-proto v0.2.1-gogo-unary
