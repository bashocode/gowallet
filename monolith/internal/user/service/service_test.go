package service

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	userModel "github.com/bashocode/gowallet/monolith/internal/user/model"
	userRepo "github.com/bashocode/gowallet/monolith/internal/user/repository"
	walletRepo "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func TestRegister_Success(t *testing.T) {
	// sql mock
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// initiate mock repositories
	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	// make sure all mock if called and expected
	mockUserRepo.AssertExpectations(t)
	mockWalletRepo.AssertExpectations(t)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	resp, err := svc.Login(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	mockUserRepo.AssertExpectations(t)
}

func TestLogin_InvalidCredentials_EmailNotFound(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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

	mockUserRepo := new(userRepo.MockUserRepository)
	mockWalletRepo := new(walletRepo.MockWalletRepository)
	svc := NewUserService(db, mockUserRepo, mockWalletRepo)

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
