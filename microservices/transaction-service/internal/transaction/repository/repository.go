package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
)

type TransactionRepository interface {
	Create(ctx context.Context, t *model.Transaction) error
	CreateTx(ctx context.Context, tx *sql.Tx, t *model.Transaction) error
	GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error)
	GetHistory(ctx context.Context, walletID string, params model.PaginationParams) ([]model.Transaction, int64, error)
	GetHistoryCursor(ctx context.Context, walletID string, limit int, cursor *string, status string) ([]model.Transaction, *string, error)
	GetHistoryByUserIDCursor(ctx context.Context, userID string, limit int, cursor *string, status string) ([]model.Transaction, *string, error)
	GetByIDs(ctx context.Context, ids []string) ([]model.Transaction, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, id, status string) error
	CountToday(ctx context.Context) (int64, error)
	CreateOutboxTx(ctx context.Context, tx *sql.Tx, event *model.OutboxEvent) error
	FetchEventsToArchive(ctx context.Context, minAge time.Duration, limit int) ([]model.OutboxEvent, error)
	DeleteArchivedEvents(ctx context.Context, ids []string) error
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

func (r *mysqlTransactionRepository) CreateTx(ctx context.Context, tx *sql.Tx, t *model.Transaction) error {
	query := `INSERT INTO transactions (id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query, t.ID, t.SenderWalletID, t.ReceiverWalletID, t.Amount, t.Description, t.IdempotencyKey, t.Status)
	return err
}

func (r *mysqlTransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error) {
	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, description, idempotency_key, status, created_at FROM transactions WHERE idempotency_key = ?`
	t := &model.Transaction{}
	var sender sql.NullString
	var receiver sql.NullString
	var desc sql.NullString
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&t.ID,
		&sender,
		&receiver,
		&t.Amount,
		&desc,
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
	if receiver.Valid {
		t.ReceiverWalletID = receiver.String
	}
	if desc.Valid {
		t.Description = desc.String
	}

	return t, nil
}

func (r *mysqlTransactionRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE transactions SET status = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

func (r *mysqlTransactionRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, id, status string) error {
	query := `UPDATE transactions SET status = ? WHERE id = ?`
	_, err := tx.ExecContext(ctx, query, status, id)
	return err
}

func (r *mysqlTransactionRepository) CreateOutboxTx(ctx context.Context, tx *sql.Tx, event *model.OutboxEvent) error {
	query := `INSERT INTO outbox_events (id, event_type, payload, status) VALUES (?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query, event.ID, event.EventType, event.Payload, event.Status)
	return err
}

// CountToday returns the number of transactions created since UTC midnight today.
func (r *mysqlTransactionRepository) CountToday(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM transactions WHERE created_at >= DATE(UTC_TIMESTAMP())`
	var count int64
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
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
		var receiver sql.NullString
		var desc sql.NullString
		err := rows.Scan(
			&t.ID,
			&sender,
			&receiver,
			&t.Amount,
			&desc,
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
		if receiver.Valid {
			t.ReceiverWalletID = receiver.String
		}
		if desc.Valid {
			t.Description = desc.String
		}
		txs = append(txs, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return txs, total, nil
}

func (r *mysqlTransactionRepository) FetchEventsToArchive(
	ctx context.Context,
	minAge time.Duration,
	limit int,
) ([]model.OutboxEvent, error) {
	query := `
		SELECT id, event_type, payload, status, created_at 
		FROM outbox_events 
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

	var events []model.OutboxEvent
	for rows.Next() {
		var ev model.OutboxEvent
		if err := rows.Scan(&ev.ID, &ev.EventType, &ev.Payload, &ev.Status, &ev.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *mysqlTransactionRepository) DeleteArchivedEvents(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf("DELETE FROM outbox_events WHERE id IN (%s)", placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// GetHistoryCursor retrieves transaction history using cursor-based pagination (by wallet)
// This avoids slow OFFSET queries on large datasets. Cursor is based on created_at timestamp.
// Returns transactions and next cursor (timestamp of last entry, or nil if no more pages)
func (r *mysqlTransactionRepository) GetHistoryCursor(ctx context.Context, walletID string, limit int, cursor *string, status string) ([]model.Transaction, *string, error) {
	var query string
	var rows *sql.Rows
	var err error
	var args []interface{}

	// Base query - select only needed columns for list view (avoid SELECT *)
	baseSelect := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, status, created_at FROM transactions`
	baseWhere := ` WHERE (sender_wallet_id = ? OR receiver_wallet_id = ?)`

	if cursor == nil || *cursor == "" {
		// First page
		query = baseSelect + baseWhere
		args = append(args, walletID, walletID)

		if status != "" {
			query += ` AND status = ?`
			args = append(args, status)
		}

		query += ` ORDER BY created_at DESC LIMIT ?`
		args = append(args, limit+1) // Fetch one extra to check if there are more pages
	} else {
		// Subsequent pages - use cursor
		query = baseSelect + baseWhere + ` AND created_at < ?`
		args = append(args, walletID, walletID, *cursor)

		if status != "" {
			query += ` AND status = ?`
			args = append(args, status)
		}

		query += ` ORDER BY created_at DESC LIMIT ?`
		args = append(args, limit+1)
	}

	rows, err = r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var txs []model.Transaction
	for rows.Next() {
		var t model.Transaction
		var sender sql.NullString
		var receiver sql.NullString

		err := rows.Scan(&t.ID, &sender, &receiver, &t.Amount, &t.Status, &t.CreatedAt)
		if err != nil {
			return nil, nil, err
		}

		if sender.Valid {
			t.SenderWalletID = &sender.String
		}
		if receiver.Valid {
			t.ReceiverWalletID = receiver.String
		}

		txs = append(txs, t)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Check if there are more pages
	var nextCursor *string
	if len(txs) > limit {
		lastTx := txs[limit-1]
		cursorValue := lastTx.CreatedAt.Format("2006-01-02 15:04:05.999999")
		nextCursor = &cursorValue
		txs = txs[:limit] // Return only requested limit
	}

	return txs, nextCursor, nil
}

// GetHistoryByUserIDCursor retrieves transaction history by sender_user_id using cursor pagination
// Uses the composite index idx_transactions_sender_user_id_created_at for optimal performance
func (r *mysqlTransactionRepository) GetHistoryByUserIDCursor(ctx context.Context, userID string, limit int, cursor *string, status string) ([]model.Transaction, *string, error) {
	var query string
	var rows *sql.Rows
	var err error
	var args []interface{}

	baseSelect := `SELECT id, sender_user_id, sender_wallet_id, receiver_wallet_id, amount, status, created_at FROM transactions`
	baseWhere := ` WHERE sender_user_id = ?`

	if cursor == nil || *cursor == "" {
		// First page
		query = baseSelect + baseWhere
		args = append(args, userID)

		if status != "" {
			query += ` AND status = ?`
			args = append(args, status)
		}

		query += ` ORDER BY created_at DESC LIMIT ?`
		args = append(args, limit+1)
	} else {
		// Subsequent pages
		query = baseSelect + baseWhere + ` AND created_at < ?`
		args = append(args, userID, *cursor)

		if status != "" {
			query += ` AND status = ?`
			args = append(args, status)
		}

		query += ` ORDER BY created_at DESC LIMIT ?`
		args = append(args, limit+1)
	}

	rows, err = r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var txs []model.Transaction
	for rows.Next() {
		var t model.Transaction
		var senderUserID sql.NullString
		var sender sql.NullString
		var receiver sql.NullString

		err := rows.Scan(&t.ID, &senderUserID, &sender, &receiver, &t.Amount, &t.Status, &t.CreatedAt)
		if err != nil {
			return nil, nil, err
		}

		if senderUserID.Valid {
			t.SenderUserID = &senderUserID.String
		}
		if sender.Valid {
			t.SenderWalletID = &sender.String
		}
		if receiver.Valid {
			t.ReceiverWalletID = receiver.String
		}

		txs = append(txs, t)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var nextCursor *string
	if len(txs) > limit {
		lastTx := txs[limit-1]
		cursorValue := lastTx.CreatedAt.Format("2006-01-02 15:04:05.999999")
		nextCursor = &cursorValue
		txs = txs[:limit]
	}

	return txs, nextCursor, nil
}

// GetByIDs batch fetches transactions by IDs to avoid N+1 queries
// Returns a slice of transactions in the same order as the input IDs
func (r *mysqlTransactionRepository) GetByIDs(ctx context.Context, ids []string) ([]model.Transaction, error) {
	if len(ids) == 0 {
		return []model.Transaction{}, nil
	}

	// Build placeholders for IN clause
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	query := fmt.Sprintf(`SELECT id, sender_user_id, sender_wallet_id, receiver_wallet_id,
		amount, description, idempotency_key, status, created_at
		FROM transactions WHERE id IN (%s)`, placeholders)

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Use map for O(1) lookup, then reorder by input IDs
	txMap := make(map[string]model.Transaction)
	for rows.Next() {
		var t model.Transaction
		var senderUserID sql.NullString
		var sender sql.NullString
		var receiver sql.NullString
		var desc sql.NullString

		err := rows.Scan(&t.ID, &senderUserID, &sender, &receiver, &t.Amount, &desc, &t.IdempotencyKey, &t.Status, &t.CreatedAt)
		if err != nil {
			return nil, err
		}

		if senderUserID.Valid {
			t.SenderUserID = &senderUserID.String
		}
		if sender.Valid {
			t.SenderWalletID = &sender.String
		}
		if receiver.Valid {
			t.ReceiverWalletID = receiver.String
		}
		if desc.Valid {
			t.Description = desc.String
		}

		txMap[t.ID] = t
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return transactions in the same order as input IDs
	result := make([]model.Transaction, 0, len(ids))
	for _, id := range ids {
		if tx, ok := txMap[id]; ok {
			result = append(result, tx)
		}
	}

	return result, nil
}
