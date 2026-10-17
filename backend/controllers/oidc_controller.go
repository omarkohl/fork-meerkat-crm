package controllers

import (
	"context"
	"crypto/subtle"
	"errors"
	"meerkat/api"
	"meerkat/middleware"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"meerkat/config"
	"meerkat/logger"
	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// returns whether OIDC is enabled and a provider name hint.
// OIDCConfigOutput is the response of GET /auth/oidc/config.
type OIDCConfigOutput struct {
	Body struct {
		Enabled      bool   `json:"enabled" doc:"Whether single sign-on is configured"`
		ProviderName string `json:"provider_name,omitempty" doc:"A display name for the provider, present when enabled"`
	}
}

// RegisterOIDCConfig registers GET /auth/oidc/config, the first operation served by huma
// (docs/adr/0011-api-contract-with-huma.md).
func RegisterOIDCConfig(a huma.API, cfg *config.Config) {
	api.Register(a, huma.Operation{
		OperationID: "get-oidc-config",
		Method:      http.MethodGet,
		Path:        "/auth/oidc/config",
		Summary:     "Single sign-on status",
		Tags:        []string{"auth"},
	}, func(context.Context, *struct{}) (*OIDCConfigOutput, error) {
		out := &OIDCConfigOutput{}
		out.Body.Enabled = cfg.OIDC.Enabled
		if cfg.OIDC.Enabled {
			out.Body.ProviderName = services.ProviderName(cfg.OIDC.ProviderURL)
		}
		return out, nil
	})
}

// generates a random state and nonce, stores in cookies, then redirects the browser
func OIDCLoginHandler(provider *services.OIDCProvider, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		state, err := services.GenerateStateToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state"})
			return
		}
		nonce, err := services.GenerateStateToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate nonce"})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("oidc_state", state, 600, "/api/v1/auth/oidc/callback", cfg.CookieDomain, cfg.CookieSecure, true)
		c.SetCookie("oidc_nonce", nonce, 600, "/api/v1/auth/oidc/callback", cfg.CookieDomain, cfg.CookieSecure, true)

		c.Redirect(http.StatusFound, provider.BuildAuthURL(state, nonce))
	}
}

// handles the provider redirect (validates state, exchanges code, finds/creates user, sets the auth cookie, redirects to /).
func OIDCCallbackHandler(provider *services.OIDCProvider, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.FromContext(c)

		// Provider-side errors (e.g. user denied consent)
		if errParam := c.Query("error"); errParam != "" {
			c.Redirect(http.StatusFound, "/login?error=oidc_denied")
			return
		}

		// Retrieve and immediately clear the state and nonce cookies.
		stateCookie, err := c.Cookie("oidc_state")
		nonceCookie, nonceErr := c.Cookie("oidc_nonce")
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("oidc_state", "", -1, "/api/v1/auth/oidc/callback", cfg.CookieDomain, cfg.CookieSecure, true)
		c.SetCookie("oidc_nonce", "", -1, "/api/v1/auth/oidc/callback", cfg.CookieDomain, cfg.CookieSecure, true)

		if err != nil || stateCookie == "" {
			log.Warn().Msg("OIDC callback: missing state cookie")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}
		if nonceErr != nil || nonceCookie == "" {
			log.Warn().Msg("OIDC callback: missing nonce cookie")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}

		stateParam := c.Query("state")
		if subtle.ConstantTimeCompare([]byte(stateCookie), []byte(stateParam)) != 1 {
			log.Warn().Msg("OIDC callback: state mismatch")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}

		code := c.Query("code")
		if code == "" {
			log.Warn().Msg("OIDC callback: missing code")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}

		idToken, oauthToken, err := provider.ExchangeAndVerify(c.Request.Context(), code)
		if err != nil {
			log.Error().Err(err).Msg("OIDC token exchange/verification failed")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}

		if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonceCookie)) != 1 {
			log.Warn().Msg("OIDC callback: nonce mismatch")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}

		claims, err := services.ExtractClaims(idToken, cfg.OIDC.ProviderURL)
		if err != nil {
			log.Error().Err(err).Msg("OIDC: failed to extract claims")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}

		// some providers expose email/profile claims via the userinfo endpoint (Fallback)
		if claims.NeedsUserInfo() {
			if err := provider.EnrichFromUserInfo(c.Request.Context(), oauthToken, claims); err != nil {
				log.Warn().Err(err).Msg("OIDC: userinfo lookup failed, continuing with id_token claims only")
			}
		}

		db := c.MustGet("db").(*gorm.DB)

		user, err := services.FindOrProvisionUser(db, claims, cfg)
		if err != nil {
			if errors.Is(err, services.ErrOIDCUserNotFound) {
				c.Redirect(http.StatusFound, "/login?error=oidc_no_account")
				return
			}
			log.Error().Err(err).Msg("OIDC: failed to find or provision user")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}

		tokenString, err := services.IssueSession(db, *user, cfg, c.ClientIP(), c.Request.UserAgent(), middleware.RequestNow(c))
		if err != nil {
			log.Error().Err(err).Uint("user_id", user.ID).Msg("OIDC: failed to generate JWT")
			c.Redirect(http.StatusFound, "/login?error=oidc_error")
			return
		}

		auditFor(c, db, &user.ID, models.AuditLogin, "method:oidc")

		maxAge := cfg.JWTExpiryHours * 3600
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("auth_token", tokenString, maxAge, "/", cfg.CookieDomain, cfg.CookieSecure, true)

		c.Redirect(http.StatusFound, "/")
	}
}
