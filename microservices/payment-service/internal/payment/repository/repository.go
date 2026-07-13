package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/model"
)

type PaymentRepository interface {
	Create(ctx context.Context, p *model.Payment) error
	GetByStripeSessionID(ctx context.Context, sessionID string) (*model.Payment, error)
	GetByStripeSessionIDTx(ctx context.Context, tx *sql.Tx, sessionID string) (*model.Payment, error)
	UpdateStatus(ctx context.Context, sessionID string, status string) error
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, sessionID string, status string) error
}

type mysqlPaymentRepository struct {
	db *sql.DB
}

func NewMySQLPaymentRepository(db *sql.DB) PaymentRepository {
	return &mysqlPaymentRepository{db: db}
}

func (r *mysqlPaymentRepository) Create(ctx context.Context, p *model.Payment) error {
	query := `INSERT INTO payments (id, user_id, amount, currency, stripe_session_id, status) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, p.ID, p.UserID, p.Amount, p.Currency, p.StripeSessionID, p.Status)
	return err
}

func (r *mysqlPaymentRepository) GetByStripeSessionID(ctx context.Context, sessionID string) (*model.Payment, error) {
	query := `SELECT id, user_id, amount, currency, stripe_session_id, status, created_at, updated_at FROM payments WHERE stripe_session_id = ?`
	row := r.db.QueryRowContext(ctx, query, sessionID)

	var p model.Payment
	err := row.Scan(&p.ID, &p.UserID, &p.Amount, &p.Currency, &p.StripeSessionID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *mysqlPaymentRepository) GetByStripeSessionIDTx(ctx context.Context, tx *sql.Tx, sessionID string) (*model.Payment, error) {
	query := `SELECT id, user_id, amount, currency, stripe_session_id, status, created_at, updated_at FROM payments WHERE stripe_session_id = ?`
	row := tx.QueryRowContext(ctx, query, sessionID)

	var p model.Payment
	err := row.Scan(&p.ID, &p.UserID, &p.Amount, &p.Currency, &p.StripeSessionID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *mysqlPaymentRepository) UpdateStatus(ctx context.Context, sessionID string, status string) error {
	query := `UPDATE payments SET status = ? WHERE stripe_session_id = ?`
	_, err := r.db.ExecContext(ctx, query, status, sessionID)
	return err
}

func (r *mysqlPaymentRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, sessionID string, status string) error {
	query := `UPDATE payments SET status = ? WHERE stripe_session_id = ?`
	_, err := tx.ExecContext(ctx, query, status, sessionID)
	return err
}
