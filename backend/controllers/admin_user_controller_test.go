package controllers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"meerkat/config"
	"meerkat/models"
	"meerkat/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteUser_RemovesTokensAndFeed(t *testing.T) {
	db, router := setupRouter()
	require.NoError(t, db.AutoMigrate(&models.ApiToken{}, &models.IcsFeed{}, &models.CardDAVConnection{}, &models.CalendarSubscription{}, &models.CardDAVContactLink{}, &models.CalendarEventLink{}, models.Session{}, models.AuditLog{}, models.RecoveryCode{}))

	var admin models.User
	require.NoError(t, db.First(&admin).Error)
	require.NoError(t, db.Model(&admin).Update("is_admin", true).Error)

	victim := models.User{Username: "victim", Email: "victim@example.com", Password: "x"}
	bystander := models.User{Username: "bystander", Email: "by@example.com", Password: "x"}
	require.NoError(t, db.Create(&victim).Error)
	require.NoError(t, db.Create(&bystander).Error)

	for _, u := range []models.User{victim, bystander} {
		require.NoError(t, db.Create(&models.ApiToken{UserID: u.ID, Name: "t", TokenHash: "h" + u.Username}).Error)
		require.NoError(t, db.Create(&models.IcsFeed{UserID: u.ID, TokenHash: "f" + u.Username, CreatedAt: time.Now()}).Error)
		require.NoError(t, db.Create(&models.CardDAVConnection{UserID: u.ID}).Error)
		require.NoError(t, db.Create(&models.CalendarSubscription{UserID: u.ID}).Error)
		wh := models.Webhook{UserID: u.ID}
		require.NoError(t, db.Create(&wh).Error)
		require.NoError(t, db.Create(&models.WebhookDelivery{WebhookID: wh.ID}).Error)
		uid := u.ID
		require.NoError(t, db.Create(&models.EmailOutbox{UserID: &uid, ToAddr: u.Email, Subject: "s", HTML: "h", Status: models.OutboxPending, NextAttemptAt: time.Now()}).Error)
	}
	var victimWebhook models.Webhook
	require.NoError(t, db.Where("user_id = ?", victim.ID).First(&victimWebhook).Error)

	router.DELETE("/admin/users/:id", DeleteUser)
	req, _ := http.NewRequest("DELETE", "/admin/users/"+strconv.FormatUint(uint64(victim.ID), 10), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var n int64
	db.Unscoped().Model(&models.ApiToken{}).Where("user_id = ?", victim.ID).Count(&n)
	assert.Zero(t, n, "the deleted user's API tokens must be gone")
	db.Model(&models.IcsFeed{}).Where("user_id = ?", victim.ID).Count(&n)
	assert.Zero(t, n, "the deleted user's calendar feed must be gone")

	for _, m := range []any{&models.CardDAVConnection{}, &models.CalendarSubscription{}, &models.Webhook{}} {
		db.Unscoped().Model(m).Where("user_id = ?", victim.ID).Count(&n)
		assert.Zero(t, n, "%T must be gone", m)
	}
	db.Unscoped().Model(&models.WebhookDelivery{}).Where("webhook_id = ?", victimWebhook.ID).Count(&n)
	assert.Zero(t, n, "webhook deliveries must be gone")
	db.Model(&models.EmailOutbox{}).Where("user_id = ?", victim.ID).Count(&n)
	assert.Zero(t, n, "queued emails of the deleted user must be gone")
	db.Model(&models.EmailOutbox{}).Where("user_id = ?", bystander.ID).Count(&n)
	assert.Equal(t, int64(1), n, "other users' queued emails stay")

	// Other users keep theirs.
	db.Model(&models.ApiToken{}).Where("user_id = ?", bystander.ID).Count(&n)
	assert.Equal(t, int64(1), n)
	db.Model(&models.IcsFeed{}).Where("user_id = ?", bystander.ID).Count(&n)
	assert.Equal(t, int64(1), n)
	db.Model(&models.CardDAVConnection{}).Where("user_id = ?", bystander.ID).Count(&n)
	assert.Equal(t, int64(1), n)
	db.Model(&models.WebhookDelivery{}).Count(&n)
	assert.Equal(t, int64(1), n, "only the bystander's delivery remains")
}

// A deleted OIDC user keeps a soft-deleted row; its email, username and OIDC identity
// must be released so the same person can log in again and gets a new account.
func TestDeleteUser_ReleasesOIDCIdentity(t *testing.T) {
	db, router := setupRouter()
	require.NoError(t, db.AutoMigrate(&models.ApiToken{}, &models.IcsFeed{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_users_oidc_subject ON users(oidc_subject, oidc_provider) WHERE oidc_subject IS NOT NULL").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_users_username_t ON users(username)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_users_email_t ON users(email)").Error)

	var admin models.User
	require.NoError(t, db.First(&admin).Error)
	require.NoError(t, db.Model(&admin).Update("is_admin", true).Error)

	cfg := &config.Config{}
	cfg.OIDC.AllowAutoProvision = true
	claims := &services.OIDCClaims{Subject: "sub-1", Provider: "https://idp", Email: "bob@example.com", EmailVerified: true, PreferredUsername: "bob"}
	first, err := services.FindOrProvisionUser(db, claims, cfg)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.Contact{UserID: first.ID, Firstname: "Old"}).Error)

	router.DELETE("/admin/users/:id", DeleteUser)
	req, _ := http.NewRequest("DELETE", "/admin/users/"+strconv.FormatUint(uint64(first.ID), 10), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	second, err := services.FindOrProvisionUser(db, claims, cfg)
	require.NoError(t, err, "logging in again must create a new account")
	assert.NotEqual(t, first.ID, second.ID)
	assert.Equal(t, "bob@example.com", second.Email)

	var n int64
	db.Model(&models.Contact{}).Where("user_id = ?", first.ID).Count(&n)
	assert.Zero(t, n, "the old data stays deleted")
	db.Model(&models.User{}).Where("id = ?", first.ID).Count(&n)
	assert.Zero(t, n, "the old account stays soft-deleted")
}
