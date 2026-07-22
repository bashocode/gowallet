package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/model"
	"github.com/redis/go-redis/v9"
)

type WalletCacheRepository interface {
	GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error)
	SetWalletByUserID(ctx context.Context, userID string, wallet *model.Wallet, ttl time.Duration) error
	DeleteWalletByUserID(ctx context.Context, userID string) error
	DeleteWalletByID(ctx context.Context, walletID string) error
}

type walletCacheRepo struct {
	rdb *redis.Client
}

func NewWalletCacheRepository(rdb *redis.Client) WalletCacheRepository {
	return &walletCacheRepo{rdb: rdb}
}

func (r *walletCacheRepo) GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	key := fmt.Sprintf("wallet:user:%s", userID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var wallet model.Wallet
	if err := json.Unmarshal([]byte(val), &wallet); err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *walletCacheRepo) SetWalletByUserID(ctx context.Context, userID string, wallet *model.Wallet, ttl time.Duration) error {
	keyUser := fmt.Sprintf("wallet:user:%s", userID)
	keyID := fmt.Sprintf("wallet:id:%s", wallet.ID)

	data, err := json.Marshal(wallet)
	if err != nil {
		return err
	}

	if err := r.rdb.Set(ctx, keyUser, data, ttl).Err(); err != nil {
		return err
	}
	return r.rdb.Set(ctx, keyID, data, ttl).Err()
}

func (r *walletCacheRepo) DeleteWalletByUserID(ctx context.Context, userID string) error {
	key := fmt.Sprintf("wallet:user:%s", userID)
	return r.rdb.Del(ctx, key).Err()
}

func (r *walletCacheRepo) DeleteWalletByID(ctx context.Context, walletID string) error {
	key := fmt.Sprintf("wallet:id:%s", walletID)
	return r.rdb.Del(ctx, key).Err()
}
