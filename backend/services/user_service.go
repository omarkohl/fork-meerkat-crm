package services

import (
	"errors"
	"meerkat/config"
	"meerkat/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrPasswordTooLong = errors.New("password must not exceed 72 characters")

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password cannot be empty")
	}

	if len([]byte(password)) > 72 {
		return "", ErrPasswordTooLong
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// GenerateToken signs a JWT without a session (no "sid" claim). Logins use IssueSession.
func GenerateToken(user models.User, cfg *config.Config) (string, error) {
	return signToken(user, cfg, "")
}

func signToken(user models.User, cfg *config.Config, sessionID string) (string, error) {
	JWTSecretKey := cfg.JWTSecretKey
	if JWTSecretKey == "" {
		return "", errors.New("JWT secret key is empty")
	}

	JWTExpiryHours := cfg.JWTExpiryHours
	if JWTExpiryHours <= 0 {
		return "", errors.New("JWT expiry hours is invalid")
	}

	// Note: is_admin is intentionally NOT included in the JWT (AdminMiddleware handles this)
	claims := jwt.MapClaims{
		"authorized": true,
		"username":   user.Username,
		"user_id":    user.ID,
		"exp":        time.Now().Add(time.Hour * time.Duration(JWTExpiryHours)).Unix(),
		// Bumping users.token_version invalidates every token issued before.
		"tv": user.TokenVersion,
	}
	if sessionID != "" {
		claims["sid"] = sessionID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(JWTSecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
