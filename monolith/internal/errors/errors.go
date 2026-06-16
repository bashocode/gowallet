package errors

import "net/http"

type AppError struct {
	StatusCode int    `json:"-"` // we dont want to expose this to json client
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(status int, code string, msg string) *AppError {
	return &AppError{
		StatusCode: status,
		Code:       code,
		Message:    msg,
	}
}

// define the error standard that mostly use
var (
	ErrInternalServer = NewAppError(http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Something went wrong on the server, please try again later.")
	ErrBadRequest     = NewAppError(http.StatusBadRequest, "BAD_REQUEST", "Bad request, please check the request payload.")
	ErrNotFound       = NewAppError(http.StatusNotFound, "NOT_FOUND", "Resource not found.")
	ErrUnauthorized   = NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized, please check the credentials.")
)
