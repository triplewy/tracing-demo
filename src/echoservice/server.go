package main

import (
	"fmt"
	"net"
	"os"
	"time"

	//"go.opentelemetry.io/otel/api/global"
	//"go.opentelemetry.io/otel/plugin/grpctrace"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	//"github.com/openzipkin/zipkin-go/propagation/b3"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	zLogger *zap.Logger
	sugar   *zap.SugaredLogger

	cc *grpc.ClientConn

	port    string
	svcAddr string
)

func init() {
	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()

	if v := os.Getenv("PORT"); v != "" {
		port = v
	} else {
		sugar.Fatal("PORT not found in ENV")
	}

	if v := os.Getenv("SVC_ADDR"); v != "" {
		svcAddr = v
	} else {
		sugar.Fatal("SVC_ADDR not found in ENV")
	}

	var err error
	if cc, err = dial(); err != nil {
		sugar.Fatal(err)
	}
}

func dial() (*grpc.ClientConn, error) {
	return grpc.Dial(
		svcAddr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(UnaryClientInterceptor),
	)
}

// UnaryClientInterceptor for passing incoming metadata to outgoing metadata
func UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption) error {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		sugar.Infof("Metadata:")
		for key, value := range md {
			fmt.Printf("key: %v, value: %v\n", key, value)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
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
	// Sleep for 1 second to simulate request processing
	time.Sleep(1 * time.Second)

	client := NewServiceClient(cc)

	reply, err := client.Echo(ctx, &Empty{})
	if err != nil {
		cc, err = dial()
		if err != nil {
			return nil, err
		}
		client = NewServiceClient(cc)
		return client.Echo(ctx, &Empty{})
	}
	return reply, nil
}
