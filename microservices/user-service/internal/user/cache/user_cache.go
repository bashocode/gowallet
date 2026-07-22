package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/user-service/internal/user/model"
	"github.com/redis/go-redis/v9"
)

type UserCacheRepository interface {
	GetUserByID(ctx context.Context, userID string) (*model.User, error)
	SetUserByID(ctx context.Context, userID string, user *model.User, ttl time.Duration) error
	DeleteUserByID(ctx context.Context, userID string) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	SetUserByEmail(ctx context.Context, email string, user *model.User, ttl time.Duration) error
	DeleteUserByEmail(ctx context.Context, email string) error
}

type userCacheRepo struct {
	rdb *redis.Client
}

func NewUserCacheRepository(rdb *redis.Client) UserCacheRepository {
	return &userCacheRepo{rdb: rdb}
}

func (r *userCacheRepo) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	key := fmt.Sprintf("user:profile:%s", userID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var user model.User
	if err := json.Unmarshal([]byte(val), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userCacheRepo) SetUserByID(ctx context.Context, userID string, user *model.User, ttl time.Duration) error {
	key := fmt.Sprintf("user:profile:%s", userID)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, key, data, ttl).Err()
}

func (r *userCacheRepo) DeleteUserByID(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user:profile:%s", userID)
	return r.rdb.Del(ctx, key).Err()
}

func (r *userCacheRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	key := fmt.Sprintf("user:email:%s", email)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var user model.User
	if err := json.Unmarshal([]byte(val), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userCacheRepo) SetUserByEmail(ctx context.Context, email string, user *model.User, ttl time.Duration) error {
	key := fmt.Sprintf("user:email:%s", email)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, key, data, ttl).Err()
}

func (r *userCacheRepo) DeleteUserByEmail(ctx context.Context, email string) error {
	key := fmt.Sprintf("user:email:%s", email)
	return r.rdb.Del(ctx, key).Err()
}
