package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type LedgerCacheRepository interface {
	GetBalance(ctx context.Context, walletID string) (string, error)
	SetBalance(ctx context.Context, walletID string, balance string, ttl time.Duration) error
	DeleteBalance(ctx context.Context, walletID string) error
}

type ledgerCacheRepo struct {
	rdb *redis.Client
}

func NewLedgerCacheRepository(rdb *redis.Client) LedgerCacheRepository {
	return &ledgerCacheRepo{rdb: rdb}
}

func (r *ledgerCacheRepo) GetBalance(ctx context.Context, walletID string) (string, error) {
	key := fmt.Sprintf("ledger:balance:%s", walletID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *ledgerCacheRepo) SetBalance(ctx context.Context, walletID string, balance string, ttl time.Duration) error {
	key := fmt.Sprintf("ledger:balance:%s", walletID)
	return r.rdb.Set(ctx, key, balance, ttl).Err()
}

func (r *ledgerCacheRepo) DeleteBalance(ctx context.Context, walletID string) error {
	key := fmt.Sprintf("ledger:balance:%s", walletID)
	return r.rdb.Del(ctx, key).Err()
}
