package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"meerkat/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// request sends a JWT with the given sid and tv claims (nil = claim absent) to a router whose
// database was prepared by prepare.
func sessionRequest(t *testing.T, prepare func(db *gorm.DB), sid any, tv any) (int, *gorm.DB) {
	t.Helper()
	db, router := setupAuthTestRouter()
	if prepare != nil {
		prepare(db)
	}
	claims := validClaims()
	if sid != nil {
		claims["sid"] = sid
	}
	if tv != nil {
		claims["tv"] = tv
	}
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signJWT(t, jwt.SigningMethodHS256, jwtTestSecret, claims))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code, db
}

func addSession(t *testing.T, db *gorm.DB, s models.Session) {
	t.Helper()
	require.NoError(t, db.Create(&s).Error)
}

func TestAuthMiddleware_LiveSessionIsAcceptedAndTouched(t *testing.T) {
	code, db := sessionRequest(t, func(db *gorm.DB) {
		addSession(t, db, models.Session{UserID: 1, PublicID: "live", ExpiresAt: time.Now().Add(time.Hour), LastSeenAt: time.Now().Add(-time.Hour)})
	}, "live", 0)
	assert.Equal(t, http.StatusOK, code)
	var s models.Session
	require.NoError(t, db.Where("public_id = ?", "live").First(&s).Error)
	assert.WithinDuration(t, time.Now(), s.LastSeenAt, time.Minute)
}

func TestAuthMiddleware_RevokedExpiredOrUnknownSessionIsRefused(t *testing.T) {
	revoked := time.Now()
	for name, prepare := range map[string]func(db *gorm.DB){
		"revoked": func(db *gorm.DB) {
			addSession(t, db, models.Session{UserID: 1, PublicID: "s", ExpiresAt: time.Now().Add(time.Hour), RevokedAt: &revoked})
		},
		"expired": func(db *gorm.DB) {
			addSession(t, db, models.Session{UserID: 1, PublicID: "s", ExpiresAt: time.Now().Add(-time.Minute)})
		},
		"unknown": nil,
		"someone else's": func(db *gorm.DB) {
			require.NoError(t, db.Create(&models.User{Username: "other", Email: "other@example.com", Password: "x"}).Error)
			addSession(t, db, models.Session{UserID: 2, PublicID: "s", ExpiresAt: time.Now().Add(time.Hour)})
		},
	} {
		t.Run(name, func(t *testing.T) {
			code, _ := sessionRequest(t, prepare, "s", 0)
			assert.Equal(t, http.StatusUnauthorized, code)
		})
	}
}

func TestAuthMiddleware_TokenVersion(t *testing.T) {
	bump := func(db *gorm.DB) {
		require.NoError(t, db.Exec("UPDATE users SET token_version = 2 WHERE id = 1").Error)
	}
	live := func(db *gorm.DB) {
		bump(db)
		addSession(t, db, models.Session{UserID: 1, PublicID: "s", ExpiresAt: time.Now().Add(time.Hour)})
	}
	code, _ := sessionRequest(t, live, "s", 2)
	assert.Equal(t, http.StatusOK, code, "matching version")
	code, _ = sessionRequest(t, live, "s", 1)
	assert.Equal(t, http.StatusUnauthorized, code, "older version")
	code, _ = sessionRequest(t, bump, nil, nil)
	assert.Equal(t, http.StatusUnauthorized, code, "a token from before versions existed dies once the version is raised")
	code, _ = sessionRequest(t, nil, nil, nil)
	assert.Equal(t, http.StatusOK, code, "a token from before versions existed works at version 0")
	code, _ = sessionRequest(t, nil, nil, "x")
	assert.Equal(t, http.StatusUnauthorized, code, "tv of the wrong type")
	code, _ = sessionRequest(t, nil, 5, 0)
	assert.Equal(t, http.StatusUnauthorized, code, "sid of the wrong type")
}
