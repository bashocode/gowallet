package rabbitresilience

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRetryCountUsesDeathCountForRetryQueue(t *testing.T) {
	headers := amqp.Table{"x-death": []interface{}{
		amqp.Table{"queue": "notification.payment_settled.retry", "count": int64(3)},
		amqp.Table{"queue": "notification.payment_settled", "count": int64(8)},
	}}
	if got := RetryCount(headers, "notification.payment_settled.retry"); got != 3 {
		t.Fatalf("RetryCount() = %d, want 3", got)
	}
}
