package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
)

type ProtectedUserClient struct {
	client pb.UserServiceClient
	cb     *gobreaker.CircuitBreaker
}

func NewProtectedUserClient(grpcConn *grpc.ClientConn) *ProtectedUserClient {
	client := pb.NewUserServiceClient(grpcConn)

	cbSettings := gobreaker.Settings{
		Name:        "user-service-cb",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	}

	return &ProtectedUserClient{
		client: client,
		cb:     gobreaker.NewCircuitBreaker(cbSettings),
	}
}

func (c *ProtectedUserClient) GetUserByEmail(ctx context.Context, email string) (*pb.UserResponse, error) {
	result, err := c.cb.Execute(func() (any, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		return c.client.GetUserByEmail(timeoutCtx, &pb.GetUserByEmailRequest{Email: email})
	})

	if err != nil {
		return nil, err
	}

	resp, ok := result.(*pb.UserResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from circuit breaker: %T", result)
	}

	return resp, nil
}

func (c *ProtectedUserClient) GetUserByID(ctx context.Context, userID string) (*pb.UserResponse, error) {
	result, err := c.cb.Execute(func() (any, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		return c.client.GetUserByID(timeoutCtx, &pb.GetUserRequest{Id: userID})
	})

	if err != nil {
		return nil, err
	}

	resp, ok := result.(*pb.UserResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from circuit breaker: %T", result)
	}

	return resp, nil
}

func (c *ProtectedUserClient) CircuitBreakerState() gobreaker.State {
	return c.cb.State()
}
