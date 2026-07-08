package scheduler

import (
	"context"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	authPb "github.com/bashocode/gowallet/microservices/auth-service/proto/auth"
	txPb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	walletPb "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
)

func (s *Scheduler) TriggerOTPCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Log.InfoContext(ctx, "[Cron Job] Triggering expired OTP cleanup via Auth gRPC...")
	_, err := s.authClient.CleanupExpiredOTPs(ctx, &authPb.CleanupRequest{})
	if err != nil {
		logger.Error(ctx, "Failed to cleanup expired OTPs", "error", err.Error())
		return
	}
	logger.Log.InfoContext(ctx, "[Cron Job] Expired OTP cleanup successfully triggered.")
}

func (s *Scheduler) TriggerRefreshTokenCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Log.InfoContext(ctx, "[Cron Job] Triggering expired Refresh Token cleanup via Auth gRPC...")
	_, err := s.authClient.CleanupExpiredRefreshTokens(ctx, &authPb.CleanupRequest{})
	if err != nil {
		logger.Error(ctx, "Failed to cleanup expired Refresh Tokens", "error", err.Error())
		return
	}
	logger.Log.InfoContext(ctx, "[Cron Job] Expired Refresh Token cleanup successfully triggered.")
}

func (s *Scheduler) TriggerBalanceReconciliation() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger.Log.InfoContext(ctx, "[Cron Job] Triggering Balance Reconciliation Audit via Wallet gRPC...")
	res, err := s.walletClient.ReconcileBalances(ctx, &walletPb.ReconcileRequest{})
	if err != nil {
		logger.Error(ctx, "Failed to reconcile balances", "error", err.Error())
		return
	}
	logger.Log.InfoContext(ctx, "[Cron Job] Balance Reconciliation completed.", "mismatches_found", res.GetMismatchCount())
}

func (s *Scheduler) TriggerDailyReportGeneration() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Log.InfoContext(ctx, "[Cron Job] Triggering Daily Report Generation via Transaction gRPC...")
	_, err := s.txClient.GenerateDailyReport(ctx, &txPb.ReportRequest{})
	if err != nil {
		logger.Error(ctx, "Failed to trigger daily report", "error", err.Error())
		return
	}
	logger.Log.InfoContext(ctx, "[Cron Job] Daily report generation successfully triggered.")
}
