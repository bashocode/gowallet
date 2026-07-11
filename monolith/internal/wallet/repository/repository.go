package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bashocode/gowallet/monolith/internal/wallet/model"
	"github.com/shopspring/decimal"
)

type WalletRepository interface {
	CreateTx(ctx context.Context, tx *sql.Tx, w *model.Wallet) error
	GetByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, amount decimal.Decimal, currentVersion int) error
	GetByEmail(ctx context.Context, email string) (*model.WalletInquiry, error)
	GetWalletByEmail(ctx context.Context, email string) (*model.Wallet, error)
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Wallet not found")
		}
		return nil, err
	}
	return w, nil
}

func (r *mysqlWalletRepository) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, amount decimal.Decimal, currentVersion int) error {
	query := `UPDATE wallets
              SET balance = balance - ?, version = version + 1
              WHERE id = ? AND version = ? AND balance >= ?`
	result, err := tx.ExecContext(ctx, query, amount, walletID, currentVersion, amount)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// if 0 rows affected, it means concurrent update detected or insufficient balance
	if rowsAffected == 0 {
		return errors.New("concurrent update detected or insufficient balance")
	}

	return nil
}

func (r *mysqlWalletRepository) GetByEmail(ctx context.Context, email string) (*model.WalletInquiry, error) {
	query := `
		SELECT w.id, u.full_name, u.email
		FROM wallets w
		JOIN users u ON u.id = w.user_id
		WHERE LOWER(u.email) = LOWER(?)
		  AND u.deleted_at IS NULL
		  AND w.deleted_at IS NULL
		  AND w.status = 'active'
		LIMIT 1
	`
	inquiry := &model.WalletInquiry{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&inquiry.AccountID,
		&inquiry.Name,
		&inquiry.Email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Account not found")
		}
		return nil, err
	}

	inquiry.Valid = true
	return inquiry, nil
}

func (r *mysqlWalletRepository) GetWalletByEmail(ctx context.Context, email string) (*model.Wallet, error) {
	query := `
		SELECT w.id, w.user_id, w.balance, w.currency, w.status, w.version, w.created_at, w.updated_at
		FROM wallets w
		JOIN users u ON u.id = w.user_id
		WHERE LOWER(u.email) = LOWER(?)
		  AND u.deleted_at IS NULL
		  AND w.deleted_at IS NULL
		  AND w.status = 'active'
		LIMIT 1
	`
	w := &model.Wallet{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
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
