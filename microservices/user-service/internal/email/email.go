package email

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/bashocode/gowallet/microservices/shared/logger"
)

type EmailSender interface {
	SendEmail(ctx context.Context, to string, subject string, body string) error
}

type smtpEmailSender struct {
	host string
	port string
	from string
}

func NewSMTPEmailSender(host string, port string, from string) EmailSender {
	return &smtpEmailSender{
		host: host,
		port: port,
		from: from,
	}
}

func (s *smtpEmailSender) SendEmail(ctx context.Context, to string, subject string, body string) error {
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body))

	addr := s.host + ":" + s.port
	err := smtp.SendMail(addr, nil, s.from, []string{to}, msg)
	if err != nil {
		logger.Log.Error("Failed to send email via SMTP", "to", to, "error", err)
		return err
	}

	logger.Log.Info("Email sent successfully", "to", to)
	return nil
}
