package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"meerkat/logger"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestLoggingMiddlewareRedactsIcsToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orig := logger.Logger
	prevLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	defer func() { logger.Logger = orig; zerolog.SetGlobalLevel(prevLevel) }()
	var buf bytes.Buffer
	logger.Logger = zerolog.New(&buf)

	r := gin.New()
	r.Use(LoggingMiddleware())
	r.GET("/api/v1/ics/:file", func(c *gin.Context) { c.Status(http.StatusNotFound) })
	r.GET("/api/v1/contacts/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	token := "SECRETTOKENSECRETTOKENSECRETTOKENSECRETTOKEN1"
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/ics/"+token+".ics", nil)
	r.ServeHTTP(w, req)

	assert.NotContains(t, buf.String(), token)
	assert.Contains(t, buf.String(), "/api/v1/ics/:file")

	buf.Reset()
	req, _ = http.NewRequest("GET", "/api/v1/contacts/42", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.Contains(t, buf.String(), "/api/v1/contacts/42")
}

func TestIsSecretPathSkipsGinAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var out bytes.Buffer
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{Output: &out, Skip: IsSecretPath}))
	r.GET("/api/v1/ics/:file", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/other", func(c *gin.Context) { c.Status(http.StatusOK) })
	req, _ := http.NewRequest("GET", "/api/v1/ics/abc.ics", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.Empty(t, out.String())
	req, _ = http.NewRequest("GET", "/other", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.Contains(t, out.String(), "/other")
}
