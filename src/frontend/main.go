package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const RETRIES = 3

var (
	zLogger *zap.Logger
	sugar   *zap.SugaredLogger

	port string
	svcAddr string

	cc *grpc.ClientConn
)

func init() {
	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()

	if val := os.Getenv("PORT"); val != "" {
		port = val
	} else {
		sugar.Fatalf("PORT not found in ENV")
	}

	if val := os.Getenv("SVC_ADDR"); val != "" {
		svcAddr = val
	} else {
		sugar.Fatalf("SVC_ADDR not found in ENV")
	}

	var err error
	if cc, err = dial(); err != nil {
		sugar.Fatal(err)
	}
}

func dial() (cc *grpc.ClientConn, err error) {
	for i := RETRIES; i > 0; i-- {
		cc, err = func () (*grpc.ClientConn, error) {
			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 2 * time.Second)
			defer cancel()

			return grpc.DialContext(
				ctx,
				svcAddr,
				grpc.WithInsecure(),
				grpc.WithBlock(),
				grpc.WithUnaryInterceptor(UnaryClientInterceptor),
			)
		}()
		if err == nil {
			sugar.Info("Connected to gRPC service")
			return
		}
	}
	return
}

func UnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		sugar.Infof("Metadata:")
		for key, value := range md {
			fmt.Printf("key: %v, value: %v\n", key, value)
		}
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

func main() {
	r := mux.NewRouter()
	// Add handlers
	r.HandleFunc("/", homeHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/_healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })

	// Add middleware
	r.Use(tracingMiddleware)

	sugar.Infof("starting server on :" + port)
	sugar.Fatal(http.ListenAndServe(":"+port, r))
}

func tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tracingHeaders := []string{
			"x-request-id",
			"x-b3-traceid",
			"x-b3-spanid",
			"x-b3-sampled",
			"x-b3-parentspanid",
			"x-b3-flags",
			"x-ot-span-context",
		}
		for _, key := range tracingHeaders {
			if val := r.Header.Get(key); val != "" {
				if key == "X-Request-Id" {
					fmt.Println("X-Request-Id: ", val)
				}
				ctx = metadata.AppendToOutgoingContext(ctx, key, val)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Sleep for 1 second to simulate request processing
	time.Sleep(1 * time.Second)

	client := NewServiceClient(cc)
	if _, err := client.Echo(r.Context(), &Empty{}); err != nil {
		http.Error(w, err.Error(), 500)
	} else {
		w.WriteHeader(200)
	}
}