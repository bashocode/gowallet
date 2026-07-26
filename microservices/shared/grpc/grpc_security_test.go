package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	sharedGRPC "github.com/bashocode/gowallet/microservices/shared/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Dummy server handler for test
func dummyHandler(ctx context.Context, req any) (any, error) {
	return "ok", nil
}

func TestRequireServiceIdentity_MissingIdentity(t *testing.T) {
	interceptor := sharedGRPC.RequireServiceIdentity(true, "allowed-service")
	ctx := context.Background()

	info := &grpc.UnaryServerInfo{FullMethod: "/test.TestService/TestMethod"}
	_, err := interceptor(ctx, "req", info, dummyHandler)

	if err == nil {
		t.Fatal("expected error for missing service identity, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated status code, got %v", err)
	}
}

func TestRequireServiceIdentity_UnauthorizedCaller(t *testing.T) {
	interceptor := sharedGRPC.RequireServiceIdentity(true, "transaction-service")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(sharedGRPC.ServiceIdentityMetadataKey, "unauthorized-service"))

	info := &grpc.UnaryServerInfo{FullMethod: "/test.TestService/TestMethod"}
	_, err := interceptor(ctx, "req", info, dummyHandler)

	if err == nil {
		t.Fatal("expected error for unauthorized caller, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied status code, got %v", err)
	}
}

func TestRequireServiceIdentity_AllowedCaller(t *testing.T) {
	interceptor := sharedGRPC.RequireServiceIdentity(true, "transaction-service", "wallet-service")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(sharedGRPC.ServiceIdentityMetadataKey, "transaction-service"))

	info := &grpc.UnaryServerInfo{FullMethod: "/test.TestService/TestMethod"}
	res, err := interceptor(ctx, "req", info, dummyHandler)

	if err != nil {
		t.Fatalf("expected allowed caller to succeed, got %v", err)
	}
	if res != "ok" {
		t.Fatalf("expected 'ok', got %v", res)
	}
}

func TestUnaryClientTimeout_AppliesDeadline(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	defer srv.Stop()

	go srv.Serve(lis)

	var deadline time.Time
	var deadlineSet bool

	customClientInterceptor := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		deadline, deadlineSet = ctx.Deadline()
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientTimeout(100*time.Millisecond),
			customClientInterceptor,
		),
	)
	if err != nil {
		t.Fatalf("failed to connect client: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	_ = conn.Invoke(ctx, "/nonexistent.Service/Method", nil, nil)

	if !deadlineSet {
		t.Error("expected deadline to be set by client timeout interceptor")
	}
	if deadline.IsZero() {
		t.Error("expected valid deadline time")
	}
}
