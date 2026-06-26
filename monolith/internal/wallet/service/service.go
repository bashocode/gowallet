package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	customErr "github.com/bashocode/gowallet/monolith/internal/errors"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/wallet/model"
	"github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/redis/go-redis/v9"
)

type WalletService interface {
	GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error)
}

type walletService struct {
	repo repository.WalletRepository
	rdb  *redis.Client
}

func NewWalletService(repo repository.WalletRepository, rdb *redis.Client) WalletService {
	return &walletService{
		repo: repo,
		rdb:  rdb,
	}
}

func (s *walletService) GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	cacheKey := fmt.Sprintf("wallet:user:%s", userID)

	// check if data exist in redis
	cachedVal, err := s.rdb.Get(ctx, cacheKey).Result()
	if err != nil {
		if err != redis.Nil {
			// redis is down or has an issues, don't fail the request
			logger.Warn(
				ctx,
				"Redis error during cache lookup, falling back to MySQL",
				"error", err.Error(), "user_id", userID,
			)
		}
		// cache miss or redis down
		logger.Info(
			ctx,
			"Cache miss for wallet, fetching from MySQL...",
			"user_id", userID,
		)
	} else {
		// cache hit, Deserialize JSON string to model.Wallet struct
		wallet := &model.Wallet{}
		if err := json.Unmarshal([]byte(cachedVal), wallet); err == nil {
			logger.Info(
				ctx,
				"Cache hit for wallet, returning from Redis",
				"user_id", userID,
			)

			return wallet, nil
		}

		logger.Warn(
			ctx,
			"Failed to deserialize cached wallet data, falling back to MySQL",
			"error", err.Error(), "user_id", userID,
		)
	}

	wallet, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, customErr.NewAppError(http.StatusNotFound, "WALLET_NOT_FOUND", "wallet not found")
	}

	// save to redis for 5 minutes TTL
	walletBytes, err := json.Marshal(wallet)
	if err == nil {
		s.rdb.Set(ctx, cacheKey, walletBytes, 5*time.Minute)
	}

	return wallet, nil
}
