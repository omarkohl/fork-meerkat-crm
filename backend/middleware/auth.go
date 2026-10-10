package middleware

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"meerkat/config"
	"meerkat/idutil"
	"meerkat/logger"
	"meerkat/models"
	"meerkat/session"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// First, try to get token from httpOnly cookie
		if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
			tokenString = cookie
		} else {
			// Fall back to Authorization header (for API clients like CardDAV)
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
				c.Abort()
				return
			}

			// Check if Authorization header is formatted properly
			if !strings.HasPrefix(authHeader, "Bearer ") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must start with Bearer"})
				c.Abort()
				return
			}

			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Handle API tokens (meerkat_ prefix)
		if strings.HasPrefix(tokenString, "meerkat_") {
			db := c.MustGet("db").(*gorm.DB)
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(tokenString)))
			var apiToken models.ApiToken
			if err := db.Where("token_hash = ? AND revoked_at IS NULL", hash).First(&apiToken).Error; err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				c.Abort()
				return
			}
			if !userExists(c, db, apiToken.UserID) {
				return
			}
			c.Set("userID", apiToken.UserID)
			c.Set("isAPIToken", true)
			go func(id uint) {
				if err := db.Model(&models.ApiToken{}).Where("id = ?", id).Update("last_used_at", time.Now()).Error; err != nil {
					logger.Logger.Warn().Err(err).Uint("api_token_id", id).Msg("Failed to update api token last_used_at")
				}
			}(apiToken.ID)
			c.Next()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(cfg.JWTSecretKey), nil
		},
			// Tokens are only issued as HS256 with an exp claim (UserService).
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(), // v4 rejected a future iat by default; v5 needs this
		)

		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
				c.Abort()
				return
			}
			if errors.Is(err, jwt.ErrTokenMalformed) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Malformed token"})
				c.Abort()
				return
			}
			if errors.Is(err, jwt.ErrSignatureInvalid) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token signature"})
				c.Abort()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			if username, exists := claims["username"].(string); exists {
				c.Set("username", username)
			}

			// Tokens are issued with a numeric user_id, which JSON decodes as float64.
			idValue, _ := claims["user_id"].(float64)
			userID, ok := idutil.FromFloat(idValue)
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				c.Abort()
				return
			}

			// Users are soft-deleted; a signed token must not outlive its account. The same
			// lookup yields the token version ("sign out everywhere" raises it).
			db := c.MustGet("db").(*gorm.DB)
			version, found := userTokenVersion(c, db, userID)
			if !found {
				return
			}
			if !sessionStillValid(c, db, userID, version, claims) {
				return
			}
			c.Set("userID", userID)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// userExists reports whether the (not soft-deleted) user exists. Otherwise it has
// already answered: 401 when the user is gone, 500 on a database error.
func userExists(c *gin.Context, db *gorm.DB, userID uint) bool {
	var count int64
	if err := db.Model(&models.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		logger.Logger.Error().Err(err).Uint("user_id", userID).Msg("Failed to look up the authenticated user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		c.Abort()
		return false
	}
	if count == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		c.Abort()
		return false
	}
	return true
}

// userTokenVersion returns the user's token version. Like userExists, it has already
// answered when it returns false.
func userTokenVersion(c *gin.Context, db *gorm.DB, userID uint) (int, bool) {
	var user models.User
	err := db.Select("id", "token_version").Where("id = ?", userID).Take(&user).Error
	switch {
	case err == nil:
		return user.TokenVersion, true
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
	default:
		logger.Logger.Error().Err(err).Uint("user_id", userID).Msg("Failed to look up the authenticated user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
	c.Abort()
	return 0, false
}

// sessionStillValid checks the token's version ("tv", absent in tokens issued before it
// existed, which count as 0) against the user's, and the session ("sid") against the
// sessions table: revoked and expired sessions are refused even though the JWT is valid.
// Tokens without a "sid" are from before sessions existed and have no session to check.
// It has already answered when it returns false.
func sessionStillValid(c *gin.Context, db *gorm.DB, userID uint, version int, claims jwt.MapClaims) bool {
	refuse := func(msg string) bool {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
		c.Abort()
		return false
	}

	tokenVersion := 0
	if raw, present := claims["tv"]; present {
		f, isNumber := raw.(float64)
		if !isNumber || f != float64(int(f)) {
			return refuse("Invalid token")
		}
		tokenVersion = int(f)
	}
	if tokenVersion != version {
		return refuse("Session revoked")
	}

	raw, present := claims["sid"]
	if !present {
		return true
	}
	sid, isString := raw.(string)
	if !isString || sid == "" {
		return refuse("Invalid token")
	}
	live, err := session.Find(db, userID, sid)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return refuse("Session revoked")
	}
	if err != nil {
		logger.Logger.Error().Err(err).Uint("user_id", userID).Msg("Failed to look up the session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		c.Abort()
		return false
	}
	// Writes only when last_seen_at is older than a minute.
	if err := session.Touch(db, live); err != nil {
		logger.Logger.Warn().Err(err).Msg("Failed to update session last_seen_at")
	}
	c.Set("sessionID", sid)
	return true
}
