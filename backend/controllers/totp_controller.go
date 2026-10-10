package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	apperrors "meerkat/errors"
	"meerkat/idutil"
	"meerkat/middleware"
	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const totpIssuer = "Meerkat CRM"

// totpLimiterKey names the failed-attempt counter of an account's second factor. It is by
// account, not by what was typed at login, so changing the identifier does not reset it.
func totpLimiterKey(userID uint) string { return fmt.Sprintf("2fa:%d", userID) }

type factorResult int

const (
	factorOK factorResult = iota
	factorWrong
	factorLocked
)

func respondLocked(c *gin.Context, remainingSecs int) {
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error":          "Account temporarily locked",
		"message":        "Too many failed attempts. Please try again later.",
		"retry_after":    remainingSecs,
		"retry_after_at": time.Now().Add(time.Duration(remainingSecs) * time.Second).Format(time.RFC3339),
	})
	c.Abort()
}

// verifySecondFactor checks a code or recovery code under the per-account lockout. On
// factorLocked the 429 answer is already written; on factorWrong the caller answers.
func verifySecondFactor(c *gin.Context, db *gorm.DB, user models.User, code, recovery string) factorResult {
	cfg := currentConfig(c)
	limiter := middleware.GetAccountRateLimiter()
	key := totpLimiterKey(user.ID)
	if locked, secs := limiter.IsLocked(key); locked {
		respondLocked(c, secs)
		return factorLocked
	}
	ok, usedRecovery := services.CheckSecondFactor(db, &cfg, user, code, recovery, time.Now())
	if ok {
		limiter.RecordSuccessfulLogin(key)
		if usedRecovery {
			auditFor(c, db, &user.ID, models.AuditTOTPRecoveryUsed, "")
		}
		return factorOK
	}
	if locked, secs := limiter.RecordFailedAttempt(key); locked {
		respondLocked(c, secs)
		return factorLocked
	}
	return factorWrong
}

func refuseAPITokenFor2FA(c *gin.Context) bool {
	if c.GetBool("isAPIToken") {
		apperrors.AbortWithError(c, apperrors.ErrForbidden("API tokens cannot manage two-factor authentication"))
		return true
	}
	return false
}

func loadCurrentUser(c *gin.Context) (*gorm.DB, models.User, bool) {
	db := c.MustGet("db").(*gorm.DB)
	var user models.User
	userID, ok := currentUserID(c)
	if !ok || refuseAPITokenFor2FA(c) {
		return db, user, false
	}
	if err := db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("User"))
		} else {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("query user").WithError(err))
		}
		return db, user, false
	}
	return db, user, true
}

// GetTOTPStatus says whether two-factor is on and how many recovery codes are left.
func GetTOTPStatus(c *gin.Context) {
	db, user, ok := loadCurrentUser(c)
	if !ok {
		return
	}
	remaining := 0
	if user.TOTPEnabled {
		remaining = services.RecoveryCodesRemaining(db, user.ID)
	}
	c.JSON(http.StatusOK, gin.H{"enabled": user.TOTPEnabled, "recovery_codes_remaining": remaining})
}

// SetupTOTP creates a pending secret and returns it with the otpauth URI for the QR code.
// Two-factor stays off until EnableTOTP proves the app works.
func SetupTOTP(c *gin.Context) {
	db, user, ok := loadCurrentUser(c)
	if !ok {
		return
	}
	cfg := currentConfig(c)
	secret, err := services.BeginTOTPSetup(db, &cfg, user.ID)
	if errors.Is(err, services.ErrTOTPAlreadyEnabled) {
		apperrors.AbortWithError(c, apperrors.ErrConflict("Two-factor authentication is already enabled"))
		return
	}
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrInternal("Could not start two-factor setup").WithError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"secret":      secret,
		"otpauth_uri": services.TOTPURI(totpIssuer, user.Username, secret),
	})
}

// EnableTOTP turns two-factor on after a valid code and returns the recovery codes, which
// are shown this once.
func EnableTOTP(c *gin.Context) {
	db, user, ok := loadCurrentUser(c)
	if !ok {
		return
	}
	input, appErr := middleware.GetValidated[models.TOTPCodeInput](c)
	if appErr != nil {
		apperrors.AbortWithError(c, appErr)
		return
	}
	if user.TOTPEnabled {
		apperrors.AbortWithError(c, apperrors.ErrConflict("Two-factor authentication is already enabled"))
		return
	}
	limiter := middleware.GetAccountRateLimiter()
	key := totpLimiterKey(user.ID)
	if locked, secs := limiter.IsLocked(key); locked {
		respondLocked(c, secs)
		return
	}
	cfg := currentConfig(c)
	codes, err := services.EnableTOTP(db, &cfg, user.ID, input.Code, time.Now())
	switch {
	case errors.Is(err, services.ErrInvalidSecondFactor):
		if locked, secs := limiter.RecordFailedAttempt(key); locked {
			respondLocked(c, secs)
			return
		}
		apperrors.AbortWithError(c, apperrors.ErrInvalidInput("code", "Invalid verification code"))
		return
	case errors.Is(err, services.ErrTOTPAlreadyEnabled):
		apperrors.AbortWithError(c, apperrors.ErrConflict("Two-factor authentication is already enabled"))
		return
	case err != nil:
		apperrors.AbortWithError(c, apperrors.ErrInternal("Could not enable two-factor authentication").WithError(err))
		return
	}
	limiter.RecordSuccessfulLogin(key)
	audit(c, db, models.AuditTOTPEnabled, "")
	c.JSON(http.StatusOK, gin.H{"recovery_codes": codes})
}

// confirmSensitiveChange re-authenticates: current password plus a second factor.
func confirmSensitiveChange(c *gin.Context, db *gorm.DB, user models.User) (*models.TOTPConfirmInput, bool) {
	input, appErr := middleware.GetValidated[models.TOTPConfirmInput](c)
	if appErr != nil {
		apperrors.AbortWithError(c, appErr)
		return nil, false
	}
	if !user.TOTPEnabled {
		apperrors.AbortWithError(c, apperrors.ErrInvalidInput("", "Two-factor authentication is not enabled"))
		return nil, false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		apperrors.AbortWithError(c, apperrors.ErrInvalidInput("password", "Password is incorrect"))
		return nil, false
	}
	switch verifySecondFactor(c, db, user, input.Code, input.RecoveryCode) {
	case factorLocked:
		return nil, false
	case factorWrong:
		apperrors.AbortWithError(c, apperrors.ErrInvalidInput("code", "Invalid verification code"))
		return nil, false
	}
	return input, true
}

// DisableTOTP turns two-factor off; it needs the password and a code or recovery code.
func DisableTOTP(c *gin.Context) {
	db, user, ok := loadCurrentUser(c)
	if !ok {
		return
	}
	if _, ok := confirmSensitiveChange(c, db, user); !ok {
		return
	}
	if err := services.DisableTOTP(db, user.ID); err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}
	audit(c, db, models.AuditTOTPDisabled, "")
	c.JSON(http.StatusOK, gin.H{"message": "Two-factor authentication disabled"})
}

// RegenerateRecoveryCodes replaces the recovery codes (the old ones stop working).
func RegenerateRecoveryCodes(c *gin.Context) {
	db, user, ok := loadCurrentUser(c)
	if !ok {
		return
	}
	if _, ok := confirmSensitiveChange(c, db, user); !ok {
		return
	}
	codes, err := services.RegenerateRecoveryCodes(db, user.ID)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}
	audit(c, db, models.AuditTOTPRecoveryRegenerated, "")
	c.JSON(http.StatusOK, gin.H{"recovery_codes": codes})
}

// ResetUserTOTP lets an admin switch off a user's two-factor (lost phone and recovery
// codes). The user's sessions end, since whoever asked for the reset is not verified here.
func ResetUserTOTP(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	id, err := idutil.Parse(c.Param("id"))
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrInvalidInput("id", "Invalid user ID"))
		return
	}
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("User"))
		} else {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("query user").WithError(err))
		}
		return
	}
	if err := services.DisableTOTP(db, user.ID); err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}
	if err := services.RevokeAllSessions(db, user.ID); err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}
	middleware.GetAccountRateLimiter().RecordSuccessfulLogin(totpLimiterKey(user.ID))
	audit(c, db, models.AuditTOTPAdminReset, "user "+user.Username)
	c.JSON(http.StatusOK, gin.H{"message": "Two-factor authentication reset"})
}
