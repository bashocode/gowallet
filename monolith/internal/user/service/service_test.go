package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bashocode/gowallet/monolith/internal/auth"
	"github.com/bashocode/gowallet/monolith/internal/email"
	otpModel "github.com/bashocode/gowallet/monolith/internal/otp/model"
	otpRepo "github.com/bashocode/gowallet/monolith/internal/otp/repository"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	userModel "github.com/bashocode/gowallet/monolith/internal/user/model"
	userRepo "github.com/bashocode/gowallet/monolith/internal/user/repository"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	logger.InitLogger()
}


func TestRegister_Success(t *testing.T) {
	// sql mock
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// redis mock
	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	// initiate mock repositories
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	req := userModel.CreateUserRequest{
		FullName: "John Doe",
		Email:    "john.doe@example.com",
		Password: "secretpassword",
	}

	// expect sql transaction
	dbMock.ExpectBegin()
	dbMock.ExpectCommit()

	// expect mock
	mockUserRepo.On("GetByEmail", ctx, "john.doe@example.com").Return(nil, errors.New("user not found"))
	mockUserRepo.On("CreateTx", ctx, mock.Anything, mock.Anything).Return(nil)
	mockWalletRepo.On("CreateTx", ctx, mock.Anything, mock.Anything).Return(nil)
	mockOTPRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockEmailSender.On("SendEmail", mock.Anything, "john.doe@example.com", mock.Anything, mock.Anything).Return(nil)

	expectedUser := &userModel.User{
		ID:       "some-uuid",
		FullName: req.FullName,
		Email:    req.Email,
	}
	mockUserRepo.On("GetByID", ctx, mock.Anything).Return(expectedUser, nil)

	// run the function
	user, err := svc.Register(ctx, req)

	// verify result
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, req.FullName, user.FullName)
	assert.Equal(t, req.Email, user.Email)

	// give goroutine time to run
	time.Sleep(50 * time.Millisecond)

	// make sure all mock if called and expected
	mockUserRepo.AssertExpectations(t)
	mockWalletRepo.AssertExpectations(t)
	mockOTPRepo.AssertExpectations(t)
	mockEmailSender.AssertExpectations(t)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	req := userModel.CreateUserRequest{
		FullName: "John Doe",
		Email:    "john.doe@example.com",
		Password: "secretpassword",
	}

	// email already in db
	existingUser := &userModel.User{
		FullName: "John Doe",
		Email:    "john.doe@example.com",
	}
	mockUserRepo.On("GetByEmail", ctx, req.Email).Return(existingUser, nil)

	// run the function
	user, err := svc.Register(ctx, req)

	// verify result should be error and no return the user object
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, "this email already registered.", err.Error())

	// make sure all mock if called and expected
	mockUserRepo.AssertExpectations(t)
}

func TestGetProfile_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	expectedUser := &userModel.User{
		ID:       userID,
		FullName: "John Doe",
		Email:    "john.doe@example.com",
	}

	mockUserRepo.On("GetByID", ctx, userID).Return(expectedUser, nil)

	user, err := svc.GetProfile(ctx, userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedUser, user)
	mockUserRepo.AssertExpectations(t)
}

func TestGetProfile_NotFound(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "non-existent"

	mockUserRepo.On("GetByID", ctx, userID).Return(nil, errors.New("not found"))

	user, err := svc.GetProfile(ctx, userID)

	assert.Error(t, err)
	assert.Nil(t, user)
	mockUserRepo.AssertExpectations(t)
}

func TestUpdateProfile_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	req := userModel.UpdateUserRequest{
		FullName: "Jane Doe",
	}
	existingUser := &userModel.User{
		ID:       userID,
		FullName: "John Doe",
		Email:    "john.doe@example.com",
	}
	updatedUser := &userModel.User{
		ID:       userID,
		FullName: "Jane Doe",
		Email:    "john.doe@example.com",
	}

	mockUserRepo.On("GetByID", ctx, userID).Return(existingUser, nil).Once()
	mockUserRepo.On("Update", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("GetByID", ctx, userID).Return(updatedUser, nil).Once()

	user, err := svc.UpdateProfile(ctx, userID, req)

	assert.NoError(t, err)
	assert.Equal(t, "Jane Doe", user.FullName)
	mockUserRepo.AssertExpectations(t)
}

func TestUpdateProfile_NotFound(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "non-existent"
	req := userModel.UpdateUserRequest{
		FullName: "Jane Doe",
	}

	mockUserRepo.On("GetByID", ctx, userID).Return(nil, errors.New("not found"))

	user, err := svc.UpdateProfile(ctx, userID, req)

	assert.Error(t, err)
	assert.Nil(t, user)
	mockUserRepo.AssertExpectations(t)
}

func TestUpdateProfile_UpdateFailure(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	req := userModel.UpdateUserRequest{
		FullName: "Jane Doe",
	}
	existingUser := &userModel.User{
		ID:       userID,
		FullName: "John Doe",
		Email:    "john.doe@example.com",
	}

	mockUserRepo.On("GetByID", ctx, userID).Return(existingUser, nil)
	mockUserRepo.On("Update", ctx, mock.Anything).Return(errors.New("db error"))

	user, err := svc.UpdateProfile(ctx, userID, req)

	assert.Error(t, err)
	assert.Nil(t, user)
	mockUserRepo.AssertExpectations(t)
}

func TestLogin_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	mockRtRepo := new(userRepo.MockRefreshTokenRepository)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)
	svc.(*userService).rtRepo = mockRtRepo

	ctx := context.TODO()
	req := userModel.LoginRequest{
		Email:    "john.doe@example.com",
		Password: "secretpassword",
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("secretpassword"), bcrypt.DefaultCost)
	existingUser := &userModel.User{
		ID:           "user-123",
		FullName:     "John Doe",
		Email:        "john.doe@example.com",
		PasswordHash: string(hashedPassword),
	}

	mockUserRepo.On("GetByEmail", ctx, req.Email).Return(existingUser, nil)
	mockRtRepo.On("Create", ctx, mock.Anything).Return(nil)

	resp, err := svc.Login(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	mockUserRepo.AssertExpectations(t)
	mockRtRepo.AssertExpectations(t)
}

func TestLogin_InvalidCredentials_EmailNotFound(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	req := userModel.LoginRequest{
		Email:    "john.doe@example.com",
		Password: "secretpassword",
	}

	mockUserRepo.On("GetByEmail", ctx, req.Email).Return(nil, errors.New("not found"))

	resp, err := svc.Login(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	mockUserRepo.AssertExpectations(t)
}

func TestLogin_InvalidCredentials_WrongPassword(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	req := userModel.LoginRequest{
		Email:    "john.doe@example.com",
		Password: "wrongpassword",
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("secretpassword"), bcrypt.DefaultCost)
	existingUser := &userModel.User{
		ID:           "user-123",
		FullName:     "John Doe",
		Email:        "john.doe@example.com",
		PasswordHash: string(hashedPassword),
	}

	mockUserRepo.On("GetByEmail", ctx, req.Email).Return(existingUser, nil)

	resp, err := svc.Login(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	mockUserRepo.AssertExpectations(t)
}

func TestUpdateAvatar_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	path := "/uploads/avatar.png"

	mockUserRepo.On("UpdateAvatar", ctx, userID, path).Return(nil)

	err := svc.UpdateAvatar(ctx, userID, path)

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestUpdateAvatar_Failure(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	path := "/uploads/avatar.png"

	mockUserRepo.On("UpdateAvatar", ctx, userID, path).Return(errors.New("db error"))

	err := svc.UpdateAvatar(ctx, userID, path)

	assert.Error(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestDeleteAccount_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	existingUser := &userModel.User{
		ID:       userID,
		FullName: "John Doe",
		Email:    "john.doe@example.com",
	}

	mockUserRepo.On("GetByID", ctx, userID).Return(existingUser, nil)
	mockUserRepo.On("SoftDelete", ctx, userID).Return(nil)

	err := svc.DeleteAccount(ctx, userID)

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestDeleteAccount_NotFound(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"

	mockUserRepo.On("GetByID", ctx, userID).Return(nil, errors.New("not found"))

	err := svc.DeleteAccount(ctx, userID)

	assert.Error(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestDeleteAccount_SoftDeleteFailure(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	existingUser := &userModel.User{
		ID:       userID,
		FullName: "John Doe",
		Email:    "john.doe@example.com",
	}

	mockUserRepo.On("GetByID", ctx, userID).Return(existingUser, nil)
	mockUserRepo.On("SoftDelete", ctx, userID).Return(errors.New("db error"))

	err := svc.DeleteAccount(ctx, userID)

	assert.Error(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestLogout_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, mockRedis := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	mockRtRepo := new(userRepo.MockRefreshTokenRepository)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)
	svc.(*userService).rtRepo = mockRtRepo

	ctx := context.TODO()
	userID := "user-123"
	email := "test@example.com"
	role := "user"

	token, err := auth.GenerateToken(userID, email, role, 15*time.Minute)
	assert.NoError(t, err)

	blacklistKey := fmt.Sprintf("blacklist:%s", token)
	mockRedis.CustomMatch(func(expected, actual []interface{}) error {
		if len(actual) >= 3 && actual[0] == "set" && actual[1] == blacklistKey && actual[2] == "logged_out" {
			return nil
		}
		return fmt.Errorf("expected set for key %s, got %v", blacklistKey, actual)
	}).ExpectSet(blacklistKey, "logged_out", 15*time.Minute).SetVal("OK")

	mockRtRepo.On("RevokeAllByUserID", ctx, userID).Return(nil)

	err = svc.Logout(ctx, token)
	assert.NoError(t, err)
	assert.NoError(t, mockRedis.ExpectationsWereMet())
	mockRtRepo.AssertExpectations(t)
}

func TestLogout_InvalidToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	invalidToken := "invalid.token.here"

	err := svc.Logout(ctx, invalidToken)
	assert.Error(t, err)
	assert.Equal(t, "token is invalid or expired.", err.Error())
}

func TestLogout_RedisError(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, mockRedis := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	mockRtRepo := new(userRepo.MockRefreshTokenRepository)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)
	svc.(*userService).rtRepo = mockRtRepo

	ctx := context.TODO()
	userID := "user-123"
	email := "test@example.com"
	role := "user"

	token, err := auth.GenerateToken(userID, email, role, 15*time.Minute)
	assert.NoError(t, err)

	blacklistKey := fmt.Sprintf("blacklist:%s", token)
	mockRedis.CustomMatch(func(expected, actual []interface{}) error {
		if len(actual) >= 3 && actual[0] == "set" && actual[1] == blacklistKey && actual[2] == "logged_out" {
			return nil
		}
		return fmt.Errorf("expected set for key %s, got %v", blacklistKey, actual)
	}).ExpectSet(blacklistKey, "logged_out", 15*time.Minute).SetErr(errors.New("redis failure"))

	mockRtRepo.On("RevokeAllByUserID", ctx, userID).Return(nil)

	err = svc.Logout(ctx, token)
	assert.Error(t, err)
	assert.Equal(t, "Something went wrong on the server, please try again later.", err.Error())
	assert.NoError(t, mockRedis.ExpectationsWereMet())
	mockRtRepo.AssertExpectations(t)
}

func TestVerifyEmail_Success(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	code := "123456"

	otpData := &otpModel.OTP{
		ID:     "otp-uuid",
		UserID: userID,
		Code:   code,
		Type:   "email_verification",
	}

	mockOTPRepo.On("GetActiveOTP", ctx, userID, code, "email_verification").Return(otpData, nil)

	dbMock.ExpectBegin()
	mockUserRepo.On("UpdateVerificationStatusTx", ctx, mock.Anything, userID, true).Return(nil)
	mockOTPRepo.On("MarkAsUsedTx", ctx, mock.Anything, "otp-uuid").Return(nil)
	dbMock.ExpectCommit()

	err = svc.VerifyEmail(ctx, userID, code)

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
	mockOTPRepo.AssertExpectations(t)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestVerifyEmail_InvalidOTP(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	userID := "user-123"
	code := "111111"

	mockOTPRepo.On("GetActiveOTP", ctx, userID, code, "email_verification").Return(nil, errors.New("otp not found"))

	err := svc.VerifyEmail(ctx, userID, code)

	assert.Error(t, err)
	assert.Equal(t, "invalid or expired verification code.", err.Error())
	mockOTPRepo.AssertExpectations(t)
}

func TestRequestPasswordReset_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	emailAddr := "test@example.com"
	u := &userModel.User{
		ID:    "user-uuid",
		Email: emailAddr,
	}

	mockUserRepo.On("GetByEmailNoErrorNotFound", ctx, emailAddr).Return(u, nil)
	mockOTPRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockEmailSender.On("SendEmail", mock.Anything, emailAddr, mock.Anything, mock.Anything).Return(nil)

	err := svc.RequestPasswordReset(ctx, emailAddr)
	assert.NoError(t, err)
	time.Sleep(50 * time.Millisecond) // Wait for email goroutine
	mockUserRepo.AssertExpectations(t)
	mockOTPRepo.AssertExpectations(t)
}

func TestRequestPasswordReset_UserNotFound(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	emailAddr := "nonexistent@example.com"

	mockUserRepo.On("GetByEmailNoErrorNotFound", ctx, emailAddr).Return(nil, nil)

	err := svc.RequestPasswordReset(ctx, emailAddr)
	assert.NoError(t, err) // Should return nil (no-op) to prevent email enumeration
	mockUserRepo.AssertExpectations(t)
}

func TestVerifyPasswordReset_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	emailAddr := "test@example.com"
	code := "123456"
	u := &userModel.User{
		ID:    "user-uuid",
		Email: emailAddr,
	}
	otpData := &otpModel.OTP{
		ID:     "otp-uuid",
		UserID: "user-uuid",
		Code:   code,
		Type:   "password_reset",
	}

	mockUserRepo.On("GetByEmailNoErrorNotFound", ctx, emailAddr).Return(u, nil)
	mockOTPRepo.On("GetActiveOTP", ctx, "user-uuid", code, "password_reset").Return(otpData, nil)
	mockOTPRepo.On("MarkAsUsed", ctx, "otp-uuid").Return(nil)

	userID, err := svc.VerifyPasswordReset(ctx, emailAddr, code)
	assert.NoError(t, err)
	assert.Equal(t, "user-uuid", userID)
	mockUserRepo.AssertExpectations(t)
	mockOTPRepo.AssertExpectations(t)
}

func TestResetPassword_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	mockRtRepo := new(userRepo.MockRefreshTokenRepository)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)
	svc.(*userService).rtRepo = mockRtRepo

	ctx := context.TODO()
	userID := "user-uuid"
	newPassword := "newsecurepass"

	mockUserRepo.On("UpdatePassword", ctx, userID, mock.Anything).Return(nil)
	mockRtRepo.On("RevokeAllByUserID", ctx, userID).Return(nil)

	err := svc.ResetPassword(ctx, userID, newPassword)
	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
	mockRtRepo.AssertExpectations(t)
}

func TestGetAllUsers_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()
	users := []*userModel.User{
		{ID: "user-1", FullName: "User One", Email: "one@example.com"},
		{ID: "user-2", FullName: "User Two", Email: "two@example.com"},
	}

	mockUserRepo.On("GetAll", ctx).Return(users, nil)

	result, err := svc.GetAllUsers(ctx)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "user-1", result[0].ID)
	mockUserRepo.AssertExpectations(t)
}

func TestGetAllUsers_Failure(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	rdb, _ := redismock.NewClientMock()
	defer rdb.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	mockOTPRepo := new(otpRepo.MockOTPRepository)
	mockEmailSender := new(email.MockEmailSender)
	svc := NewUserService(db, rdb, mockUserRepo, mockWalletRepo, mockOTPRepo, mockEmailSender)

	ctx := context.TODO()

	mockUserRepo.On("GetAll", ctx).Return(nil, errors.New("db error"))

	result, err := svc.GetAllUsers(ctx)
	assert.Error(t, err)
	assert.Nil(t, result)
	mockUserRepo.AssertExpectations(t)
}

