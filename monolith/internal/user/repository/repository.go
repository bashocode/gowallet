package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bashocode/gowallet/monolith/internal/user/model"
)

type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, u *model.User) error
	CreateTx(ctx context.Context, tx *sql.Tx, u *model.User) error
	SoftDelete(ctx context.Context, id string) error
}

type mysqlUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) UserRepository {
	return &mysqlUserRepository{db: db}
}

func (r *mysqlUserRepository) Create(ctx context.Context, u *model.User) error {
	query := `INSERT INTO users (id, full_name, email, password_hash) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.PasswordHash)
	return err
}

func (r *mysqlUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	query := `SELECT id, full_name, email, password_hash, created_at, updated_at, deleted_at FROM users WHERE id = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, id).Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}

		return nil, err
	}

	return u, nil
}

func (r *mysqlUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT id, full_name, email, password_hash, created_at, updated_at, deleted_at FROM users WHERE email = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, email).Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}

		return nil, err
	}

	return u, nil
}

func (r *mysqlUserRepository) Update(ctx context.Context, u *model.User) error {
	query := `UPDATE users SET full_name = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, u.FullName, u.ID)
	return err
}

func (r *mysqlUserRepository) CreateTx(ctx context.Context, tx *sql.Tx, u *model.User) error {
	query := `INSERT INTO users (id, full_name, email, password_hash) VALUES (?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.PasswordHash)
	return err
}

func (r *mysqlUserRepository) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
