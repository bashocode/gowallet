package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type WalletCacheRepository interface {
	GetBalance(ctx context.Context, walletID string) (int64, error)
	SetBalance(ctx context.Context, walletID string, balance int64, ttl time.Duration) error
	DeleteBalance(ctx context.Context, walletID string) error
}

type walletCacheRepo struct {
	rdb *redis.Client
}

func NewWalletCacheRepository(rdb *redis.Client) WalletCacheRepository {
	return &walletCacheRepo{rdb: rdb}
}

func (r *walletCacheRepo) GetBalance(ctx context.Context, walletID string) (int64, error) {
	key := fmt.Sprintf("wallet:balance:%s", walletID)
	val, err := r.rdb.Get(ctx, key).Int64()
	if err != nil {
		return 0, err
	}
	return val, nil
}

func (r *walletCacheRepo) SetBalance(ctx context.Context, walletID string, balance int64, ttl time.Duration) error {
	key := fmt.Sprintf("wallet:balance:%s", walletID)
	return r.rdb.Set(ctx, key, balance, ttl).Err()
}

func (r *walletCacheRepo) DeleteBalance(ctx context.Context, walletID string) error {
	key := fmt.Sprintf("wallet:balance:%s", walletID)
	return r.rdb.Del(ctx, key).Err()
}
