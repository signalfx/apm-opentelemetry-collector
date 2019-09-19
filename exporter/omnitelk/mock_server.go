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
	"context"
	"io"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"

	omnitelpb "github.com/Omnition/omnition-opentelemetry-service/exporter/omnitelk/gen"
)

type gRPCServer struct {
	srv       *mockServer
	onReceive func(request *omnitelpb.ExportRequest)
}

func (s *gRPCServer) Export(stream omnitelpb.OmnitelK_ExportServer) error {
	for {
		// Wait for request from client.
		request, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if request.Id == 0 {
			log.Fatal("Received 0 Id")
		}

		// Process received request.
		s.onReceive(request)

		// Send response to client.
		response := &omnitelpb.ExportResponse{
			Id:             request.Id,
			ResultCode:     s.srv.nextResponseCode,
			ShardingConfig: s.srv.nextResponseShardingConfig,
		}
		if err := stream.Send(response); err != nil {
			return err
		}
	}
}

func (s *gRPCServer) GetShardingConfig(context.Context, *omnitelpb.ConfigRequest) (*omnitelpb.ShardingConfig, error) {
	return s.srv.config, nil
}

type mockServer struct {
	s                          *grpc.Server
	config                     *omnitelpb.ShardingConfig
	sink                       mockServerSink
	nextResponseCode           omnitelpb.ExportResponse_ResultCode
	nextResponseShardingConfig *omnitelpb.ShardingConfig
}

func (srv *mockServer) Listen(endpoint string, onReceive func(request *omnitelpb.ExportRequest)) error {
	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	srv.s = grpc.NewServer()
	omnitelpb.RegisterOmnitelKServer(srv.s, &gRPCServer{srv: srv, onReceive: onReceive})
	if err := srv.s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	return nil
}

func (srv *mockServer) Stop() {
	srv.s.Stop()
}

func runServer(srv *mockServer, listenAddress string) {

	log.Printf("Server: listening on %s", listenAddress)

	if srv.config == nil {
		srv.config = &omnitelpb.ShardingConfig{}
	}

	//totalSpans := 0
	//prevTime := time.Now()

	srv.Listen(listenAddress, func(request *omnitelpb.ExportRequest) {
		//t := time.Now()
		//d := t.Sub(prevTime)
		//prevTime = t

		//rate := float64(request.spanCount) / d.Seconds()
		//
		//totalSpans += spanCount
		//log.Printf("Server: total spans received %v, current rate %.1f", totalSpans, rate)

		srv.sink.onReceive(request)
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
	mss.records = append(mss.records, request.Records...)
}

func (mss *mockServerSink) getRecords() []*omnitelpb.EncodedRecord {
	mss.mutex.Lock()
	defer mss.mutex.Unlock()
	return mss.records
}
