package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bashocode/gowallet/microservices/user-service/internal/user/model"
)

// === UserRepository ===

type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByEmailNoErrorNotFound(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, u *model.User) error
	CreateTx(ctx context.Context, tx *sql.Tx, u *model.User) error
	UpdateAvatar(ctx context.Context, id string, path string) error
	SoftDelete(ctx context.Context, id string) error
	UpdateVerificationStatus(ctx context.Context, id string, verified bool) error
	UpdateVerificationStatusTx(ctx context.Context, tx *sql.Tx, id string, verified bool) error
	UpdatePassword(ctx context.Context, id string, passwordHash string) error
	GetByOAuth(ctx context.Context, provider, oauthID string) (*model.User, error)
	GetAll(ctx context.Context, params model.PaginationParams) ([]*model.User, int64, error)
}

type mysqlUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) UserRepository {
	return &mysqlUserRepository{db: db}
}

func (r *mysqlUserRepository) Create(ctx context.Context, u *model.User) error {
	query := `INSERT INTO users (id, full_name, email, password_hash, oauth_provider, oauth_id, avatar_url, is_verified) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.PasswordHash, u.OAuthProvider, u.OAuthID, u.AvatarURL, u.IsVerified)
	return err
}

func (r *mysqlUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	query := `SELECT id, full_name, email, role, password_hash, oauth_provider, 
		oauth_id, avatar_url, is_verified, created_at, updated_at, 
		deleted_at FROM users WHERE id = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash,
		&u.OAuthProvider, &u.OAuthID, &u.AvatarURL, &u.IsVerified,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return u, nil
}

func (r *mysqlUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT id, full_name, email, role, password_hash, avatar_url, is_verified, created_at, updated_at, deleted_at FROM users WHERE email = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, email).Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash, &u.AvatarURL, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return u, nil
}

func (r *mysqlUserRepository) GetByEmailNoErrorNotFound(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT id, full_name, email, role, password_hash, avatar_url, is_verified, created_at, updated_at, deleted_at FROM users WHERE email = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, email).Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash, &u.AvatarURL, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
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
	query := `INSERT INTO users (id, full_name, email, password_hash, oauth_provider, oauth_id, avatar_url, is_verified) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.PasswordHash, u.OAuthProvider, u.OAuthID, u.AvatarURL, u.IsVerified)
	return err
}

func (r *mysqlUserRepository) UpdateAvatar(ctx context.Context, id string, path string) error {
	query := `UPDATE users SET avatar_url = ? WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, path, id)
	return err
}

func (r *mysqlUserRepository) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *mysqlUserRepository) UpdateVerificationStatus(ctx context.Context, id string, verified bool) error {
	query := `UPDATE users SET is_verified = ? WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, verified, id)
	return err
}

func (r *mysqlUserRepository) UpdateVerificationStatusTx(ctx context.Context, tx *sql.Tx, id string, verified bool) error {
	query := `UPDATE users SET is_verified = ? WHERE id = ? AND deleted_at IS NULL`
	_, err := tx.ExecContext(ctx, query, verified, id)
	return err
}

func (r *mysqlUserRepository) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	query := `UPDATE users SET password_hash = ? WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, passwordHash, id)
	return err
}

func (r *mysqlUserRepository) GetByOAuth(ctx context.Context, provider, oauthID string) (*model.User, error) {
	query := `SELECT id, full_name, email, role, password_hash, oauth_provider, oauth_id, avatar_url, is_verified, created_at, updated_at, deleted_at FROM users WHERE oauth_provider = ? AND oauth_id = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, provider, oauthID).Scan(
		&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash, &u.OAuthProvider, &u.OAuthID, &u.AvatarURL, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return u, nil
}

func (r *mysqlUserRepository) GetAll(ctx context.Context, params model.PaginationParams) ([]*model.User, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	allowedSortColumns := map[string]bool{
		"id":         true,
		"full_name":  true,
		"email":      true,
		"role":       true,
		"created_at": true,
		"updated_at": true,
	}
	sortCol := "created_at"
	if allowedSortColumns[params.Sort] {
		sortCol = params.Sort
	}

	orderDir := "DESC"
	if params.Order == "asc" || params.Order == "ASC" {
		orderDir = "ASC"
	}

	query := fmt.Sprintf(`SELECT id, full_name, email, role, oauth_provider, oauth_id, avatar_url, is_verified, created_at, updated_at, deleted_at FROM users WHERE deleted_at IS NULL ORDER BY %s %s LIMIT ? OFFSET ?`, sortCol, orderDir)
	rows, err := r.db.QueryContext(ctx, query, params.Limit, params.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		err := rows.Scan(
			&u.ID, &u.FullName, &u.Email, &u.Role, &u.OAuthProvider, &u.OAuthID, &u.AvatarURL, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// === OTPRepository ===

type OTPRepository interface {
	Create(ctx context.Context, o *model.OTP) error
	GetActiveOTP(ctx context.Context, userID string, code string, otpType string) (*model.OTP, error)
	GetActiveOTPTx(ctx context.Context, tx *sql.Tx, userID string, code string, otpType string) (*model.OTP, error)
	MarkAsUsed(ctx context.Context, id string) error
	MarkAsUsedTx(ctx context.Context, tx *sql.Tx, id string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type mysqlOTPRepository struct {
	db *sql.DB
}

func NewMySQLOTPRepository(db *sql.DB) OTPRepository {
	return &mysqlOTPRepository{db: db}
}

func (r *mysqlOTPRepository) Create(ctx context.Context, o *model.OTP) error {
	query := `INSERT INTO otp_codes (id, user_id, code, type, expires_at, used) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, o.ID, o.UserID, o.Code, o.Type, o.ExpiresAt, o.Used)
	return err
}

func (r *mysqlOTPRepository) GetActiveOTP(ctx context.Context, userID string, code string, otpType string) (*model.OTP, error) {
	query := `SELECT id, user_id, code, type, expires_at, used 
				FROM otp_codes WHERE user_id = ? AND code = ? 
				AND type = ? AND expires_at > NOW() AND used = 0`
	o := &model.OTP{}
	err := r.db.QueryRowContext(ctx, query, userID, code, otpType).
		Scan(&o.ID, &o.UserID, &o.Code, &o.Type, &o.ExpiresAt, &o.Used)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("active OTP not found or expired")
		}
		return nil, err
	}
	return o, nil
}

func (r *mysqlOTPRepository) GetActiveOTPTx(ctx context.Context, tx *sql.Tx, userID string, code string, otpType string) (*model.OTP, error) {
	query := `SELECT id, user_id, code, type, expires_at, used 
				FROM otp_codes WHERE user_id = ? AND code = ? 
				AND type = ? AND expires_at > NOW() AND used = 0 FOR UPDATE`
	o := &model.OTP{}
	err := tx.QueryRowContext(ctx, query, userID, code, otpType).
		Scan(&o.ID, &o.UserID, &o.Code, &o.Type, &o.ExpiresAt, &o.Used)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("active OTP not found or expired")
		}
		return nil, err
	}
	return o, nil
}

func (r *mysqlOTPRepository) MarkAsUsed(ctx context.Context, id string) error {
	query := `UPDATE otp_codes SET used = 1 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *mysqlOTPRepository) MarkAsUsedTx(ctx context.Context, tx *sql.Tx, id string) error {
	query := `UPDATE otp_codes SET used = 1 WHERE id = ?`
	_, err := tx.ExecContext(ctx, query, id)
	return err
}

func (r *mysqlOTPRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM otp_codes WHERE expires_at <= NOW()`
	res, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
