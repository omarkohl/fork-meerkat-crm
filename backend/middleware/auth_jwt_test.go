package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"meerkat/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func jwtRequest(t *testing.T, token string) (int, map[string]any) {
	t.Helper()
	_, router := setupAuthTestRouter()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func signJWT(t *testing.T, method jwt.SigningMethod, secret string, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"username": "authtest",
		"user_id":  1,
		"exp":      time.Now().Add(time.Hour).Unix(),
	}
}

func TestAuthMiddleware_JWT_Valid(t *testing.T) {
	code, body := jwtRequest(t, signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, validClaims()))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(1), body["user_id"])
}

func TestAuthMiddleware_JWT_Rejected(t *testing.T) {
	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Hour).Unix()

	noUser := validClaims()
	delete(noUser, "user_id")

	valid := signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, validClaims())
	parts := strings.Split(valid, ".")
	tampered := parts[0] + "." + parts[1] + "." + parts[2][:len(parts[2])-2] + "xx"

	noExp := validClaims()
	delete(noExp, "exp")

	stringExp := validClaims()
	stringExp["exp"] = "tomorrow"

	futureIat := validClaims()
	futureIat["iat"] = time.Now().Add(time.Hour).Unix()

	noneToken, err := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims()).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		token   string
		wantMsg string
	}{
		{"expired", signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, expired), "Token expired"},
		{"wrong secret", signJWT(t, jwt.SigningMethodHS256, "another-secret-another-secret-123", validClaims()), "Invalid token signature"},
		{"tampered signature", tampered, "Invalid token signature"},
		{"alg none", noneToken, "Invalid token"},
		{"malformed", "not-a-jwt", "Malformed token"},
		{"no exp", signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, noExp), "Invalid token"},
		{"string exp", signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, stringExp), "Invalid token"},
		{"iat in the future", signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, futureIat), "Invalid token"},
		{"HS384 (only HS256 is issued)", signJWT(t, jwt.SigningMethodHS384, jwtTestSecret, validClaims()), "Invalid token"},
		{"missing user_id", signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, noUser), "Invalid token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body := jwtRequest(t, tt.token)
			assert.Equal(t, http.StatusUnauthorized, code)
			assert.Equal(t, tt.wantMsg, body["error"])
		})
	}
}

func TestAuthMiddleware_JWT_NonHMACMethodRejected(t *testing.T) {
	// An RS256 header with an HMAC-style signature must not be accepted.
	valid := signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, validClaims())
	parts := strings.Split(valid, ".")
	rsHeader := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9" // {"alg":"RS256","typ":"JWT"}
	code, body := jwtRequest(t, rsHeader+"."+parts[1]+"."+parts[2])
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "Invalid token", body["error"])
}

func TestAuthMiddleware_JWT_DeletedUserRejected(t *testing.T) {
	db, router := setupAuthTestRouter()
	token := signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, validClaims())

	do := func() (int, map[string]any) {
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body
	}

	code, _ := do()
	assert.Equal(t, http.StatusOK, code)

	assert.NoError(t, db.Delete(&models.User{}, 1).Error)

	code, body := do()
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "Invalid token", body["error"])
}

func TestAuthMiddleware_JWT_MissingUserIDRejected(t *testing.T) {
	claims := validClaims()
	delete(claims, "user_id")
	code, body := jwtRequest(t, signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, claims))
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "Invalid token", body["error"])
}

func TestAuthMiddleware_JWT_UnknownUserRejected(t *testing.T) {
	claims := validClaims()
	claims["user_id"] = 999
	code, _ := jwtRequest(t, signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, claims))
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestAuthMiddleware_JWT_BadUserIDRejected(t *testing.T) {
	for name, id := range map[string]any{"zero": 0, "negative": -3, "fraction": 1.5, "string": "1", "huge": 1e20} {
		t.Run(name, func(t *testing.T) {
			claims := validClaims()
			claims["user_id"] = id
			code, _ := jwtRequest(t, signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, claims))
			assert.Equal(t, http.StatusUnauthorized, code)
		})
	}
}
