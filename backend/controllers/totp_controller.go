package controllers

import (
	"errors"
	"fmt"
	"meerkat/clock"
	"net/http"
	"time"

	apperrors "meerkat/errors"
	"meerkat/idutil"
	"meerkat/logger"
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
	// factorLocked: the account was already locked; the 429 answer is written.
	factorLocked
	// factorWrongNowLocked: this wrong attempt triggered the lockout; nothing is written yet.
	factorWrongNowLocked
)

func respondLocked(c *gin.Context, remainingSecs int) {
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error":          "Account temporarily locked",
		"message":        "Too many failed attempts. Please try again later.",
		"retry_after":    remainingSecs,
		"retry_after_at": totpNow().Add(time.Duration(remainingSecs) * time.Second).Format(time.RFC3339),
	})
	c.Abort()
}

// rejectIfTOTPLocked answers 429 and returns true when the account's second factor is locked.
func rejectIfTOTPLocked(c *gin.Context, userID uint) bool {
	if locked, secs := middleware.GetAccountRateLimiter().IsLocked(totpLimiterKey(userID)); locked {
		respondLocked(c, secs)
		return true
	}
	return false
}

// verifySecondFactor checks a code or recovery code under the per-account lockout. On
// factorLocked the 429 answer is already written; on factorWrong and factorWrongNowLocked
// (secs is the lockout length) the caller answers.
func verifySecondFactor(c *gin.Context, db *gorm.DB, user models.User, code, recovery string) (factorResult, int) {
	cfg := currentConfig(c)
	limiter := middleware.GetAccountRateLimiter()
	key := totpLimiterKey(user.ID)
	if rejectIfTOTPLocked(c, user.ID) {
		return factorLocked, 0
	}
	ok, usedRecovery := services.CheckSecondFactor(db, &cfg, user, code, recovery, totpNow())
	if ok {
		limiter.RecordSuccessfulLogin(key)
		if usedRecovery {
			auditFor(c, db, &user.ID, models.AuditTOTPRecoveryUsed, "")
		}
		return factorOK, 0
	}
	if locked, secs := limiter.RecordFailedAttempt(key); locked {
		return factorWrongNowLocked, secs
	}
	return factorWrong, 0
}

func refuseAPITokenFor2FA(c *gin.Context) bool {
	if c.GetBool("isAPIToken") {
		apperrors.AbortWithError(c, apperrors.ErrForbidden("API tokens cannot manage two-factor authentication"))
		return true
	}
	return false
}

// refuseWithoutPassword: accounts that only sign in through single sign-on have no password
// step to protect and could not confirm changes; the identity provider is their second factor.
func refuseWithoutPassword(c *gin.Context, user models.User) bool {
	if user.Password == "" {
		apperrors.AbortWithError(c, apperrors.ErrInvalidInput("", "Two-factor authentication is not available for single sign-on accounts"))
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
	c.JSON(http.StatusOK, gin.H{
		"enabled":                  user.TOTPEnabled,
		"recovery_codes_remaining": remaining,
		// available is false for accounts without a password (single sign-on only)
		"available": user.Password != "",
	})
}

// SetupTOTP creates a pending secret and returns it with the otpauth URI for the QR code.
// Two-factor stays off until EnableTOTP proves the app works.
func SetupTOTP(c *gin.Context) {
	db, user, ok := loadCurrentUser(c)
	if !ok || refuseWithoutPassword(c, user) {
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

// EnableTOTP turns two-factor on after the password and a valid code, and returns the
// recovery codes, which are shown this once. Other sessions are signed out.
func EnableTOTP(c *gin.Context) {
	db, user, ok := loadCurrentUser(c)
	if !ok || refuseWithoutPassword(c, user) {
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
	if rejectIfTOTPLocked(c, user.ID) {
		return
	}
	if !checkPasswordCounted(c, user, input.Password) {
		return
	}
	cfg := currentConfig(c)
	codes, err := services.EnableTOTP(db, &cfg, user.ID, input.Code, totpNow())
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
	endOtherSessions(c, db, user.ID)
	audit(c, db, models.AuditTOTPEnabled, "")
	c.JSON(http.StatusOK, gin.H{"recovery_codes": codes})
}

// endOtherSessions signs out every browser but the one making the change: a login that
// happened before the second step existed (or that is about to lose it) should not linger.
func endOtherSessions(c *gin.Context, db *gorm.DB, userID uint) {
	if err := services.RevokeOtherSessions(db, userID, c.GetString("sessionID"), middleware.RequestNow(c)); err != nil {
		logger.FromContext(c).Error().Err(err).Uint("user_id", userID).Msg("Failed to revoke other sessions after a two-factor change")
	}
}

// checkPasswordCounted verifies the password; a wrong one counts on the account's
// second-factor lockout, so the dialogs cannot be used to guess it. It answers 400 or 429.
func checkPasswordCounted(c *gin.Context, user models.User, password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		if locked, secs := middleware.GetAccountRateLimiter().RecordFailedAttempt(totpLimiterKey(user.ID)); locked {
			respondLocked(c, secs)
		} else {
			apperrors.AbortWithError(c, apperrors.ErrInvalidInput("password", "Password is incorrect"))
		}
		return false
	}
	return true
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
	if rejectIfTOTPLocked(c, user.ID) || !checkPasswordCounted(c, user, input.Password) {
		return nil, false
	}
	switch result, secs := verifySecondFactor(c, db, user, input.Code, input.RecoveryCode); result {
	case factorLocked:
		return nil, false
	case factorWrongNowLocked:
		respondLocked(c, secs)
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
	endOtherSessions(c, db, user.ID)
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
	cfgValue := currentConfig(c)
	codes, err := services.RegenerateRecoveryCodes(db, &cfgValue, user.ID)
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
	if !user.TOTPEnabled {
		apperrors.AbortWithError(c, apperrors.ErrConflict("Two-factor authentication is not enabled for this user"))
		return
	}
	if err := services.DisableTOTP(db, user.ID); err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}
	if err := services.RevokeAllSessions(db, user.ID, middleware.RequestNow(c)); err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}
	middleware.GetAccountRateLimiter().RecordSuccessfulLogin(totpLimiterKey(user.ID))
	audit(c, db, models.AuditTOTPAdminReset, fmt.Sprintf("user:%s#%d", auditValue(user.Username), user.ID))
	c.JSON(http.StatusOK, gin.H{"message": "Two-factor authentication reset"})
}

// totpNow is the time codes are checked against. Authenticator apps compute them from the
// real time, so FAKE_NOW must not move it or two-factor users would be locked out.
func totpNow() time.Time { return clock.Real().Now() }
