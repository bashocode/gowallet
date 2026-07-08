package scheduler

import (
	"github.com/bashocode/gowallet/microservices/shared/logger"
	authPb "github.com/bashocode/gowallet/microservices/auth-service/proto/auth"
	txPb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	walletPb "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/robfig/cron/v3"
)

// Scheduler is a lightweight cron orchestrator. It owns NO database connection:
// per the Database-per-Service boundary, every job triggers domain logic in the
// owning service through internal gRPC.
type Scheduler struct {
	cron         *cron.Cron
	authClient   authPb.AuthServiceClient
	walletClient walletPb.WalletServiceClient
	txClient     txPb.TransactionServiceClient
}

func NewScheduler(
	authClient authPb.AuthServiceClient,
	walletClient walletPb.WalletServiceClient,
	txClient txPb.TransactionServiceClient,
) *Scheduler {
	c := cron.New(cron.WithSeconds())
	return &Scheduler{
		cron:         c,
		authClient:   authClient,
		walletClient: walletClient,
		txClient:     txClient,
	}
}

func (s *Scheduler) Start() {
	// 1. Job 1: Clean expired OTP tokens every 30 minutes
	s.cron.AddFunc("0 */30 * * * *", s.TriggerOTPCleanup)

	// 2. Job 2: Daily Balance Reconciliation at 02:00 AM
	s.cron.AddFunc("0 0 2 * * *", s.TriggerBalanceReconciliation)

	// 3. Job 3: Clean expired Refresh Tokens daily at 03:00 AM
	s.cron.AddFunc("0 0 3 * * *", s.TriggerRefreshTokenCleanup)

	// 4. Job 4: Export daily transaction report at 23:59 PM
	s.cron.AddFunc("0 59 23 * * *", s.TriggerDailyReportGeneration)

	s.cron.Start()
	logger.Log.Info("Centralized Scheduler Service started successfully!")
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	logger.Log.Info("Centralized Scheduler Service stopped.")
}
