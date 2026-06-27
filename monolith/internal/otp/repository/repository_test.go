package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bashocode/gowallet/monolith/internal/otp/model"
	"github.com/stretchr/testify/assert"
)

func TestOTP_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLOTPRRepository(db)

	now := time.Now()
	otp := &model.OTP{
		ID:        "1",
		UserID:    "user1",
		Code:      "123456",
		Type:      "email_verification",
		ExpiresAt: now,
		Used:      false,
	}

	mock.ExpectExec("INSERT INTO otp_codes").
		WithArgs("1", "user1", "123456", "email_verification", now, false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), otp)
	assert.NoError(t, err)
}

func TestOTP_GetActiveOTP(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLOTPRRepository(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "code", "type", "expires_at", "used"}).
		AddRow("1", "user1", "123456", "email_verification", now, false)

	mock.ExpectQuery("SELECT id, user_id, code, type, expires_at, used").
		WithArgs("user1", "123456", "email_verification").
		WillReturnRows(rows)

	otp, err := repo.GetActiveOTP(context.Background(), "user1", "123456", "email_verification")
	assert.NoError(t, err)
	assert.Equal(t, "1", otp.ID)
}

func TestOTP_GetActiveOTPTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLOTPRRepository(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "code", "type", "expires_at", "used"}).
		AddRow("1", "user1", "123456", "email_verification", now, false)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, user_id, code, type, expires_at, used").
		WithArgs("user1", "123456", "email_verification").
		WillReturnRows(rows)
	mock.ExpectCommit()

	tx, err := db.Begin()
	assert.NoError(t, err)

	otp, err := repo.GetActiveOTPTx(context.Background(), tx, "user1", "123456", "email_verification")
	assert.NoError(t, err)
	assert.Equal(t, "1", otp.ID)

	tx.Commit()
}

func TestOTP_MarkAsUsed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLOTPRRepository(db)

	mock.ExpectExec("UPDATE otp_codes SET used = 1 WHERE id = ?").
		WithArgs("1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.MarkAsUsed(context.Background(), "1")
	assert.NoError(t, err)
}

func TestOTP_MarkAsUsedTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLOTPRRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE otp_codes SET used = 1 WHERE id = ?").
		WithArgs("1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.Begin()
	assert.NoError(t, err)

	err = repo.MarkAsUsedTx(context.Background(), tx, "1")
	assert.NoError(t, err)

	tx.Commit()
}
