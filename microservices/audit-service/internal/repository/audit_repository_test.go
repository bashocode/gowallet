package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bashocode/gowallet/microservices/audit-service/internal/model"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestAuditRepository_SaveAuditLog(t *testing.T) {
	logger.InitLogger()

	mongoURL := os.Getenv("MONGO_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://localhost:27017"
	}

	client, err := database.ConnectMongoDB(mongoURL)
	if err != nil {
		t.Skipf("Skipping audit repository test: mongodb not reachable: %v", err)
		return
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("failed to disconnect from MongoDB: %v", err)
		}
	}()

	db := client.Database("gowallet_audit_test")
	defer func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Errorf("failed to drop test database: %v", err)
		}
	}()

	repo := NewAuditRepository(db)

	t.Run("save new audit log successfully", func(t *testing.T) {
		ctx := context.Background()
		auditLog := model.AuditLog{
			ID:          "test-event-123",
			EventType:   "payment.settled",
			MessageID:   "msg-123",
			Source:      "payment-service",
			Payload:     map[string]any{"amount": "100000", "currency": "IDR"},
			ReceivedAt:  time.Now().UTC(),
			ProcessedAt: time.Now().UTC(),
		}

		err := repo.SaveAuditLog(ctx, auditLog)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("save duplicate audit log is idempotent", func(t *testing.T) {
		ctx := context.Background()
		auditLog := model.AuditLog{
			ID:          "test-event-duplicate",
			EventType:   "payment.settled",
			MessageID:   "msg-dup",
			Source:      "payment-service",
			Payload:     map[string]any{"amount": "50000"},
			ReceivedAt:  time.Now().UTC(),
			ProcessedAt: time.Now().UTC(),
		}

		err := repo.SaveAuditLog(ctx, auditLog)
		if err != nil {
			t.Errorf("first save: expected no error, got: %v", err)
		}

		err = repo.SaveAuditLog(ctx, auditLog)
		if err != nil {
			t.Errorf("duplicate save: expected no error (idempotent), got: %v", err)
		}
	})
}
