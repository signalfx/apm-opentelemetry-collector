module github.com/Omnition/omnition-opentelemetry-collector

go 1.12

require (
	contrib.go.opencensus.io/exporter/ocagent v0.6.0
	contrib.go.opencensus.io/resource v0.1.2
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/census-instrumentation/opencensus-proto v0.2.2
	github.com/client9/misspell v0.3.4
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/gogo/protobuf v1.3.0
	github.com/golang/protobuf v1.3.2
	github.com/google/addlicense v0.0.0-20190510175307-22550fa7c1b0
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/grpc-gateway v1.11.1
	github.com/jaegertracing/jaeger v1.14.0
	github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9 // indirect
	github.com/omnition/gogoproto-rewriter v0.0.0-20190723134119-239e2d24817f
	github.com/omnition/opencensus-go-exporter-kinesis v0.3.3-0.20190919185502-7031b700cdfe
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191016224815-dfabfb0c1d1e
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver v0.0.0-20191021165924-bb954188ac10
	github.com/rs/cors v1.6.0
	github.com/shirou/gopsutil v2.19.9+incompatible
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.0-20190530013331-054be550cb49 // indirect
	github.com/signalfx/gohistogram v0.0.0-20160107210732-1ccfd2ff5083 // indirect
	github.com/signalfx/golib v2.5.1+incompatible
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/soheilhy/cmux v0.1.4
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.1
	go.uber.org/zap v1.10.0
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/net v0.0.0-20190724013045-ca1201d0de80
	golang.org/x/tools v0.0.0-20190906203814-12febf440ab1
	google.golang.org/genproto v0.0.0-20190801165951-fa694d86fc64
	google.golang.org/grpc v1.23.0
	honnef.co/go/tools v0.0.1-2019.2.3
	k8s.io/api v0.0.0-20190813020757-36bff7324fb7
	k8s.io/apimachinery v0.0.0-20190809020650-423f5d784010
	k8s.io/client-go v12.0.0+incompatible
)

replace contrib.go.opencensus.io/exporter/ocagent => github.com/omnition/opencensus-go-exporter-ocagent v0.6.0-omnition

replace github.com/census-instrumentation/opencensus-proto => github.com/omnition/opencensus-proto v0.2.1-gogo-unary

replace git.apache.org/thrift.git => github.com/apache/thrift v0.12.0
