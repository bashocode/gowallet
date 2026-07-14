package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bashocode/gowallet/microservices/user-service/internal/user/model"
)

type OTPRepository interface {
	Create(ctx context.Context, o *model.OTP) error
	CreateTx(ctx context.Context, tx *sql.Tx, o *model.OTP) error
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

func (r *mysqlOTPRepository) CreateTx(ctx context.Context, tx *sql.Tx, o *model.OTP) error {
	query := `INSERT INTO otp_codes (id, user_id, code, type, expires_at, used) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query, o.ID, o.UserID, o.Code, o.Type, o.ExpiresAt, o.Used)
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
