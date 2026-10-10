package middleware

import (
	"meerkat/logger"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// icsRoutePattern is the route pattern of the public calendar feed, whose path
// carries the secret token.
const icsRoutePattern = "/ics/:file"

// IsSecretPath reports whether the request path contains a credential (the ICS feed
// token) and so must never be logged.
func IsSecretPath(c *gin.Context) bool {
	return strings.HasSuffix(c.FullPath(), icsRoutePattern)
}

// LoggedPath returns the path to write to logs: the route pattern for secret paths.
func LoggedPath(c *gin.Context) string {
	if IsSecretPath(c) {
		return c.FullPath()
	}
	return c.Request.URL.Path
}

// LoggingMiddleware logs HTTP requests with structured logging
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := LoggedPath(c)
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Get status code
		statusCode := c.Writer.Status()

		// Get request ID
		requestID, _ := c.Get("request_id")
		requestIDStr, _ := requestID.(string)

		// Get user ID if available
		var userID uint
		if uid, exists := c.Get("userID"); exists {
			if id, ok := uid.(uint); ok {
				userID = id
			}
		}

		// Create log event
		event := logger.Logger.Info()

		// Add level based on status code
		if statusCode >= 500 {
			event = logger.Logger.Error()
		} else if statusCode >= 400 {
			event = logger.Logger.Warn()
		}

		// Build log entry
		logEntry := event.
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", statusCode).
			Dur("duration", duration).
			Str("ip", c.ClientIP()).
			Str("user_agent", c.Request.UserAgent())

		if requestIDStr != "" {
			logEntry = logEntry.Str("request_id", requestIDStr)
		}

		if userID > 0 {
			logEntry = logEntry.Uint("user_id", userID)
		}

		if query != "" {
			logEntry = logEntry.Str("query", query)
		}

		// Add error message if present
		if len(c.Errors) > 0 {
			logEntry = logEntry.Str("error", c.Errors.String())
		}

		// Log the request
		logEntry.Msg("HTTP request")
	}
}
