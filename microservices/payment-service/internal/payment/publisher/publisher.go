package publisher

import (
	"context"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/model"
)

type PaymentPublisher interface {
	PublishPaymentSettled(ctx context.Context, event model.PaymentSettledEvent) error
	Close() error
}
