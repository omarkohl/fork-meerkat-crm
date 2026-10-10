package controllers

import (
	"net/http"
	"testing"

	"meerkat/config"
	"meerkat/middleware"
	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const testJWTSecret = "unit-test-secret"

func credentialRouter(t *testing.T) (*gorm.DB, *gin.Engine, models.User) {
	t.Helper()
	db, router := setupRouter()
	router.Use(func(c *gin.Context) { c.Set("cfg", config.Config{JWTSecretKey: testJWTSecret}); c.Next() })
	var me models.User
	require.NoError(t, db.First(&me).Error)
	router.POST("/calendars", middleware.ValidateJSONMiddleware(&models.CalendarSubscriptionInput{}), CreateCalendarSubscription)
	router.PUT("/calendars/:id", middleware.ValidateJSONMiddleware(&models.CalendarSubscriptionInput{}), UpdateCalendarSubscription)
	router.GET("/calendars", ListCalendarSubscriptions)
	router.DELETE("/calendars/:id", DeleteCalendarSubscription)
	router.PUT("/carddav/connection", middleware.ValidateJSONMiddleware(&models.CardDAVConnectionInput{}), SaveCardDAVConnection)
	router.GET("/carddav/connection", GetCardDAVConnection)
	router.DELETE("/carddav/connection", DeleteCardDAVConnection)
	return db, router, me
}

func TestCalendarSubscriptionPasswordIsEncryptedAndNeverReturned(t *testing.T) {
	db, router, me := credentialRouter(t)
	w := doJSON(router, "POST", "/calendars", map[string]any{"name": "Work", "url": "https://cal.example.com/a.ics", "username": "u", "password": "hunter2"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "hunter2")

	var sub models.CalendarSubscription
	require.NoError(t, db.Where("user_id = ?", me.ID).First(&sub).Error)
	assert.NotContains(t, sub.PasswordEncrypted, "hunter2")
	plain, err := services.DecryptCredential(testJWTSecret, sub.PasswordEncrypted)
	require.NoError(t, err)
	assert.Equal(t, "hunter2", plain)

	list := doJSON(router, "GET", "/calendars", nil)
	assert.NotContains(t, list.Body.String(), "hunter2")
	assert.NotContains(t, list.Body.String(), sub.PasswordEncrypted)
}

func TestCalendarSubscriptionRejectsOtherSchemesAndOtherUsersRows(t *testing.T) {
	db, router, _ := credentialRouter(t)
	for _, u := range []string{"file:///etc/passwd", "ftp://example.com/x.ics", "gopher://x"} {
		w := doJSON(router, "POST", "/calendars", map[string]any{"name": "x", "url": u})
		assert.Equal(t, http.StatusBadRequest, w.Code, u)
	}

	other := models.User{Username: "other", Email: "o@example.com", Password: "x"}
	require.NoError(t, db.Create(&other).Error)
	theirs := models.CalendarSubscription{UserID: other.ID, Name: "T", URL: "https://t.example.com/x.ics"}
	require.NoError(t, db.Create(&theirs).Error)
	assert.Equal(t, http.StatusNotFound, doJSON(router, "PUT", "/calendars/"+itoa(theirs.ID), map[string]any{"name": "mine now", "url": "https://evil.example.com/x.ics"}).Code)
	assert.Equal(t, http.StatusNotFound, doJSON(router, "DELETE", "/calendars/"+itoa(theirs.ID), nil).Code)
	var reloaded models.CalendarSubscription
	require.NoError(t, db.First(&reloaded, theirs.ID).Error)
	assert.Equal(t, "T", reloaded.Name)
}

// A stored password belongs to the server it was entered for. Pointing the subscription at another
// host without entering a password must not send the old one there.
func TestCalendarSubscriptionPasswordDoesNotFollowAUrlChangeToAnotherHost(t *testing.T) {
	db, router, me := credentialRouter(t)
	require.Equal(t, http.StatusCreated, doJSON(router, "POST", "/calendars",
		map[string]any{"name": "Work", "url": "https://cal.example.com/a.ics", "username": "u", "password": "hunter2"}).Code)
	var sub models.CalendarSubscription
	require.NoError(t, db.Where("user_id = ?", me.ID).First(&sub).Error)

	// Same host, other path: the password stays.
	require.Equal(t, http.StatusOK, doJSON(router, "PUT", "/calendars/"+itoa(sub.ID),
		map[string]any{"name": "Work", "url": "https://CAL.example.com/b.ics", "username": "u"}).Code)
	require.NoError(t, db.First(&sub, sub.ID).Error)
	assert.NotEmpty(t, sub.PasswordEncrypted)

	// Other host: it is dropped.
	require.Equal(t, http.StatusOK, doJSON(router, "PUT", "/calendars/"+itoa(sub.ID),
		map[string]any{"name": "Work", "url": "https://attacker.example.net/b.ics", "username": "u"}).Code)
	require.NoError(t, db.First(&sub, sub.ID).Error)
	assert.Empty(t, sub.PasswordEncrypted)
}

func TestCardDAVConnectionPasswordHandling(t *testing.T) {
	db, router, me := credentialRouter(t)
	body := map[string]any{"base_url": "https://user:inline@dav.example.com/dav", "address_book_path": "/books/a/", "username": "u", "password": "hunter2"}
	w := doJSON(router, "PUT", "/carddav/connection", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "hunter2")
	assert.NotContains(t, w.Body.String(), "inline", "credentials in the URL are dropped")

	var conn models.CardDAVConnection
	require.NoError(t, db.Where("user_id = ?", me.ID).First(&conn).Error)
	assert.NotContains(t, conn.BaseURL, "inline")
	plain, err := services.DecryptCredential(testJWTSecret, conn.PasswordEncrypted)
	require.NoError(t, err)
	assert.Equal(t, "hunter2", plain)

	got := doJSON(router, "GET", "/carddav/connection", nil)
	assert.Contains(t, got.Body.String(), `"has_password":true`)
	assert.NotContains(t, got.Body.String(), conn.PasswordEncrypted)

	// Relative address book paths and non-http schemes are refused.
	assert.Equal(t, http.StatusBadRequest, doJSON(router, "PUT", "/carddav/connection",
		map[string]any{"base_url": "https://dav.example.com", "address_book_path": "books/a"}).Code)
	assert.Equal(t, http.StatusBadRequest, doJSON(router, "PUT", "/carddav/connection",
		map[string]any{"base_url": "file:///etc", "address_book_path": "/x"}).Code)

	// Same server: keeping the stored password when none is sent.
	require.Equal(t, http.StatusOK, doJSON(router, "PUT", "/carddav/connection",
		map[string]any{"base_url": "https://dav.example.com/other", "address_book_path": "/books/b/", "username": "u"}).Code)
	require.NoError(t, db.Where("user_id = ?", me.ID).First(&conn).Error)
	assert.NotEmpty(t, conn.PasswordEncrypted)

	// Another server: the stored password is not carried along.
	require.Equal(t, http.StatusOK, doJSON(router, "PUT", "/carddav/connection",
		map[string]any{"base_url": "https://evil.example.net/dav", "address_book_path": "/books/b/", "username": "u"}).Code)
	require.NoError(t, db.Where("user_id = ?", me.ID).First(&conn).Error)
	assert.Empty(t, conn.PasswordEncrypted)
}

func TestCardDAVSwitchingAddressBookDropsLinks(t *testing.T) {
	db, router, me := credentialRouter(t)
	require.Equal(t, http.StatusCreated, doJSON(router, "PUT", "/carddav/connection",
		map[string]any{"base_url": "https://dav.example.com", "address_book_path": "/a/", "password": "p"}).Code)
	var conn models.CardDAVConnection
	require.NoError(t, db.Where("user_id = ?", me.ID).First(&conn).Error)
	require.NoError(t, db.Model(&conn).Update("sync_token", "tok").Error)
	require.NoError(t, db.Create(&models.CardDAVContactLink{ConnectionID: conn.ID, UserID: me.ID, ContactID: 1, RemoteUID: "u1", RemotePath: "/a/1.vcf"}).Error)

	require.Equal(t, http.StatusOK, doJSON(router, "PUT", "/carddav/connection",
		map[string]any{"base_url": "https://dav.example.com", "address_book_path": "/b/"}).Code)
	var n int64
	db.Model(&models.CardDAVContactLink{}).Where("connection_id = ?", conn.ID).Count(&n)
	assert.Zero(t, n)
	require.NoError(t, db.First(&conn, conn.ID).Error)
	assert.Empty(t, conn.SyncToken)
}
