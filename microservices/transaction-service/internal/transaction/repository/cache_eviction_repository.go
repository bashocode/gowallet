package repository

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

type CacheEvictionRepository interface {
	EvictWalletCache(ctx context.Context, userID string, walletID string) error
}

type cacheEvictionRepo struct {
	rdb *redis.Client
}

func NewCacheEvictionRepository(rdb *redis.Client) CacheEvictionRepository {
	return &cacheEvictionRepo{rdb: rdb}
}

func (r *cacheEvictionRepo) EvictWalletCache(ctx context.Context, userID string, walletID string) error {
	keyUser := fmt.Sprintf("wallet:user:%s", userID)
	keyID := fmt.Sprintf("wallet:id:%s", walletID)
	
	var errs []error
	
	if userID != "" {
		if err := r.rdb.Del(ctx, keyUser).Err(); err != nil {
			log.Printf("Warning: Failed to evict cache key %s: %v", keyUser, err)
			errs = append(errs, err)
		} else {
			log.Printf("Successfully evicted cache key: %s", keyUser)
		}
	}
	
	if walletID != "" {
		if err := r.rdb.Del(ctx, keyID).Err(); err != nil {
			log.Printf("Warning: Failed to evict cache key %s: %v", keyID, err)
			errs = append(errs, err)
		} else {
			log.Printf("Successfully evicted cache key: %s", keyID)
		}
	}
	
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
