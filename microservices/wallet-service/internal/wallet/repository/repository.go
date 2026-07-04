package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/shopspring/decimal"
)

type WalletRepository interface {
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error)
	Create(ctx context.Context, w *model.Wallet) error
}

type mysqlWalletRepository struct {
	db *sql.DB
}

func NewMySQLWalletRepository(db *sql.DB) WalletRepository {
	return &mysqlWalletRepository{db: db}
}

func (r *mysqlWalletRepository) Create(ctx context.Context, w *model.Wallet) error {
	query := "INSERT INTO wallets (id, user_id, balance, currency, status, version) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, w.ID, w.UserID, w.Balance, w.Currency, w.Status, w.Version)
	return err
}

func (r *mysqlWalletRepository) GetByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	query := `SELECT id, user_id, balance, currency, status, version, created_at, updated_at FROM wallets WHERE user_id = ? AND deleted_at IS NULL`
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
		return nil, err
	}
	return w, nil
}

func (r *mysqlWalletRepository) UpdateBalanceWithOwnerCheck(ctx context.Context, userID string, amount decimal.Decimal, expectedVersion int32) (*model.Wallet, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `UPDATE wallets SET balance = balance + ?, version = version + 1 WHERE user_id = ? AND version = ? AND balance + ? >= 0`
	result, err := tx.ExecContext(ctx, query, amount, userID, expectedVersion, amount)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, errors.New("concurrent update detected or insufficient balance")
	}

	queryGet := `SELECT id, user_id, balance, currency, status, version, created_at, updated_at FROM wallets WHERE user_id = ?`
	w := &model.Wallet{}
	err = tx.QueryRowContext(ctx, queryGet, userID).Scan(
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
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return w, nil
}
