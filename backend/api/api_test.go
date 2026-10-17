package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"meerkat/api"
	"meerkat/clock"
	apperrors "meerkat/errors"
	"meerkat/middleware"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type echoIn struct {
	Body struct {
		Name string `json:"name" minLength:"1" doc:"a name"`
	}
}
type echoOut struct {
	Body struct {
		Name string `json:"name"`
	}
}

// newTestAPI mounts huma on a Gin group that runs the real request-clock middleware, a
// group-level middleware and an operation-level one, like routes.go does.
func newTestAPI(t *testing.T) (*gin.Engine, huma.API, *[]string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var order []string
	r := gin.New()
	r.Use(middleware.RequestIDMiddleware(), middleware.ClockMiddleware(clock.NewFake(time.Date(2031, 2, 3, 4, 5, 6, 0, time.UTC))))
	group := r.Group("/api/v1", func(c *gin.Context) {
		order = append(order, "group")
		c.Next()
	})
	a := api.New(group, r)
	api.Register(a, huma.Operation{OperationID: "echo", Method: http.MethodPost, Path: "/echo"},
		func(ctx context.Context, in *echoIn) (*echoOut, error) {
			out := &echoOut{}
			out.Body.Name = in.Body.Name
			return out, nil
		}, func(c *gin.Context) {
			order = append(order, "route")
			c.Next()
		})
	api.Register(a, huma.Operation{OperationID: "teapot", Method: http.MethodGet, Path: "/fail"},
		func(ctx context.Context, _ *struct{}) (*echoOut, error) {
			return nil, apperrors.ErrForbidden("nope")
		})
	return r, a, &order
}

func do(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGinMiddlewareRunsAroundHumaOperations(t *testing.T) {
	r, _, order := newTestAPI(t)
	w := do(r, http.MethodPost, "/api/v1/echo", `{"name":"x"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"name":"x"}`, w.Body.String(), "no $schema field is added to responses")
	assert.Equal(t, []string{"group", "route"}, *order, "group middleware first, then the operation's own")
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"), "the request ID middleware ran")
}

type errBody struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

func TestValidationErrorsKeepTheAppErrorFormat(t *testing.T) {
	r, _, _ := newTestAPI(t)
	w := do(r, http.MethodPost, "/api/v1/echo", `{"name":""}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, apperrors.ErrCodeValidation, b.Error.Code)
	assert.Contains(t, b.Error.Details, "name")
	assert.NotEmpty(t, b.RequestID)
	assert.Equal(t, "2031-02-03T04:05:06Z", b.Timestamp, "the timestamp comes from the request clock")
}

func TestMalformedJSONIsAnInvalidInputError(t *testing.T) {
	r, _, _ := newTestAPI(t)
	w := do(r, http.MethodPost, "/api/v1/echo", `{not json`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, apperrors.ErrCodeInvalidInput, b.Error.Code)
}

func TestHandlersCanReturnAppErrors(t *testing.T) {
	r, _, _ := newTestAPI(t)
	w := do(r, http.MethodGet, "/api/v1/fail", "")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, apperrors.ErrCodeForbidden, b.Error.Code)
	assert.Equal(t, "nope", b.Error.Message)
	assert.NotEmpty(t, b.RequestID)
}

func TestSpecIsOpenAPI31WithTheErrorSchema(t *testing.T) {
	_, a, _ := newTestAPI(t)
	spec, err := json.Marshal(a.OpenAPI())
	require.NoError(t, err)
	assert.Contains(t, string(spec), `"openapi":"3.1`)
	assert.Contains(t, string(spec), `"/echo"`)
	assert.Contains(t, string(spec), `"request_id"`, "the documented error body is the AppError response")
}
