package controllers

import (
	"net/http"
	"strings"

	apperrors "meerkat/errors"
	"meerkat/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ListAuditLog returns audit entries, newest first (admin only). Filters: event (exact),
// username (substring, case-insensitive). Paginated like the other lists.
func ListAuditLog(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	p := GetPaginationParams(c)

	query := db.Model(&models.AuditLog{})
	if event := c.Query("event"); event != "" {
		query = query.Where("event = ?", event)
	}
	if name := strings.ToLower(strings.TrimSpace(c.Query("username"))); name != "" {
		query = query.Where("instr(lower(username), ?) > 0", name)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("count").WithError(err))
		return
	}
	entries := []models.AuditLog{}
	if err := query.Order("id DESC").Limit(p.Limit).Offset(p.Offset).Find(&entries).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("query").WithError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"entries":        entries,
		"total":          total,
		"page":           p.Page,
		"limit":          p.Limit,
		"events":         models.AuditEvents,
		"retention_days": currentConfig(c).AuditLogRetentionDays,
	})
}
