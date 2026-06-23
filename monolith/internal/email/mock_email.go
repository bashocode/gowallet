package email

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockEmailSender struct {
	mock.Mock
}

func (m *MockEmailSender) SendEmail(ctx context.Context, to string, subject string, body string) error {
	args := m.Called(ctx, to, subject, body)
	return args.Error(0)
}
