package scheduler

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bashocode/gowallet/monolith/internal/logger"
)

func (s *Scheduler) CleanupExpiredOTPs() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// delete otp that expired more than 1 hour ago
	query := `DELETE FROM otp_codes WHERE expires_at < NOW()`

	// for now we log and bypass query if the table doesnt exist
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		logger.Warn(ctx, "[Cron Job] Bypass: Table otp_codes not created yet. Skipped.", "error", err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Log.InfoContext(ctx, "[Cron Job] Cleaned expired OTPs finished successfully.", "deleted_rows", rowsAffected)
}

func (s *Scheduler) ReconcileAllBalances() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Log.InfoContext(ctx, "[Cron Job] Starting daily balance reconciliation audit...")

	// get all wallet in db
	query := `SELECT id, user_id, balance FROM wallets`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		logger.Error(ctx, "[Cron Job] Reconciliation failed to query wallets", "error", err.Error())
	}
	defer rows.Close()

	missmatchCount := 0
	for rows.Next() {
		var walletID string
		var userID string
		var currentBalance float64

		if err := rows.Scan(&walletID, &userID, &currentBalance); err != nil {
			continue
		}

		// calculate total sum from ledger entries for this wallet
		ledgerBalance, err := s.ledgerRepo.GetBalanceByWalletID(ctx, walletID)
		if err != nil {
			logger.Error(ctx, "[Cron Job] Reconciliation failed to get ledger balance", "wallet_id", walletID, "error", err.Error())
			continue
		}

		// compare balance
		if currentBalance != ledgerBalance {
			missmatchCount++
			logger.Error(
				ctx,
				"CRITICAL: BALANCE MISSMATCH DETECTED DURING AUDIT!",
				"wallet_id", walletID,
				"user_id", userID,
				"wallet_table_balance", currentBalance,
				"ledger_calculated_balance", ledgerBalance,
				"difference", currentBalance-ledgerBalance,
			)

			// in production, we can add slack/telegram alert to dev team
		}
	}

	logger.Log.WarnContext(ctx, "[Cron Job] Daily balance reconciliation finished.", "mismatch_wallets_count", missmatchCount)
}

func (s *Scheduler) CleanupExpiredRefreshTokens() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger.Log.InfoContext(ctx, "[Cron Job] Starting expired refresh token cleanup...")

	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW()`

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		logger.Warn(ctx, "[Cron Job] Bypass: Table refresh_tokens not created yet. Skipped.", "error", err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Log.InfoContext(ctx, "[Cron Job] Cleaned expired refresh tokens finished successfully.", "deleted_rows", rowsAffected)
}

func (s *Scheduler) ExportDailyTransactions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Log.InfoContext(ctx, "[Cron Job] Starting daily transaction report generation...")

	// query transaction today
	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, status,
			created_at FROM transactions WHERE DATE(created_at) = CURDATE()`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		logger.Error(ctx, "[Cron Job] Daily transaction report generation failed to query transactions", "error", err.Error())
		return
	}
	defer rows.Close()

	// create reports directory folder
	reportsDir := "./reports"
	_ = os.MkdirAll(reportsDir, os.ModePerm)

	// create csv file
	filename := filepath.Join(
		reportsDir,
		fmt.Sprintf("transaction_report_%s.csv", time.Now().Format("20060102")),
	)
	file, err := os.Create(filename)
	if err != nil {
		logger.Error(ctx, "Failed to create report file", "error", err.Error())
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// write csv column headers
	_ = writer.Write([]string{
		"Transaction ID",
		"Sender Wallet ID",
		"Receiver Wallet ID",
		"Amount",
		"Status",
		"Created At",
	})

	rowCount := 0
	for rows.Next() {
		var id, sender, receiver, status, createdAt string
		var amount float64

		_ = rows.Scan(&id, &sender, &receiver, &amount, &status, &createdAt)
		_ = writer.Write([]string{id, sender, receiver, fmt.Sprintf("%.2f", amount), status, createdAt})

		rowCount++
	}

	logger.Log.InfoContext(ctx, "[Cron Job] Daily transaction report generation finished successfully", "rows_written", rowCount, "file", filename)
}
