package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/model"
	"github.com/shopspring/decimal"
)

type LedgerRepository interface {
	Create(ctx context.Context, entry *model.LedgerEntry) error
	CreateBatch(ctx context.Context, entries []*model.LedgerEntry) error
	GetBalanceByWalletID(ctx context.Context, walletID string) (decimal.Decimal, error)
	GetEntriesByWalletID(ctx context.Context, walletID string) ([]model.LedgerEntry, error)
	GetEntriesByWalletIDPaginated(ctx context.Context, walletID string, limit int, cursor *string) ([]model.LedgerEntry, *string, error)
	GetEntriesByWalletIDWithType(ctx context.Context, walletID string, entryType string, limit int, cursor *string) ([]model.LedgerEntry, *string, error)
}

type mysqlLedgerRepository struct {
	db *sql.DB
}

func NewMySQLLedgerRepository(db *sql.DB) LedgerRepository {
	return &mysqlLedgerRepository{db: db}
}

func (r *mysqlLedgerRepository) Create(ctx context.Context, entry *model.LedgerEntry) error {
	query := `INSERT INTO ledger_entries (id, wallet_id, transaction_id, entry_type, amount) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, entry.ID, entry.WalletID, entry.TransactionID, entry.EntryType, entry.Amount)
	return err
}

func (r *mysqlLedgerRepository) CreateBatch(ctx context.Context, entries []*model.LedgerEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO ledger_entries (id, wallet_id, transaction_id, entry_type, amount) VALUES (?, ?, ?, ?, ?)`
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx, query, e.ID, e.WalletID, e.TransactionID, e.EntryType, e.Amount); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *mysqlLedgerRepository) GetBalanceByWalletID(ctx context.Context, walletID string) (decimal.Decimal, error) {
	// balance = sum(credit) - sum(debit)
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount ELSE 0 END), 0)
		FROM ledger_entries
		WHERE wallet_id = ?`

	var balance decimal.Decimal
	err := r.db.QueryRowContext(ctx, query, walletID).Scan(&balance)
	if err != nil {
		return decimal.Zero, err
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

// GetEntriesByWalletIDPaginated retrieves ledger entries with cursor-based pagination
// Uses created_at as cursor to avoid slow OFFSET queries on large datasets
// Returns entries and next cursor (ID of last entry, or nil if no more pages)
func (r *mysqlLedgerRepository) GetEntriesByWalletIDPaginated(ctx context.Context, walletID string, limit int, cursor *string) ([]model.LedgerEntry, *string, error) {
	var query string
	var rows *sql.Rows
	var err error

	if cursor == nil || *cursor == "" {
		// First page: no cursor, just use LIMIT
		query = `SELECT id, wallet_id, transaction_id, entry_type, amount, created_at
				FROM ledger_entries
				WHERE wallet_id = ?
				ORDER BY created_at DESC, id DESC
				LIMIT ?`
		rows, err = r.db.QueryContext(ctx, query, walletID, limit+1)
	} else {
		// Subsequent pages: use cursor (created_at comparison)
		// Cursor format: "created_at:id" for stable pagination
		query = `SELECT id, wallet_id, transaction_id, entry_type, amount, created_at
				FROM ledger_entries
				WHERE wallet_id = ? AND (created_at < ? OR (created_at = ? AND id < ?))
				ORDER BY created_at DESC, id DESC
				LIMIT ?`
		rows, err = r.db.QueryContext(ctx, query, walletID, *cursor, *cursor, *cursor, limit+1)
	}

	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var entries []model.LedgerEntry
	for rows.Next() {
		var e model.LedgerEntry
		if err := rows.Scan(&e.ID, &e.WalletID, &e.TransactionID, &e.EntryType, &e.Amount, &e.CreatedAt); err != nil {
			return nil, nil, err
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Check if there are more pages
	var nextCursor *string
	if len(entries) > limit {
		// There are more entries, return cursor for next page
		lastEntry := entries[limit-1]
		cursorValue := lastEntry.CreatedAt.Format("2006-01-02 15:04:05.999999")
		nextCursor = &cursorValue
		entries = entries[:limit] // Return only requested limit
	}

	return entries, nextCursor, nil
}

// GetEntriesByWalletIDWithType retrieves ledger entries filtered by entry type with cursor pagination
// Useful for queries like "show only credit entries" or "show only debit entries"
func (r *mysqlLedgerRepository) GetEntriesByWalletIDWithType(ctx context.Context, walletID string, entryType string, limit int, cursor *string) ([]model.LedgerEntry, *string, error) {
	var query string
	var rows *sql.Rows
	var err error

	if cursor == nil || *cursor == "" {
		// First page
		query = `SELECT id, wallet_id, transaction_id, entry_type, amount, created_at
				FROM ledger_entries
				WHERE wallet_id = ? AND entry_type = ?
				ORDER BY created_at DESC, id DESC
				LIMIT ?`
		rows, err = r.db.QueryContext(ctx, query, walletID, entryType, limit+1)
	} else {
		// Subsequent pages
		query = `SELECT id, wallet_id, transaction_id, entry_type, amount, created_at
				FROM ledger_entries
				WHERE wallet_id = ? AND entry_type = ? AND (created_at < ? OR (created_at = ? AND id < ?))
				ORDER BY created_at DESC, id DESC
				LIMIT ?`
		rows, err = r.db.QueryContext(ctx, query, walletID, entryType, *cursor, *cursor, *cursor, limit+1)
	}

	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var entries []model.LedgerEntry
	for rows.Next() {
		var e model.LedgerEntry
		if err := rows.Scan(&e.ID, &e.WalletID, &e.TransactionID, &e.EntryType, &e.Amount, &e.CreatedAt); err != nil {
			return nil, nil, err
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var nextCursor *string
	if len(entries) > limit {
		lastEntry := entries[limit-1]
		cursorValue := lastEntry.CreatedAt.Format("2006-01-02 15:04:05.999999")
		nextCursor = &cursorValue
		entries = entries[:limit]
	}

	return entries, nextCursor, nil
}
