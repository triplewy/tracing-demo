package main

import (
	fmt "fmt"
	"net"
	"os"
	"time"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.uber.org/zap"
	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

var (
	zLogger *zap.Logger
	sugar   *zap.SugaredLogger

	port string
)

func init() {
	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()

	if v := os.Getenv("PORT"); v != "" {
		port = v
	} else {
		sugar.Fatal("PORT not found in ENV")
	}
}

func main() {
	defer zLogger.Sync()

	sugar.Infof("server running on port %v", port)
	run(port)
	select {}
}

func run(port string) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		sugar.Fatal(err)
	}
	srv := grpc.NewServer()
	svc := &service{}
	RegisterServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	go srv.Serve(l)
}

type service struct{}

func (s *service) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (s *service) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (s *service) Echo(ctx context.Context, in *Empty) (*Empty, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		sugar.Infof("Metadata:")
		for key, value := range md {
			fmt.Printf("key: %v, value: %v\n", key, value)
		}
	}

	// Sleep for 1 second to simulate request processing
	time.Sleep(1 * time.Second)

	return &Empty{}, nil
}
