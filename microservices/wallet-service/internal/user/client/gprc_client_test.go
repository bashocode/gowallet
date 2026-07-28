package client

import (
	"context"
	"testing"
	"time"

	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockUserServiceClient struct {
	pb.UserServiceClient
	callCount      int
	shouldFail     bool
	failUntilCount int
}

func (m *mockUserServiceClient) GetUserByEmail(ctx context.Context, in *pb.GetUserByEmailRequest, opts ...grpc.CallOption) (*pb.UserResponse, error) {
	m.callCount++
	if m.shouldFail || m.callCount <= m.failUntilCount {
		return nil, status.Error(codes.Unavailable, "service unavailable")
	}
	return &pb.UserResponse{
		Id:       "user-123",
		FullName: "Test User",
		Email:    in.Email,
	}, nil
}

func (m *mockUserServiceClient) GetUserByID(ctx context.Context, in *pb.GetUserRequest, opts ...grpc.CallOption) (*pb.UserResponse, error) {
	m.callCount++
	if m.shouldFail || m.callCount <= m.failUntilCount {
		return nil, status.Error(codes.Unavailable, "service unavailable")
	}
	return &pb.UserResponse{
		Id:       in.Id,
		FullName: "Test User",
		Email:    "test@example.com",
	}, nil
}

func TestCircuitBreakerOpens(t *testing.T) {
	mock := &mockUserServiceClient{shouldFail: true}

	cbSettings := gobreaker.Settings{
		Name:        "test-cb",
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
	}

	client := &ProtectedUserClient{
		client: mock,
		cb:     gobreaker.NewCircuitBreaker(cbSettings),
	}

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := client.GetUserByEmail(ctx, "test@example.com")
		if err != nil {
			t.Logf("Attempt %d: %v", i+1, err)
		}
	}

	if mock.callCount >= 5 {
		t.Errorf("Circuit breaker did not open. Expected fewer calls, got %d", mock.callCount)
	}

	state := client.cb.State()
	if state != gobreaker.StateOpen {
		t.Errorf("Expected circuit breaker to be OPEN, got %v", state)
	}
}

func TestCircuitBreakerRecovers(t *testing.T) {
	mock := &mockUserServiceClient{failUntilCount: 3}

	cbSettings := gobreaker.Settings{
		Name:        "test-cb-recovery",
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     1 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 2
		},
	}

	client := &ProtectedUserClient{
		client: mock,
		cb:     gobreaker.NewCircuitBreaker(cbSettings),
	}

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _ = client.GetUserByEmail(ctx, "test@example.com")
	}

	if client.cb.State() != gobreaker.StateOpen {
		t.Errorf("Expected circuit breaker to be OPEN after failures")
	}

	time.Sleep(1200 * time.Millisecond)

	resp, err := client.GetUserByEmail(ctx, "test@example.com")
	if err != nil {
		t.Errorf("Expected successful call after recovery, got error: %v", err)
	}

	if resp == nil {
		t.Errorf("Expected response, got nil")
	}

	if client.cb.State() != gobreaker.StateClosed {
		t.Errorf("Expected circuit breaker to be CLOSED after recovery, got %v", client.cb.State())
	}
}
