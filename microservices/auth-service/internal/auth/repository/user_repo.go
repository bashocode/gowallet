package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/model"
)

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type mysqlUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) UserRepository {
	return &mysqlUserRepository{db: db}
}

func (r *mysqlUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT id, full_name, email, role, password_hash, avatar_url, is_verified, created_at, updated_at FROM users WHERE email = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash, &u.AvatarURL, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return u, nil
}

func (r *mysqlUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	query := `SELECT id, full_name, email, role, password_hash, avatar_url, is_verified, created_at, updated_at FROM users WHERE id = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash, &u.AvatarURL, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return u, nil
}
