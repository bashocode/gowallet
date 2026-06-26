package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bashocode/gowallet/monolith/internal/user/model"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *model.RefreshToken) error
	GetByToken(ctx context.Context, token string) (*model.RefreshToken, error)
	Revoke(ctx context.Context, token string) error
	RevokeAllByUserID(ctx context.Context, userID string) error
}

type mysqlRefreshTokenRepository struct {
	db *sql.DB
}

func NewMySQLRefreshTokenRepository(db *sql.DB) RefreshTokenRepository {
	return &mysqlRefreshTokenRepository{db: db}
}

func (r *mysqlRefreshTokenRepository) Create(ctx context.Context, rt *model.RefreshToken) error {
	query := `INSERT INTO refresh_tokens (id, user_id, token, expires_at, revoked) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, rt.ID, rt.UserID, rt.Token, rt.ExpiresAt, rt.Revoked)
	return err
}

func (r *mysqlRefreshTokenRepository) GetByToken(ctx context.Context, token string) (*model.RefreshToken, error) {
	query := `SELECT id, user_id, token, expires_at, revoked, created_at FROM refresh_tokens WHERE token = ?`
	rt := &model.RefreshToken{}
	err := r.db.QueryRowContext(ctx, query, token).Scan(&rt.ID, &rt.UserID, &rt.Token, &rt.ExpiresAt, &rt.Revoked, &rt.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("token not found")
		}
		return nil, err
	}
	return rt, nil
}

func (r *mysqlRefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	query := `UPDATE refresh_tokens SET revoked = 1 WHERE token = ?`
	_, err := r.db.ExecContext(ctx, query, token)
	return err
}

func (r *mysqlRefreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID string) error {
	query := `UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
