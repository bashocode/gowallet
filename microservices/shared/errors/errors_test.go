package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	err := NewAppError(http.StatusBadRequest, "TEST_ERR", "test error message")
	assert.Equal(t, "test error message", err.Error())
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Equal(t, "TEST_ERR", err.Code)
}

func TestStandardErrors(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, ErrInternalServer.StatusCode)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", ErrInternalServer.Code)

	assert.Equal(t, http.StatusBadRequest, ErrBadRequest.StatusCode)
	assert.Equal(t, "BAD_REQUEST", ErrBadRequest.Code)

	assert.Equal(t, http.StatusNotFound, ErrNotFound.StatusCode)
	assert.Equal(t, "NOT_FOUND", ErrNotFound.Code)

	assert.Equal(t, http.StatusUnavailableForLegalReasons, http.StatusUnavailableForLegalReasons) // sanity check
	assert.Equal(t, http.StatusUnauthorized, ErrUnauthorized.StatusCode)
	assert.Equal(t, "UNAUTHORIZED", ErrUnauthorized.Code)
}
