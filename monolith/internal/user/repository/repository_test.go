package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bashocode/gowallet/monolith/internal/user/model"
	"github.com/stretchr/testify/assert"
)

func TestUser_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLUserRepository(db)
	u := &model.User{
		ID:           "u1",
		FullName:     "John Doe",
		Email:        "john@example.com",
		PasswordHash: "hash123",
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs("u1", "John Doe", "john@example.com", "hash123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), u)
	assert.NoError(t, err)
}

func TestUser_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLUserRepository(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "full_name", "email", "role", "password_hash", "oauth_provider", "oauth_id", "avatar_url", "is_verified", "created_at", "updated_at", "deleted_at"}).
		AddRow("u1", "John Doe", "john@example.com", "user", "hash123", nil, nil, nil, true, now, now, nil)

	mock.ExpectQuery("SELECT id, full_name, email, role, password_hash, oauth_provider, oauth_id, avatar_url, is_verified, created_at, updated_at, deleted_at FROM users").
		WithArgs("u1").
		WillReturnRows(rows)

	res, err := repo.GetByID(context.Background(), "u1")
	assert.NoError(t, err)
	assert.Equal(t, "John Doe", res.FullName)
}

func TestRefreshToken_CreateAndGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := NewMySQLRefreshTokenRepository(db)

	expires := time.Now().Add(time.Hour)
	rt := &model.RefreshToken{
		ID:        "rt1",
		UserID:    "u1",
		Token:     "token123",
		ExpiresAt: expires,
		Revoked:   false,
	}

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs("rt1", "u1", "token123", expires, false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), rt)
	assert.NoError(t, err)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "revoked", "created_at"}).
		AddRow("rt1", "u1", "token123", expires, false, now)

	mock.ExpectQuery("SELECT id, user_id, token, expires_at, revoked, created_at FROM refresh_tokens").
		WithArgs("token123").
		WillReturnRows(rows)

	res, err := repo.GetByToken(context.Background(), "token123")
	assert.NoError(t, err)
	assert.Equal(t, "u1", res.UserID)
}
