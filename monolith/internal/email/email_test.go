package email

import (
	"context"
	"testing"

	"github.com/bashocode/gowallet/monolith/internal/logger"
)

func TestNewSMTPEmailSender(t *testing.T) {
	sender := NewSMTPEmailSender("localhost", "1025", "no-reply@gowallet.com")
	if sender == nil {
		t.Fatal("expected non-nil EmailSender")
	}
}

func TestSendEmail(t *testing.T) {
	logger.InitLogger()

	sender := NewSMTPEmailSender("localhost", "1025", "no-reply@gowallet.com")
	err := sender.SendEmail(context.Background(), "test@example.com", "Test Subject", "Test Body")
	if err != nil {
		t.Skipf("Skipping SMTP test: SMTP server not reachable: %v", err)
		return
	}
}

func TestMockEmailSender(t *testing.T) {
	m := &MockEmailSender{}
	ctx := context.Background()
	m.On("SendEmail", ctx, "test@example.com", "Subject", "Body").Return(nil)

	err := m.SendEmail(ctx, "test@example.com", "Subject", "Body")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	m.AssertExpectations(t)
}
