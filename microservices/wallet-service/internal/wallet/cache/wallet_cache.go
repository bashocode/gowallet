package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type WalletCacheRepository interface {
	GetBalance(ctx context.Context, walletID string) (string, error)
	SetBalance(ctx context.Context, walletID string, balance string, ttl time.Duration) error
	DeleteBalance(ctx context.Context, walletID string) error
}

type walletCacheRepo struct {
	rdb *redis.Client
}

func NewWalletCacheRepository(rdb *redis.Client) WalletCacheRepository {
	return &walletCacheRepo{rdb: rdb}
}

func (r *walletCacheRepo) GetBalance(ctx context.Context, walletID string) (string, error) {
	key := fmt.Sprintf("wallet:balance:%s", walletID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *walletCacheRepo) SetBalance(ctx context.Context, walletID string, balance string, ttl time.Duration) error {
	key := fmt.Sprintf("wallet:balance:%s", walletID)
	return r.rdb.Set(ctx, key, balance, ttl).Err()
}

func (r *walletCacheRepo) DeleteBalance(ctx context.Context, walletID string) error {
	key := fmt.Sprintf("wallet:balance:%s", walletID)
	return r.rdb.Del(ctx, key).Err()
}
