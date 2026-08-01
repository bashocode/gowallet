package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwaggerInfo(t *testing.T) {
	assert.NotNil(t, SwaggerInfo)
	assert.Equal(t, "1.0", SwaggerInfo.Version)
	assert.Equal(t, "GoWallet Monolith API", SwaggerInfo.Title)
}
