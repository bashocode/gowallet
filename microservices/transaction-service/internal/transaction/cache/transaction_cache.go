package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/model"
	"github.com/redis/go-redis/v9"
)

type TransactionCacheRepository interface {
	GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error)
	SetByIdempotencyKey(ctx context.Context, key string, tx *model.Transaction, ttl time.Duration) error
	DeleteByIdempotencyKey(ctx context.Context, key string) error
}

type transactionCacheRepo struct {
	rdb *redis.Client
}

func NewTransactionCacheRepository(rdb *redis.Client) TransactionCacheRepository {
	return &transactionCacheRepo{rdb: rdb}
}

func (r *transactionCacheRepo) GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error) {
	cacheKey := fmt.Sprintf("transaction:idempotency:%s", key)
	val, err := r.rdb.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, err
	}

	var tx model.Transaction
	if err := json.Unmarshal([]byte(val), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *transactionCacheRepo) SetByIdempotencyKey(ctx context.Context, key string, tx *model.Transaction, ttl time.Duration) error {
	cacheKey := fmt.Sprintf("transaction:idempotency:%s", key)
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, cacheKey, data, ttl).Err()
}

func (r *transactionCacheRepo) DeleteByIdempotencyKey(ctx context.Context, key string) error {
	cacheKey := fmt.Sprintf("transaction:idempotency:%s", key)
	return r.rdb.Del(ctx, cacheKey).Err()
}
