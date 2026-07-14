package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/microservices/shared/logger"
)

type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

func (r *NotificationRepository) HasProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM processed_notifications WHERE event_id = ?)"
	err := r.db.QueryRowContext(ctx, query, eventID).Scan(&exists)
	if err != nil {
		logger.Error(ctx, "failed to check if event was processed", "error", err, "event_id", eventID)
		return false, err
	}
	return exists, nil
}

func (r *NotificationRepository) MarkProcessed(ctx context.Context, eventID string) error {
	query := "INSERT INTO processed_notifications (event_id) VALUES (?)"
	_, err := r.db.ExecContext(ctx, query, eventID)
	if err != nil {
		logger.Error(ctx, "failed to mark event as processed", "error", err, "event_id", eventID)
		return err
	}
	return nil
}
