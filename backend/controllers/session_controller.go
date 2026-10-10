package controllers

import (
	"net/http"
	"regexp"

	apperrors "meerkat/errors"
	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var sessionIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func clearAuthCookie(c *gin.Context) {
	cfg := currentConfig(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("auth_token", "", -1, "/", cfg.CookieDomain, cfg.CookieSecure, true)
}

// refuseAPIToken answers 403 for API tokens: sessions belong to browser logins, and a
// token must not be able to sign the owner out.
func refuseAPIToken(c *gin.Context) bool {
	if c.GetBool("isAPIToken") {
		apperrors.AbortWithError(c, apperrors.ErrForbidden("API tokens cannot manage sessions"))
		return true
	}
	return false
}

// ListSessions returns the user's live sessions, newest first; the one making the request is
// marked current.
func ListSessions(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok || refuseAPIToken(c) {
		return
	}
	sessions, err := services.ListSessions(db, userID)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("query").WithError(err))
		return
	}
	current := c.GetString("sessionID")
	out := make([]models.SessionResponse, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, models.SessionResponse{Session: s, Current: current != "" && s.PublicID == current})
	}
	c.JSON(http.StatusOK, gin.H{"sessions": out})
}

// RevokeSession signs one session out. Revoking the current one is a logout.
func RevokeSession(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok || refuseAPIToken(c) {
		return
	}
	id := c.Param("id")
	if !sessionIDPattern.MatchString(id) {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Session"))
		return
	}
	revoked, err := services.RevokeSession(db, userID, id)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}
	if !revoked {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Session"))
		return
	}
	audit(c, db, models.AuditSessionRevoked, "")
	if id == c.GetString("sessionID") {
		clearAuthCookie(c)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Session revoked"})
}

// SignOutEverywhere revokes every session of the user, this one included, and raises the
// token version so older tokens die too.
func SignOutEverywhere(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok || refuseAPIToken(c) {
		return
	}
	if err := services.RevokeAllSessions(db, userID); err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}
	audit(c, db, models.AuditSignOutEverywhere, "")
	clearAuthCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "Signed out everywhere"})
}
