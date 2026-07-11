package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transfer/model"
)

type OutboundTransferRepository interface {
	Create(ctx context.Context, t *model.OutboundTransfer) error
	GetByID(ctx context.Context, id string) (*model.OutboundTransfer, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*model.OutboundTransfer, error)
	GetByIdempotencyKeyTx(ctx context.Context, tx *sql.Tx, key string) (*model.OutboundTransfer, error)
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, id string, status string) error
}

type mysqlOutboundTransferRepository struct {
	db *sql.DB
}

func NewMySQLOutboundTransferRepository(db *sql.DB) OutboundTransferRepository {
	return &mysqlOutboundTransferRepository{db: db}
}

func (r *mysqlOutboundTransferRepository) Create(ctx context.Context, t *model.OutboundTransfer) error {
	query := `INSERT INTO outbound_transfers (id, sender_user_id, receiver_email, amount, currency, external_ewallet, status, idempotency_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, t.ID, t.SenderUserID, t.ReceiverEmail, t.Amount, t.Currency, t.ExternalEwallet, t.Status, t.IdempotencyKey)
	return err
}

func (r *mysqlOutboundTransferRepository) GetByID(ctx context.Context, id string) (*model.OutboundTransfer, error) {
	query := `SELECT id, sender_user_id, receiver_email, amount, currency, external_ewallet, status, idempotency_key, created_at, updated_at FROM outbound_transfers WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var t model.OutboundTransfer
	err := row.Scan(&t.ID, &t.SenderUserID, &t.ReceiverEmail, &t.Amount, &t.Currency, &t.ExternalEwallet, &t.Status, &t.IdempotencyKey, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *mysqlOutboundTransferRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.OutboundTransfer, error) {
	return r.getByIdempotencyKey(ctx, r.db, key)
}

func (r *mysqlOutboundTransferRepository) GetByIdempotencyKeyTx(ctx context.Context, tx *sql.Tx, key string) (*model.OutboundTransfer, error) {
	return r.getByIdempotencyKey(ctx, tx, key)
}

func (r *mysqlOutboundTransferRepository) getByIdempotencyKey(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, key string) (*model.OutboundTransfer, error) {
	query := `SELECT id, sender_user_id, receiver_email, amount, currency, external_ewallet, status, idempotency_key, created_at, updated_at FROM outbound_transfers WHERE idempotency_key = ?`
	row := q.QueryRowContext(ctx, query, key)

	var t model.OutboundTransfer
	err := row.Scan(&t.ID, &t.SenderUserID, &t.ReceiverEmail, &t.Amount, &t.Currency, &t.ExternalEwallet, &t.Status, &t.IdempotencyKey, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *mysqlOutboundTransferRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, id string, status string) error {
	query := `UPDATE outbound_transfers SET status = ? WHERE id = ?`
	_, err := tx.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update outbound transfer status: %w", err)
	}
	return nil
}
