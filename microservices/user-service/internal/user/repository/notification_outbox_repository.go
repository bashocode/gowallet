package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/bashocode/gowallet/microservices/user-service/internal/user/model"
)

type NotificationOutboxRepository interface {
	CreateTx(ctx context.Context, tx *sql.Tx, event *model.NotificationOutboxEvent) error
	GetPendingEvents(ctx context.Context, limit int) ([]*model.NotificationOutboxEvent, error)
	MarkAsProcessed(ctx context.Context, id string) error
	IncrementAttempts(ctx context.Context, id string, lastError string) error
	FetchEventsToArchive(ctx context.Context, minAge time.Duration, limit int) ([]*model.NotificationOutboxEvent, error)
	DeleteArchivedEvents(ctx context.Context, ids []string) error
}

type mysqlNotificationOutboxRepository struct {
	db *sql.DB
}

func NewMySQLNotificationOutboxRepository(db *sql.DB) NotificationOutboxRepository {
	return &mysqlNotificationOutboxRepository{db: db}
}

func (r *mysqlNotificationOutboxRepository) CreateTx(ctx context.Context, tx *sql.Tx, event *model.NotificationOutboxEvent) error {
	query := `INSERT INTO notification_outbox_events (id, event_type, aggregate_id, payload, status, attempts)
	          VALUES (?, ?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query, event.ID, event.EventType, event.AggregateID, event.Payload, event.Status, event.Attempts)
	return err
}

func (r *mysqlNotificationOutboxRepository) GetPendingEvents(ctx context.Context, limit int) ([]*model.NotificationOutboxEvent, error) {
	query := `SELECT id, event_type, aggregate_id, payload, status, attempts, last_error, created_at, updated_at
	          FROM notification_outbox_events
	          WHERE status = 'pending'
	          ORDER BY created_at ASC
	          LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.NotificationOutboxEvent
	for rows.Next() {
		var event model.NotificationOutboxEvent
		err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.AggregateID,
			&event.Payload,
			&event.Status,
			&event.Attempts,
			&event.LastError,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, &event)
	}

	return events, rows.Err()
}

func (r *mysqlNotificationOutboxRepository) MarkAsProcessed(ctx context.Context, id string) error {
	query := `UPDATE notification_outbox_events SET status = 'processed' WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *mysqlNotificationOutboxRepository) IncrementAttempts(ctx context.Context, id string, lastError string) error {
	query := `UPDATE notification_outbox_events SET attempts = attempts + 1, last_error = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, lastError, id)
	return err
}

func (r *mysqlNotificationOutboxRepository) FetchEventsToArchive(
	ctx context.Context,
	minAge time.Duration,
	limit int,
) ([]*model.NotificationOutboxEvent, error) {
	query := `
		SELECT id, event_type, aggregate_id, payload, status, attempts, last_error, created_at, updated_at 
		FROM notification_outbox_events 
		WHERE status = 'processed' 
		  AND created_at < NOW() - INTERVAL ? SECOND
		LIMIT ?
	`
	seconds := int(minAge.Seconds())
	rows, err := r.db.QueryContext(ctx, query, seconds, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.NotificationOutboxEvent
	for rows.Next() {
		var event model.NotificationOutboxEvent
		err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.AggregateID,
			&event.Payload,
			&event.Status,
			&event.Attempts,
			&event.LastError,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *mysqlNotificationOutboxRepository) DeleteArchivedEvents(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf("DELETE FROM notification_outbox_events WHERE id IN (%s)", placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}
