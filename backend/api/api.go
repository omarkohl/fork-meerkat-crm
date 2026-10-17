// Package api mounts huma (code-first OpenAPI) on the existing Gin router groups.
// Gin middleware (request ID, clock, auth, rate limits) still runs around every
// operation, errors keep the AppError JSON format, and the generated OpenAPI 3.1
// document is the API contract (docs/adr/0011-api-contract-with-huma.md).
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"meerkat/clock"
	apperrors "meerkat/errors"
	"meerkat/middleware"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

// ginMiddlewareKey holds the per-operation Gin middleware in Operation.Metadata.
const (
	ginMiddlewareKey = "gin.middleware"
	ginGroupKey      = "gin.group"
)

var setupErrors sync.Once

// New returns a huma API whose operations are registered on group (so the group's
// middleware runs) and served by handler (the engine). It exposes no docs, schema or
// spec routes: the spec is generated offline by cmd/openapi.
func New(group gin.IRoutes, handler http.Handler) huma.API {
	setupErrors.Do(func() { huma.NewError = newError })

	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	cfg := huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: "3.1.0",
			Info: &huma.Info{
				Title:       "Meerkat CRM API",
				Description: "The JSON API of Meerkat CRM. Errors use the AppError format: {\"error\": {\"code\", \"message\", \"details\"}, \"request_id\", \"timestamp\"}.",
				Version:     "1.0.0",
			},
			Servers:    []*huma.Server{{URL: "/api/v1"}},
			Components: &huma.Components{Schemas: registry},
		},
		Formats:       huma.DefaultFormats,
		DefaultFormat: "application/json",
		Transformers:  []huma.Transformer{errorTransformer},
	}
	return huma.NewAPI(cfg, &adapter{Handler: handler, router: group})
}

// Register adds an operation. ginMiddleware runs after the group's middleware and
// before the handler, like a middleware argument of router.POST(path, mw, handler).
func Register[I, O any](a huma.API, op huma.Operation, handler func(context.Context, *I) (*O, error), ginMiddleware ...gin.HandlerFunc) {
	if len(ginMiddleware) > 0 {
		if op.Metadata == nil {
			op.Metadata = map[string]any{}
		}
		op.Metadata[ginMiddlewareKey] = ginMiddleware
	}
	register(a, op, handler)
}

// RegisterOn is Register for an operation that belongs on another Gin group than the
// API's default, typically the authenticated group: its middleware (auth, rate limit)
// then runs around the operation. A nil group means the default one.
func RegisterOn[I, O any](a huma.API, group gin.IRoutes, op huma.Operation, handler func(context.Context, *I) (*O, error), ginMiddleware ...gin.HandlerFunc) {
	if group != nil {
		if op.Metadata == nil {
			op.Metadata = map[string]any{}
		}
		op.Metadata[ginGroupKey] = group
	}
	Register(a, op, handler, ginMiddleware...)
}

func register[I, O any](a huma.API, op huma.Operation, handler func(context.Context, *I) (*O, error)) {
	if op.MaxBodyBytes == 0 {
		// The limit the global body-size middleware already enforces on every request.
		op.MaxBodyBytes = middleware.DefaultMaxBodySize
	}
	huma.Register(a, op, handler)
	relaxRequestBody(a, op)
}

type adapter struct {
	http.Handler
	router gin.IRoutes
}

func (a *adapter) Handle(op *huma.Operation, handler func(huma.Context)) {
	path := strings.ReplaceAll(strings.ReplaceAll(op.Path, "{", ":"), "}", "")
	var handlers []gin.HandlerFunc
	if mws, ok := op.Metadata[ginMiddlewareKey].([]gin.HandlerFunc); ok {
		handlers = append(handlers, mws...)
	}
	handlers = append(handlers, func(c *gin.Context) {
		handler(huma.WithValue(humagin.NewContext(op, c), ginContextKey{}, c))
	})
	router := a.router
	if g, ok := op.Metadata[ginGroupKey].(gin.IRoutes); ok {
		router = g
	}
	router.Handle(op.Method, path, handlers...)
}

// Error is the response body of every error: the AppError format. It also implements
// huma.StatusError so huma knows the status code.
type Error struct {
	apperrors.ErrorResponse
	status int
	app    *apperrors.AppError // what is logged; the response body is built from it
}

// GetStatus returns the HTTP status code.
func (e *Error) GetStatus() int { return e.status }

// Error implements error.
func (e *Error) Error() string { return e.ErrorResponse.Error.Message }

func fromAppError(e *apperrors.AppError) *Error {
	return &Error{
		ErrorResponse: apperrors.ErrorResponse{Error: apperrors.ErrorDetail{Code: e.Code, Message: e.Message, Details: e.Details}},
		status:        e.HTTPStatus,
		app:           e,
	}
}

// newError replaces huma.NewError so that errors huma raises itself (validation, bad
// JSON, unsupported media type) come out in the AppError format and with the status
// codes the API used before huma: 400 for validation, not 422.
func newError(status int, msg string, errs ...error) huma.StatusError {
	switch status {
	case http.StatusUnprocessableEntity:
		e := apperrors.ErrValidation("Request validation failed")
		whole, kept := true, 0
		for _, err := range errs {
			if d, ok := err.(huma.ErrorDetailer); ok {
				detail := d.ErrorDetail()
				if strings.HasPrefix(detail.Message, "expected required property ") && strings.HasPrefix(detail.Location, "body") {
					// The `validate:"required"` tag already reports a missing field (relaxRequestBody
					// makes huma's required set the same set), once and in the project's wording.
					continue
				}
				kept++
				if detail.Location != "body" {
					whole = false
				}
				e.WithDetails(fieldOf(detail.Location), detail.Message)
				continue
			}
			whole = false
			kept++
			e.WithDetails("request", err.Error())
		}
		if whole && kept == 1 {
			// The body as a whole is unusable (empty, wrong type): ShouldBindJSON said so too.
			return fromAppError(apperrors.ErrInvalidInput("request body", fmt.Sprint(e.Details["request body"])))
		}
		return fromAppError(e)
	case http.StatusBadRequest:
		return fromAppError(apperrors.ErrInvalidInput("request body", msg))
	case http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
		return fromAppError(apperrors.NewError(apperrors.ErrCodeInvalidInput, msg, status))
	}
	code := apperrors.ErrCodeOperationFailed
	switch status {
	case http.StatusUnauthorized:
		code = apperrors.ErrCodeUnauthorized
	case http.StatusForbidden:
		code = apperrors.ErrCodeForbidden
	case http.StatusNotFound:
		code = apperrors.ErrCodeNotFound
	case http.StatusConflict:
		code = apperrors.ErrCodeConflict
	case http.StatusTooManyRequests:
		code = apperrors.ErrCodeRateLimitExceeded
	default:
		if status >= 500 {
			code = apperrors.ErrCodeInternal
		}
	}
	appErr := apperrors.NewError(code, msg, status)
	if len(errs) > 0 {
		appErr = appErr.WithError(errs[0]) // the cause is logged, never sent
	}
	return fromAppError(appErr)
}

// fieldOf turns a huma location such as "body.password" or "body.emails[0].value" into
// the field name the old validation errors used ("password", "emails[0].value").
func fieldOf(location string) string {
	for _, prefix := range []string{"body.", "query.", "path.", "header."} {
		if strings.HasPrefix(location, prefix) {
			return strings.TrimPrefix(location, prefix)
		}
	}
	if location == "" || location == "body" {
		return "request body"
	}
	return location
}

// errorTransformer turns an *apperrors.AppError returned by a handler into the response
// body and fills request_id and timestamp from the Gin context, like
// apperrors.RespondWithError does.
func errorTransformer(ctx huma.Context, _ string, v any) (any, error) {
	var e *Error
	switch t := v.(type) {
	case *apperrors.AppError:
		e = fromAppError(t)
	case *Error:
		e = t
	default:
		return v, nil
	}
	gc := humagin.Unwrap(ctx)
	apperrors.LogError(gc, e.app)
	if id, ok := gc.Get("request_id"); ok {
		e.RequestID, _ = id.(string)
	}
	e.Timestamp = requestTime(gc).UTC().Format("2006-01-02T15:04:05Z07:00")
	return e, nil
}

func requestTime(c *gin.Context) time.Time {
	if v, ok := c.Get(clock.ContextKey); ok {
		if tv, ok := v.(time.Time); ok {
			return tv
		}
	}
	return clock.Real().Now()
}
