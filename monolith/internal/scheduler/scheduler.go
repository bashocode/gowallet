package scheduler

import (
	"database/sql"

	ledgerRepo "github.com/bashocode/gowallet/monolith/internal/ledger/repository"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron       *cron.Cron
	db         *sql.DB
	walletRepo walletRepo.WalletRepository
	ledgerRepo ledgerRepo.LedgerRepository
}

func NewScheduler(
	db *sql.DB,
	wRepo walletRepo.WalletRepository,
	lRepo ledgerRepo.LedgerRepository,
) *Scheduler {
	c := cron.New(cron.WithSeconds())

	s := &Scheduler{
		cron:       c,
		db:         db,
		walletRepo: wRepo,
		ledgerRepo: lRepo,
	}

	return s
}

func (s *Scheduler) Start() {
	// clean expired OTP tokens every 30 minutes
	s.cron.AddFunc("0 */30 * * * *", s.CleanupExpiredOTPs)
	// daily balance reconciliation at 2 AM
	s.cron.AddFunc("0 0 2 * * *", s.ReconcileAllBalances)
	// clean expired refresh token daily at 3 AM
	s.cron.AddFunc("0 0 3 * * *", s.CleanupExpiredRefreshTokens)
	// export daily transaction to csv at 23.59 PM
	s.cron.AddFunc("0 59 23 * * *", s.ExportDailyTransactions)

	s.cron.Start()
	logger.Log.Info("Background scheduler successfully started!")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	logger.Log.Info("Background scheduler stopped.")
}
