package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"meerkat/api"
	"meerkat/logger"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contactBody mirrors a real DTO: required comes from the validate tag, not from the
// missing omitempty, and nested items are validated too.
type contactBody struct {
	Firstname string `json:"firstname" validate:"required,min=1,max=10"`
	Email     string `json:"email" validate:"omitempty,email"`
	Note      string `json:"note"`
	Phones    []struct {
		Value string `json:"value" validate:"phone"`
	} `json:"phones" validate:"dive"`
}

type contactIn struct {
	Body contactBody
}

func (i *contactIn) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	return api.ValidateBody(prefix, &i.Body)
}

func contactAPI() http.Handler {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	a := api.New(r.Group("/api/v1"), r)
	api.Register(a, huma.Operation{OperationID: "create-contact", Method: http.MethodPost, Path: "/contacts"},
		func(context.Context, *contactIn) (*struct{}, error) { return nil, nil })
	api.Register(a, huma.Operation{OperationID: "boom", Method: http.MethodGet, Path: "/boom"},
		func(context.Context, *struct{}) (*struct{}, error) { return nil, errors.New("disk on fire") })
	return r
}

func details(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b), w.Body.String())
	return b.Error.Details
}

func TestUnknownJSONPropertiesAreIgnoredLikeShouldBindJSON(t *testing.T) {
	w := do(contactAPI(), http.MethodPost, "/api/v1/contacts", `{"firstname":"Al","something_new":1}`)
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
}

func TestRequiredComesFromTheValidateTagAndIsReportedOnce(t *testing.T) {
	w := do(contactAPI(), http.MethodPost, "/api/v1/contacts", `{"email":"a@example.com"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	d := details(t, w)
	assert.Equal(t, map[string]any{"firstname": "firstname is required"}, d, "one detail, keyed by the JSON name")
}

func TestDetailKeysAreJSONNamesAlsoInNestedItems(t *testing.T) {
	w := do(contactAPI(), http.MethodPost, "/api/v1/contacts", `{"firstname":"Al","email":"nope","phones":[{"value":"abc"}]}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	d := details(t, w)
	assert.Contains(t, d, "email")
	assert.Contains(t, d, "phones[0].value")
}

func TestRequiredFieldIsAbsentFromOptionalOnes(t *testing.T) {
	// optional fields (no required tag) may be missing even though their JSON tag has no omitempty
	w := do(contactAPI(), http.MethodPost, "/api/v1/contacts", `{"firstname":"Al"}`)
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
}

func TestEmptyBodyIsAnInvalidInputError(t *testing.T) {
	w := do(contactAPI(), http.MethodPost, "/api/v1/contacts", ``)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, "INVALID_INPUT", b.Error.Code)
}

func TestTooLargeAndWrongMediaTypeAreInvalidInput(t *testing.T) {
	h := contactAPI()
	huge := `{"firstname":"` + strings.Repeat("a", 11<<20) + `"}`
	w := do(h, http.MethodPost, "/api/v1/contacts", huge)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, "INVALID_INPUT", b.Error.Code)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contacts", strings.NewReader("x"))
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, "INVALID_INPUT", b.Error.Code)
}

func TestErrorsAreLoggedAndTheCauseIsNotSent(t *testing.T) {
	var buf bytes.Buffer
	old := logger.Logger
	logger.Logger = zerolog.New(&buf)
	t.Cleanup(func() { logger.Logger = old })

	w := do(contactAPI(), http.MethodGet, "/api/v1/boom", "")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "disk on fire", "the cause stays server side")
	assert.Contains(t, buf.String(), "disk on fire", "the cause is logged")
	assert.Contains(t, buf.String(), "Request error")
}

// Accepted differences from gin's c.JSON (ADR 0011): a trailing newline, no \u003c-style
// HTML escaping, and Content-Type "application/json" without a charset.
func TestResponseBytesDifferFromGinInKnownWays(t *testing.T) {
	r, _, _ := newTestAPI(t)
	w := do(r, http.MethodPost, "/api/v1/echo", `{"name":"a<b&c"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "{\"name\":\"a<b&c\"}\n", w.Body.String())
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}
