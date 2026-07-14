package repository

import (
	"context"

	"github.com/bashocode/gowallet/microservices/audit-service/internal/models"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuditRepository struct {
	db *mongo.Database
}

func NewAuditRepository(db *mongo.Database) *AuditRepository {
	return &AuditRepository{
		db: db,
	}
}

func (r *AuditRepository) SaveAuditLog(ctx context.Context, auditLog models.AuditLog) error {
	_, err := r.db.Collection("audit_logs").InsertOne(ctx, auditLog)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			logger.Info(ctx, "duplicate audit log, skipping", "event_id", auditLog.ID)
			return nil
		}
		logger.Error(ctx, "failed to save audit log", "error", err, "event_id", auditLog.ID)
		return err
	}
	return nil
}
