package controllers

import (
	"encoding/json"
	"net/http"
	"testing"

	"meerkat/config"
	"meerkat/middleware"
	"meerkat/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemStatusIsAdminOnlyAndWarnsOnMultiUserInstances(t *testing.T) {
	e := newBackupEnv(t) // one admin and one plain user
	mk := func(uid uint, cfg config.Config) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("db", e.db)
			c.Set("userID", uid)
			c.Set("cfg", cfg)
			c.Next()
		})
		r.GET("/admin/system-status", middleware.AdminMiddleware(), SystemStatus)
		return r
	}
	var admin, user models.User
	require.NoError(t, e.db.Where("is_admin = ?", true).First(&admin).Error)
	require.NoError(t, e.db.Where("is_admin = ?", false).First(&user).Error)

	// Not an admin: refused, nothing leaks.
	w := e.do(mk(user.ID, config.Config{}), "GET", "/admin/system-status")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NotContains(t, w.Body.String(), "private_url")

	status := func(cfg config.Config) map[string]any {
		w := e.do(mk(admin.ID, cfg), "GET", "/admin/system-status")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var out map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out
	}
	out := status(config.Config{})
	assert.Equal(t, true, out["private_url_warning"])
	assert.Equal(t, "BLOCK_PRIVATE_URLS", out["setting"])

	// Fully blocked: no warning. One flag still off: warning.
	all := config.Config{WebhookBlockPrivateURLs: true, CalDAVBlockPrivateURLs: true, CardDAVBlockPrivateURLs: true, MonicaBlockPrivateURLs: true}
	assert.Equal(t, false, status(all)["private_url_warning"])
	all.MonicaBlockPrivateURLs = false
	assert.Equal(t, true, status(all)["private_url_warning"])

	// A single-user instance is not warned; deleted users do not count.
	require.NoError(t, e.db.Delete(&user).Error)
	out = status(config.Config{})
	assert.Equal(t, false, out["private_url_warning"])
	assert.EqualValues(t, 1, out["user_count"])
}

func TestPrivateURLWarning(t *testing.T) {
	e := newBackupEnv(t)
	warn, n, err := PrivateURLWarning(e.db, true)
	require.NoError(t, err)
	assert.True(t, warn)
	assert.EqualValues(t, 2, n)
	warn, _, _ = PrivateURLWarning(e.db, false)
	assert.False(t, warn)
}
