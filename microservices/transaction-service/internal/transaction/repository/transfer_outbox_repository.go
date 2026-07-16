package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
)

type TransferOutboxRepository interface {
	Create(ctx context.Context, event *model.TransferOutboxEvent) error
	CreateTx(ctx context.Context, tx *sql.Tx, event *model.TransferOutboxEvent) error
	FetchPending(ctx context.Context, limit int) ([]model.TransferOutboxEvent, error)
	MarkProcessed(ctx context.Context, id string) error
	IncrementAttempts(ctx context.Context, id string, lastError string) error
}

type mysqlTransferOutboxRepository struct {
	db *sql.DB
}

func NewMySQLTransferOutboxRepository(db *sql.DB) TransferOutboxRepository {
	return &mysqlTransferOutboxRepository{db: db}
}

func (r *mysqlTransferOutboxRepository) Create(ctx context.Context, event *model.TransferOutboxEvent) error {
	query := `INSERT INTO outbox_events (id, event_type, aggregate_id, payload, status) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, event.ID, event.EventType, event.AggregateID, event.Payload, event.Status)
	return err
}

func (r *mysqlTransferOutboxRepository) CreateTx(ctx context.Context, tx *sql.Tx, event *model.TransferOutboxEvent) error {
	query := `INSERT INTO outbox_events (id, event_type, aggregate_id, payload, status) VALUES (?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query, event.ID, event.EventType, event.AggregateID, event.Payload, event.Status)
	return err
}

func (r *mysqlTransferOutboxRepository) FetchPending(ctx context.Context, limit int) ([]model.TransferOutboxEvent, error) {
	query := `SELECT id, event_type, payload FROM outbox_events WHERE status = 'pending' AND event_type LIKE 'transfer.%' ORDER BY created_at ASC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.TransferOutboxEvent
	for rows.Next() {
		var e model.TransferOutboxEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload); err != nil {
			continue
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *mysqlTransferOutboxRepository) MarkProcessed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE outbox_events SET status = 'processed' WHERE id = ?`, id)
	return err
}

func (r *mysqlTransferOutboxRepository) IncrementAttempts(ctx context.Context, id string, lastError string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE outbox_events SET attempts = attempts + 1, last_error = ? WHERE id = ?`, lastError, id)
	return err
}
