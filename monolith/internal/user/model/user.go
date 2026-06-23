package model

import "time"

type User struct {
	ID           string     `json:"id"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // don't expose hash password to json
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	IsVerified   bool       `json:"is_verified"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

type CreateUserRequest struct {
	FullName string `json:"full_name" binding:"required" example:"John Doe"`
	Email    string `json:"email" binding:"required,email" example:"john.doe@example.com"`
	Password string `json:"password" binding:"required,min=6" example:"secretpassword"` // more long, better
}

type UpdateUserRequest struct {
	FullName string `json:"full_name" binding:"required"`
}
