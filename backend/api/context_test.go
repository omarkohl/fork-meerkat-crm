package api_test

import (
	"context"
	"testing"

	"meerkat/api"

	"github.com/stretchr/testify/assert"
)

func TestLoggerWorksWithAndWithoutARequest(t *testing.T) {
	assert.NotNil(t, api.Logger(context.Background()))
}
