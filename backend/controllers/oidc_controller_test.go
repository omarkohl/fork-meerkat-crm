package controllers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"meerkat/config"
	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeIdP is a minimal OpenID provider: discovery, JWKS and a token endpoint that hands out an
// ID token with the claims set by the test.
type fakeIdP struct {
	srv    *httptest.Server
	key    *rsa.PrivateKey
	claims jwt.MapClaims
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	idp := &fakeIdP{key: key}
	mux := http.NewServeMux()
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": idp.srv.URL, "authorization_endpoint": idp.srv.URL + "/authorize",
			"token_endpoint": idp.srv.URL + "/token", "jwks_uri": idp.srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, idp.claims)
		tok.Header["kid"] = "k1"
		signed, err := tok.SignedString(key)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "id_token": signed})
	})
	return idp
}

func (idp *fakeIdP) idClaims(nonce, sub, email string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": idp.srv.URL, "aud": "client", "sub": sub, "nonce": nonce, "email": email,
		"email_verified": true, "name": "Test User", "preferred_username": "tester2",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	}
}

func oidcRouter(t *testing.T, idp *fakeIdP, db *gorm.DB, mutate func(*config.Config)) (*gin.Engine, *config.Config) {
	t.Helper()
	cfg := &config.Config{JWTSecretKey: "test-secret", JWTExpiryHours: 1}
	cfg.OIDC = config.OIDCConfig{Enabled: true, ProviderURL: idp.srv.URL, ClientID: "client", ClientSecret: "s", RedirectURL: "http://app/cb", AllowAutoProvision: true}
	if mutate != nil {
		mutate(cfg)
	}
	provider, err := services.InitOIDCProvider(context.Background(), cfg)
	require.NoError(t, err)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Next() })
	r.GET("/config", OIDCConfigHandler(cfg))
	r.GET("/login", OIDCLoginHandler(provider, cfg))
	r.GET("/api/v1/auth/oidc/callback", OIDCCallbackHandler(provider, cfg))
	return r, cfg
}

func cookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	var found *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			found = c
		}
	}
	return found
}

func get(r *gin.Engine, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestOIDCLoginSetsStateAndNonceCookies(t *testing.T) {
	db, _ := setupRouter()
	idp := newFakeIdP(t)
	r, _ := oidcRouter(t, idp, db, nil)

	w := get(r, "/login")
	require.Equal(t, http.StatusFound, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	state, nonce := cookie(w, "oidc_state"), cookie(w, "oidc_nonce")
	require.NotNil(t, state)
	require.NotNil(t, nonce)
	assert.Equal(t, state.Value, loc.Query().Get("state"))
	assert.Equal(t, nonce.Value, loc.Query().Get("nonce"))
	assert.Len(t, state.Value, 64)
	assert.NotEqual(t, state.Value, nonce.Value)
	for _, c := range []*http.Cookie{state, nonce} {
		assert.True(t, c.HttpOnly)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
		assert.Equal(t, "/api/v1/auth/oidc/callback", c.Path)
		assert.Positive(t, c.MaxAge)
	}
	// Every login gets fresh values.
	w2 := get(r, "/login")
	assert.NotEqual(t, state.Value, cookie(w2, "oidc_state").Value)
}

func TestOIDCConfigHandler(t *testing.T) {
	db, _ := setupRouter()
	idp := newFakeIdP(t)
	r, _ := oidcRouter(t, idp, db, nil)
	w := get(r, "/config")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":true`)
}

func TestOIDCCallbackRejectsBadStateAndMissingParts(t *testing.T) {
	db, _ := setupRouter()
	idp := newFakeIdP(t)
	r, _ := oidcRouter(t, idp, db, nil)
	st := &http.Cookie{Name: "oidc_state", Value: "state-1"}
	no := &http.Cookie{Name: "oidc_nonce", Value: "nonce-1"}

	cases := map[string]*httptest.ResponseRecorder{
		"no cookies":        get(r, "/api/v1/auth/oidc/callback?state=state-1&code=c"),
		"no nonce cookie":   get(r, "/api/v1/auth/oidc/callback?state=state-1&code=c", st),
		"no state param":    get(r, "/api/v1/auth/oidc/callback?code=c", st, no),
		"state mismatch":    get(r, "/api/v1/auth/oidc/callback?state=other&code=c", st, no),
		"missing code":      get(r, "/api/v1/auth/oidc/callback?state=state-1", st, no),
		"empty state match": get(r, "/api/v1/auth/oidc/callback?state=&code=c", &http.Cookie{Name: "oidc_state", Value: ""}, no),
	}
	for name, w := range cases {
		assert.Equal(t, http.StatusFound, w.Code, name)
		assert.Equal(t, "/login?error=oidc_error", w.Header().Get("Location"), name)
		assert.Nil(t, cookie(w, "auth_token"), name+": no session")
	}

	w := get(r, "/api/v1/auth/oidc/callback?error=access_denied", st, no)
	assert.Equal(t, "/login?error=oidc_denied", w.Header().Get("Location"))
}

func TestOIDCCallbackLogsInAndClearsCookies(t *testing.T) {
	db, _ := setupRouter()
	idp := newFakeIdP(t)
	idp.claims = idp.idClaims("nonce-1", "sub-42", "new@example.com")
	r, _ := oidcRouter(t, idp, db, nil)

	w := get(r, "/api/v1/auth/oidc/callback?state=state-1&code=c",
		&http.Cookie{Name: "oidc_state", Value: "state-1"}, &http.Cookie{Name: "oidc_nonce", Value: "nonce-1"})
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))
	auth := cookie(w, "auth_token")
	require.NotNil(t, auth)
	assert.True(t, auth.HttpOnly)
	assert.NotEmpty(t, auth.Value)
	assert.Negative(t, cookie(w, "oidc_state").MaxAge, "state cookie is single use")
	assert.Negative(t, cookie(w, "oidc_nonce").MaxAge, "nonce cookie is single use")

	var u models.User
	require.NoError(t, db.Where("email = ?", "new@example.com").First(&u).Error)
	assert.Equal(t, "sub-42", *u.OIDCSubject)
}

func TestOIDCCallbackRejectsNonceMismatch(t *testing.T) {
	db, _ := setupRouter()
	idp := newFakeIdP(t)
	idp.claims = idp.idClaims("attacker-nonce", "sub-1", "x@example.com")
	r, _ := oidcRouter(t, idp, db, nil)

	w := get(r, "/api/v1/auth/oidc/callback?state=state-1&code=c",
		&http.Cookie{Name: "oidc_state", Value: "state-1"}, &http.Cookie{Name: "oidc_nonce", Value: "my-nonce"})
	assert.Equal(t, "/login?error=oidc_error", w.Header().Get("Location"))
	assert.Nil(t, cookie(w, "auth_token"))
	var n int64
	db.Model(&models.User{}).Where("email = ?", "x@example.com").Count(&n)
	assert.Zero(t, n, "no account is created for a rejected login")
}

func TestOIDCCallbackRejectsTokenForOtherAudienceOrExpired(t *testing.T) {
	db, _ := setupRouter()
	idp := newFakeIdP(t)
	r, _ := oidcRouter(t, idp, db, nil)
	cookies := []*http.Cookie{{Name: "oidc_state", Value: "s"}, {Name: "oidc_nonce", Value: "n"}}

	idp.claims = idp.idClaims("n", "sub", "a@example.com")
	idp.claims["aud"] = "someone-else"
	w := get(r, "/api/v1/auth/oidc/callback?state=s&code=c", cookies...)
	assert.Equal(t, "/login?error=oidc_error", w.Header().Get("Location"))

	idp.claims = idp.idClaims("n", "sub", "a@example.com")
	idp.claims["exp"] = time.Now().Add(-time.Hour).Unix()
	w = get(r, "/api/v1/auth/oidc/callback?state=s&code=c", cookies...)
	assert.Equal(t, "/login?error=oidc_error", w.Header().Get("Location"))
}

func TestOIDCCallbackWithoutAccountAndNoAutoProvision(t *testing.T) {
	db, _ := setupRouter()
	idp := newFakeIdP(t)
	idp.claims = idp.idClaims("n", "sub", "stranger@example.com")
	r, _ := oidcRouter(t, idp, db, func(c *config.Config) { c.OIDC.AllowAutoProvision = false })
	w := get(r, "/api/v1/auth/oidc/callback?state=s&code=c", &http.Cookie{Name: "oidc_state", Value: "s"}, &http.Cookie{Name: "oidc_nonce", Value: "n"})
	assert.Equal(t, "/login?error=oidc_no_account", w.Header().Get("Location"))
	assert.Nil(t, cookie(w, "auth_token"))
}
