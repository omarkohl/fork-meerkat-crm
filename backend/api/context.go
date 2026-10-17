package api

import (
	"context"
	"time"

	"meerkat/clock"
	"meerkat/middleware"

	"github.com/gin-gonic/gin"
)

type ginContextKey struct{}

// ginContext returns the Gin context of the request an operation handler is serving.
func ginContext(ctx context.Context) *gin.Context {
	c, _ := ctx.Value(ginContextKey{}).(*gin.Context)
	return c
}

// UserID returns the authenticated user's ID, as set by the auth middleware of the
// operation's Gin group. ok is false when there is none (the operation is on an
// unauthenticated group, or the middleware did not run).
func UserID(ctx context.Context) (id uint, ok bool) {
	c := ginContext(ctx)
	if c == nil {
		return 0, false
	}
	v, exists := c.Get("userID")
	if !exists {
		return 0, false
	}
	id, ok = v.(uint)
	return id, ok
}

// Now is the time the clock middleware took at the start of the request.
func Now(ctx context.Context) time.Time {
	if c := ginContext(ctx); c != nil {
		return middleware.RequestNow(c)
	}
	return clock.Real().Now()
}

// Clock is the application clock, for work that reads the time again later.
func Clock(ctx context.Context) clock.Clock {
	if c := ginContext(ctx); c != nil {
		return middleware.RequestClock(c)
	}
	return clock.Real()
}
