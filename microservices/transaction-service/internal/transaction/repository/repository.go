package repository

import (
	"context"
	"database/sql"

	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
)

type TransactionRepository interface {
	Create(ctx context.Context, t *model.Transaction) error
	GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error)
	GetHistory(ctx context.Context, walletID string, params model.PaginationParams) ([]model.Transaction, int64, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

type mysqlTransactionRepository struct {
	db *sql.DB
}

func NewMySQLTransactionRepository(db *sql.DB) TransactionRepository {
	return &mysqlTransactionRepository{db: db}
}

func (r *mysqlTransactionRepository) Create(ctx context.Context, t *model.Transaction) error {
	query := `INSERT INTO transactions (id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, t.ID, t.SenderWalletID, t.ReceiverWalletID, t.Amount, t.Description, t.IdempotencyKey, t.Status)
	return err
}

func (r *mysqlTransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error) {
	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at FROM transactions WHERE idempotency_key = ?`
	t := &model.Transaction{}
	var sender sql.NullString
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&t.ID,
		&sender,
		&t.ReceiverWalletID,
		&t.Amount,
		&t.Description,
		&t.IdempotencyKey,
		&t.Status,
		&t.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if sender.Valid {
		t.SenderWalletID = &sender.String
	}

	return t, nil
}

func (r *mysqlTransactionRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE transactions SET status = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

func (r *mysqlTransactionRepository) GetHistory(ctx context.Context, walletID string, params model.PaginationParams) ([]model.Transaction, int64, error) {
	// counting total data for pagination meta
	countQuery := `SELECT COUNT(*) FROM transactions WHERE (sender_wallet_id = ? OR receiver_wallet_id = ?)`
	var total int64
	var err error

	if params.Status != "" {
		countQuery += " AND status = ?"
		err = r.db.QueryRowContext(ctx, countQuery, walletID, walletID, params.Status).Scan(&total)
	} else {
		err = r.db.QueryRowContext(ctx, countQuery, walletID, walletID).Scan(&total)
	}

	if err != nil {
		return nil, 0, err
	}

	// get the paginated data, use sort and order
	// important, use whitelist for sort and order to prevent sql injection
	sortColumn := "created_at"
	if params.Sort == "amount" {
		sortColumn = "amount"
	}

	sortOrder := "DESC"
	if params.Order == "asc" {
		sortOrder = "ASC"
	}

	query := `SELECT id, sender_wallet_id, receiver_wallet_id,
				amount, description, idempotency_key, status, created_at
			FROM transactions WHERE (sender_wallet_id = ? OR
			receiver_wallet_id = ?)`

	var rows *sql.Rows
	if params.Status != "" {
		query += " AND status = ? ORDER BY " + sortColumn + " " + sortOrder + " LIMIT ? OFFSET ?"
		rows, err = r.db.QueryContext(ctx, query, walletID, walletID, params.Status, params.Limit, params.Offset())
	} else {
		query += " ORDER BY " + sortColumn + " " + sortOrder + " LIMIT ? OFFSET ?"
		rows, err = r.db.QueryContext(ctx, query, walletID, walletID, params.Limit, params.Offset())
	}

	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	var txs []model.Transaction
	for rows.Next() {
		var t model.Transaction
		var sender sql.NullString
		err := rows.Scan(
			&t.ID,
			&sender,
			&t.ReceiverWalletID,
			&t.Amount,
			&t.Description,
			&t.IdempotencyKey,
			&t.Status,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if sender.Valid {
			t.SenderWalletID = &sender.String
		}
		txs = append(txs, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return txs, total, nil
}
