package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bashocode/gowallet/monolith/internal/wallet/model"
)

type WalletRepository interface {
	CreateTx(ctx context.Context, tx *sql.Tx, w *model.Wallet) error
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
}

type mysqlWalletRepository struct {
	db *sql.DB
}

func NewMySQLWalletRepository(db *sql.DB) WalletRepository {
	return &mysqlWalletRepository{db: db}
}

func (r *mysqlWalletRepository) CreateTx(ctx context.Context, tx *sql.Tx, w *model.Wallet) error {
	query := "INSERT INTO wallets (id, user_id, balance, currency, status) VALUES (?, ?, ?, ?, ?)"
	_, err := tx.ExecContext(ctx, query, w.ID, w.UserID, w.Balance, w.Currency, w.Status)
	return err
}

func (r *mysqlWalletRepository) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	query := `SELECT id, user_id, balance, currency, status, version, created_at, updated_at FROM wallets WHERE user_id = ?`
	w := &model.Wallet{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&w.ID,
		&w.UserID,
		&w.Balance,
		&w.Currency,
		&w.Status,
		&w.Version,
		&w.CreatedAt,
		&w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Wallet not found")
		}
		return nil, err
	}
	return w, nil
}
