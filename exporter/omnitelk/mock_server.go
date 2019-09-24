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

package omnitelk

import (
	"bytes"
	"context"
	"log"
	"math/rand"
	"net"
	"sync"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	omnitelpb "github.com/Omnition/omnition-opentelemetry-service/exporter/omnitelk/gen"
)

type gRPCServer struct {
	srv       *mockServer
	onReceive func(request *omnitelpb.ExportRequest)
}

func (s *gRPCServer) Export(ctx context.Context, request *omnitelpb.ExportRequest) (*omnitelpb.ExportResponse, error) {
	if s.srv.RandomServerError && rand.Float64() < 0.1 {
		status, err := status.New(codes.Unavailable, "Server is unavailable").
			WithDetails(&errdetails.RetryInfo{RetryDelay: &duration.Duration{Nanos: 1e6 * 100}})
		if err != nil {
			log.Fatal(err)
		}
		return nil, status.Err()
	}

	var response *omnitelpb.ExportResponse
	if !shardIsPartOfConfig(request.Shard, s.srv.GetConfig()) {
		// Client's config does not match our config.
		response = &omnitelpb.ExportResponse{
			Id:             request.Id,
			ResultCode:     omnitelpb.ExportResponse_SHARD_CONFIG_MISTMATCH,
			ShardingConfig: s.srv.GetConfig(),
		}
	} else {
		// Process received request.
		s.onReceive(request)

		// Send response to client.
		response = &omnitelpb.ExportResponse{
			Id:             request.Id,
			ResultCode:     s.srv.nextResponseCode,
			ShardingConfig: s.srv.nextResponseShardingConfig,
		}
	}

	return response, nil
}

func shardIsPartOfConfig(shard *omnitelpb.ShardDefinition, config *omnitelpb.ShardingConfig) bool {
	for _, s := range config.ShardDefinitions {
		if s.ShardId == shard.ShardId &&
			compareHashKey(s.StartingHashKey, shard.StartingHashKey) == 0 &&
			compareHashKey(s.EndingHashKey, shard.EndingHashKey) == 0 {
			return true
		}
	}
	return false
}

func compareHashKey(k1 []byte, k2 []byte) int {
	if k1 == nil {
		k1 = []byte{}
	}
	if k2 == nil {
		k2 = []byte{}
	}
	return bytes.Compare(k1, k2)
}

func (s *gRPCServer) GetShardingConfig(context.Context, *omnitelpb.ConfigRequest) (*omnitelpb.ShardingConfig, error) {
	return s.srv.GetConfig(), nil
}

type mockServer struct {
	Sink mockServerSink

	RandomServerError bool

	s                          *grpc.Server
	nextResponseCode           omnitelpb.ExportResponse_ResultCode
	nextResponseShardingConfig *omnitelpb.ShardingConfig

	config      *omnitelpb.ShardingConfig
	configMutex sync.Mutex
}

func newMockServer() *mockServer {
	server := &mockServer{
		s:      grpc.NewServer(),
		config: &omnitelpb.ShardingConfig{},
	}
	return server
}

func (srv *mockServer) GetConfig() *omnitelpb.ShardingConfig {
	srv.configMutex.Lock()
	defer srv.configMutex.Unlock()
	return srv.config
}

func (srv *mockServer) SetConfig(config *omnitelpb.ShardingConfig) {
	srv.configMutex.Lock()
	defer srv.configMutex.Unlock()
	srv.config = config
}

func (srv *mockServer) Listen(endpoint string, onReceive func(request *omnitelpb.ExportRequest)) error {
	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.Printf("failed to listen: %v", err)
	}
	omnitelpb.RegisterOmnitelKServer(srv.s, &gRPCServer{srv: srv, onReceive: onReceive})
	if err := srv.s.Serve(lis); err != nil {
		log.Printf("failed to serve: %v", err)
	}

	return nil
}

func (srv *mockServer) Stop() {
	srv.s.Stop()
}

func runServer(srv *mockServer, listenAddress string) {

	srv.Listen(listenAddress, func(request *omnitelpb.ExportRequest) {
		srv.Sink.onReceive(request)
	})
}

type mockServerSink struct {
	requests []*omnitelpb.ExportRequest
	records  []*omnitelpb.EncodedRecord
	mutex    sync.Mutex
}

func (mss *mockServerSink) onReceive(request *omnitelpb.ExportRequest) {
	mss.mutex.Lock()
	defer mss.mutex.Unlock()
	mss.requests = append(mss.requests, request)
	mss.records = append(mss.records, request.Record)
}

func (mss *mockServerSink) GetRecords() []*omnitelpb.EncodedRecord {
	mss.mutex.Lock()
	defer mss.mutex.Unlock()
	return mss.records
}
