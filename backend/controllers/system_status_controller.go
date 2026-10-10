package controllers

import (
	"net/http"

	apperrors "meerkat/errors"
	"meerkat/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PrivateURLWarning reports whether private addresses are reachable from outbound
// features (webhooks, calendar and contact sync, Monica import) on an instance with more
// than one user, where one user could point them at the server's internal network.
func PrivateURLWarning(db *gorm.DB, privateURLsAllowed bool) (warn bool, users int64, err error) {
	if err := db.Model(&models.User{}).Count(&users).Error; err != nil {
		return false, 0, err
	}
	return privateURLsAllowed && users > 1, users, nil
}

// SystemStatus returns operational warnings for admins (admin only).
func SystemStatus(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	cfg := currentConfig(c)
	warn, users, err := PrivateURLWarning(db, cfg.PrivateURLsAllowed())
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("count users").WithError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"private_urls_allowed": cfg.PrivateURLsAllowed(),
		"user_count":           users,
		"private_url_warning":  warn,
		"features":             cfg.PrivateURLFeatures(),
		"setting":              "BLOCK_PRIVATE_URLS",
	})
}
