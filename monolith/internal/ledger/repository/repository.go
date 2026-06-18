package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/monolith/internal/ledger/model"
)

type LedgerRepository interface {
	CreateTx(ctx context.Context, tx *sql.Tx, entry *model.LedgerEntry) error
	GetBalanceByWalletID(ctx context.Context, walletID string) (float64, error)
	GetEntriesByWalletID(ctx context.Context, walletID string) ([]model.LedgerEntry, error)
}

type mysqlLedgerRepository struct {
	db *sql.DB
}

func NewMysqlLedgerRepository(db *sql.DB) LedgerRepository {
	return &mysqlLedgerRepository{db: db}
}

func (r *mysqlLedgerRepository) CreateTx(ctx context.Context, tx *sql.Tx, entry *model.LedgerEntry) error {
	query := `INSERT INTO ledger_entries (id, wallet_id, transaction_id, entry_type, amount) VALUES (?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query, entry.ID, entry.WalletID, entry.TransactionID, entry.EntryType, entry.Amount)
	return err
}

func (r *mysqlLedgerRepository) GetBalanceByWalletID(ctx context.Context, walletID string) (float64, error) {
	// balance = sum(credit) - sum(debit)
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount ELSE 0 END), 0)
		FROM ledger_entries
		WHERE wallet_id = ?`

	var balance float64
	err := r.db.QueryRowContext(ctx, query, walletID).Scan(&balance)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func (r *mysqlLedgerRepository) GetEntriesByWalletID(ctx context.Context, walletID string) ([]model.LedgerEntry, error) {
	query := `SELECT id, wallet_id, transaction_id, entry_type, amount, created_at FROM ledger_entries WHERE wallet_id = ? ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.LedgerEntry
	for rows.Next() {
		var e model.LedgerEntry
		if err := rows.Scan(&e.ID, &e.WalletID, &e.TransactionID, &e.EntryType, &e.Amount, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
