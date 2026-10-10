package controllers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"meerkat/i18n"
	"meerkat/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// icsEnv wires the management routes (as user A) and the public feed route (no
// user in the context, as in production) onto separate engines sharing one DB.
type icsEnv struct {
	db      *gorm.DB
	userA   models.User
	userB   models.User
	manageA *gin.Engine
	manageB *gin.Engine
	public  *gin.Engine
}

func newICSEnv(t *testing.T) *icsEnv {
	t.Helper()
	require.NoError(t, i18n.Init())
	db, routerA := setupRouter()
	require.NoError(t, db.AutoMigrate(&models.IcsFeed{}))

	var a models.User
	db.First(&a)
	b := models.User{Username: "other", Email: "other@example.com", Password: "x"}
	require.NoError(t, db.Create(&b).Error)

	routerA.GET("/ics-feed", GetIcsFeed)
	routerA.POST("/ics-feed", CreateIcsFeed)
	routerA.DELETE("/ics-feed", DeleteIcsFeed)

	routerB := gin.New()
	routerB.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("userID", b.ID)
		c.Next()
	})
	routerB.GET("/ics-feed", GetIcsFeed)
	routerB.POST("/ics-feed", CreateIcsFeed)
	routerB.DELETE("/ics-feed", DeleteIcsFeed)

	pub := gin.New()
	pub.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	pub.GET("/ics/:file", ServeIcsFeed)

	return &icsEnv{db: db, userA: a, userB: b, manageA: routerA, manageB: routerB, public: pub}
}

func doReq(r *gin.Engine, method, path string, hdr ...string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func createFeed(t *testing.T, r *gin.Engine) models.IcsFeedCreateResponse {
	t.Helper()
	w := doReq(r, "POST", "/ics-feed")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp models.IcsFeedCreateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func TestIcsFeedStatusInitiallyDisabled(t *testing.T) {
	env := newICSEnv(t)
	w := doReq(env.manageA, "GET", "/ics-feed")
	assert.Equal(t, http.StatusOK, w.Code)
	var st models.IcsFeedStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &st))
	assert.False(t, st.Enabled)
	assert.Nil(t, st.CreatedAt)
}

func TestCreateIcsFeedReturnsTokenOnceAndStoresOnlyHash(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageA)

	assert.True(t, resp.Enabled)
	assert.NotNil(t, resp.CreatedAt)
	assert.Len(t, resp.Token, 43)

	var feed models.IcsFeed
	require.NoError(t, env.db.Where("user_id = ?", env.userA.ID).First(&feed).Error)
	assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(resp.Token))), feed.TokenHash)
	assert.NotContains(t, feed.TokenHash, resp.Token)

	// Status never reveals the token.
	w := doReq(env.manageA, "GET", "/ics-feed")
	assert.NotContains(t, w.Body.String(), resp.Token)
	assert.Contains(t, w.Body.String(), `"enabled":true`)
}

func TestIcsFeedValidTokenServesCalendar(t *testing.T) {
	env := newICSEnv(t)
	require.NoError(t, env.db.Create(&models.Contact{UserID: env.userA.ID, Firstname: "Ada", Birthday: "1815-12-10"}).Error)
	resp := createFeed(t, env.manageA)

	w := doReq(env.public, "GET", "/ics/"+resp.Token+".ics")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/calendar; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Cache-Control"), "private")
	assert.NotEmpty(t, w.Header().Get("ETag"))
	assert.Equal(t, "noindex", w.Header().Get("X-Robots-Tag"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.True(t, strings.HasPrefix(w.Body.String(), "BEGIN:VCALENDAR\r\n"))
	assert.Contains(t, w.Body.String(), "Ada")

	// Conditional request
	w2 := doReq(env.public, "GET", "/ics/"+resp.Token+".ics", "If-None-Match", w.Header().Get("ETag"))
	assert.Equal(t, http.StatusNotModified, w2.Code)
	assert.Empty(t, w2.Body.String())
}

func TestIcsFeedRejectsBadTokens(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageA)

	for name, path := range map[string]string{
		"unknown":        "/ics/" + strings.Repeat("A", 43) + ".ics",
		"too short":      "/ics/abc.ics",
		"bad charset":    "/ics/" + strings.Repeat("!", 43) + ".ics",
		"missing suffix": "/ics/" + resp.Token,
		"wrong suffix":   "/ics/" + resp.Token + ".txt",
		"tampered":       "/ics/" + resp.Token[:42] + flip(resp.Token[42]) + ".ics",
	} {
		w := doReq(env.public, "GET", path)
		assert.Equal(t, http.StatusNotFound, w.Code, name)
		assert.NotContains(t, w.Body.String(), "BEGIN:VCALENDAR", name)
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"), name)
	}
}

func flip(b byte) string {
	if b == 'A' {
		return "B"
	}
	return "A"
}

func TestIcsFeedIgnoresSessionUserInContext(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageA)
	// A logged-in user B hitting A's URL still gets A's feed, never B's data.
	require.NoError(t, env.db.Create(&models.Contact{UserID: env.userB.ID, Firstname: "Bertrand", Birthday: "1991-03-04"}).Error)
	require.NoError(t, env.db.Create(&models.Contact{UserID: env.userA.ID, Firstname: "Alice", Birthday: "1990-01-02"}).Error)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("db", env.db)
		c.Set("userID", env.userB.ID)
		c.Next()
	})
	engine.GET("/ics/:file", ServeIcsFeed)
	w := doReq(engine, "GET", "/ics/"+resp.Token+".ics")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Alice")
	assert.NotContains(t, w.Body.String(), "Bertrand")
}

func TestIcsFeedUserIsolation(t *testing.T) {
	env := newICSEnv(t)
	ca := models.Contact{UserID: env.userA.ID, Firstname: "Alice", Birthday: "1990-01-02"}
	cb := models.Contact{UserID: env.userB.ID, Firstname: "Bertrand", Birthday: "1991-03-04"}
	require.NoError(t, env.db.Create(&ca).Error)
	require.NoError(t, env.db.Create(&cb).Error)
	require.NoError(t, env.db.Create(&models.Reminder{UserID: env.userA.ID, ContactID: &ca.ID, Message: "reminder-of-a", RemindAt: time.Now(), Recurrence: "once"}).Error)
	require.NoError(t, env.db.Create(&models.Reminder{UserID: env.userB.ID, ContactID: &cb.ID, Message: "reminder-of-b", RemindAt: time.Now(), Recurrence: "once"}).Error)

	tokA := createFeed(t, env.manageA).Token
	tokB := createFeed(t, env.manageB).Token
	assert.NotEqual(t, tokA, tokB)

	wa := doReq(env.public, "GET", "/ics/"+tokA+".ics")
	wb := doReq(env.public, "GET", "/ics/"+tokB+".ics")
	require.Equal(t, http.StatusOK, wa.Code)
	require.Equal(t, http.StatusOK, wb.Code)
	assert.Contains(t, wa.Body.String(), "Alice")
	assert.Contains(t, wa.Body.String(), "reminder-of-a")
	assert.NotContains(t, wa.Body.String(), "Bertrand")
	assert.NotContains(t, wa.Body.String(), "reminder-of-b")
	assert.Contains(t, wb.Body.String(), "Bertrand")
	assert.NotContains(t, wb.Body.String(), "Alice")
	assert.NotContains(t, wb.Body.String(), "reminder-of-a")

	// B revoking does not touch A's feed.
	assert.Equal(t, http.StatusOK, doReq(env.manageB, "DELETE", "/ics-feed").Code)
	assert.Equal(t, http.StatusNotFound, doReq(env.public, "GET", "/ics/"+tokB+".ics").Code)
	assert.Equal(t, http.StatusOK, doReq(env.public, "GET", "/ics/"+tokA+".ics").Code)
}

func TestIcsFeedRegenerateInvalidatesOldToken(t *testing.T) {
	env := newICSEnv(t)
	first := createFeed(t, env.manageA)
	second := createFeed(t, env.manageA)
	assert.NotEqual(t, first.Token, second.Token)

	assert.Equal(t, http.StatusNotFound, doReq(env.public, "GET", "/ics/"+first.Token+".ics").Code)
	assert.Equal(t, http.StatusOK, doReq(env.public, "GET", "/ics/"+second.Token+".ics").Code)

	var n int64
	env.db.Model(&models.IcsFeed{}).Where("user_id = ?", env.userA.ID).Count(&n)
	assert.EqualValues(t, 1, n)
}

func TestIcsFeedRevokedTokenFails(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageA)
	assert.Equal(t, http.StatusOK, doReq(env.public, "GET", "/ics/"+resp.Token+".ics").Code)

	w := doReq(env.manageA, "DELETE", "/ics-feed")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, http.StatusNotFound, doReq(env.public, "GET", "/ics/"+resp.Token+".ics").Code)

	// Idempotent, status disabled again
	assert.Equal(t, http.StatusOK, doReq(env.manageA, "DELETE", "/ics-feed").Code)
	var st models.IcsFeedStatus
	require.NoError(t, json.Unmarshal(doReq(env.manageA, "GET", "/ics-feed").Body.Bytes(), &st))
	assert.False(t, st.Enabled)
}

func TestIcsFeedContentDoesNotLeakIntoEscapingContext(t *testing.T) {
	env := newICSEnv(t)
	c := models.Contact{UserID: env.userA.ID, Firstname: "Eve", Birthday: "1990-01-02"}
	require.NoError(t, env.db.Create(&c).Error)
	require.NoError(t, env.db.Create(&models.Reminder{UserID: env.userA.ID, ContactID: &c.ID, Message: "x\r\nBEGIN:VEVENT\r\nSUMMARY:injected", RemindAt: time.Now(), Recurrence: "once"}).Error)
	resp := createFeed(t, env.manageA)
	w := doReq(env.public, "GET", "/ics/"+resp.Token+".ics")
	events := 0
	for _, l := range strings.Split(w.Body.String(), "\r\n") {
		if l == "BEGIN:VEVENT" {
			events++
		}
		assert.NotEqual(t, "SUMMARY:injected", l)
	}
	assert.Equal(t, 2, events, "injection must be escaped")
}

func TestIcsFeedServeReturns404WhenOwnerIsGone(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageB)
	require.Equal(t, http.StatusOK, doReq(env.public, "GET", "/ics/"+resp.Token+".ics").Code)

	// Users are soft-deleted, so the foreign key cascade never removes the feed row.
	require.NoError(t, env.db.Delete(&env.userB).Error)
	assert.Equal(t, http.StatusNotFound, doReq(env.public, "GET", "/ics/"+resp.Token+".ics").Code)
}

func TestAdminDeleteUserRemovesIcsFeed(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageB)

	env.manageA.DELETE("/users/:id", DeleteUser)
	w := doReq(env.manageA, "DELETE", fmt.Sprintf("/users/%d", env.userB.ID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var n int64
	env.db.Model(&models.IcsFeed{}).Where("user_id = ?", env.userB.ID).Count(&n)
	assert.EqualValues(t, 0, n)
	assert.Equal(t, http.StatusNotFound, doReq(env.public, "GET", "/ics/"+resp.Token+".ics").Code)
}

func TestCreateIcsFeedRetriesOnUniqueConflict(t *testing.T) {
	env := newICSEnv(t)
	// Simulate a concurrent request that inserts the user's row between our delete
	// and insert: the first insert fails on the unique constraint.
	first := true
	require.NoError(t, env.db.Callback().Create().Before("gorm:create").Register("test:ics-conflict", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*models.IcsFeed); ok && first {
			first = false
			tx.AddError(fmt.Errorf("UNIQUE constraint failed: ics_feeds.user_id"))
		}
	}))

	resp := createFeed(t, env.manageA)
	assert.False(t, first, "conflict path was exercised")
	assert.Equal(t, http.StatusOK, doReq(env.public, "GET", "/ics/"+resp.Token+".ics").Code)
}

func feedStatus(t *testing.T, r *gin.Engine) models.IcsFeedStatus {
	t.Helper()
	w := doReq(r, "GET", "/ics-feed")
	require.Equal(t, http.StatusOK, w.Code)
	var st models.IcsFeedStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &st))
	return st
}

func storedLastFetched(t *testing.T, env *icsEnv, userID uint) *time.Time {
	t.Helper()
	var feed models.IcsFeed
	require.NoError(t, env.db.Where("user_id = ?", userID).First(&feed).Error)
	return feed.LastFetchedAt
}

func TestIcsFeedLastFetchedIsNullUntilFetched(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageA)
	assert.Nil(t, resp.LastFetchedAt)
	assert.Nil(t, feedStatus(t, env.manageA).LastFetchedAt)
	assert.Contains(t, doReq(env.manageA, "GET", "/ics-feed").Body.String(), `"last_fetched_at":null`)
}

func TestIcsFeedFetchRecordsLastFetchedThrottled(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageA)
	url := "/ics/" + resp.Token + ".ics"

	before := time.Now().Add(-time.Second)
	require.Equal(t, http.StatusOK, doReq(env.public, "GET", url).Code)
	first := storedLastFetched(t, env, env.userA.ID)
	require.NotNil(t, first)
	assert.True(t, first.After(before))
	st := feedStatus(t, env.manageA)
	require.NotNil(t, st.LastFetchedAt)
	assert.WithinDuration(t, *first, *st.LastFetchedAt, time.Second)

	// A second fetch within a minute does not write again.
	require.Equal(t, http.StatusOK, doReq(env.public, "GET", url).Code)
	again := storedLastFetched(t, env, env.userA.ID)
	assert.True(t, first.Equal(*again), "timestamp must not move inside the throttle window")

	// Older than a minute: updated (also for conditional requests answered 304).
	old := time.Now().Add(-2 * time.Minute)
	require.NoError(t, env.db.Model(&models.IcsFeed{}).Where("user_id = ?", env.userA.ID).Update("last_fetched_at", old).Error)
	etag := doReq(env.public, "GET", url).Header().Get("ETag")
	updated := storedLastFetched(t, env, env.userA.ID)
	require.NotNil(t, updated)
	assert.True(t, updated.After(old.Add(time.Minute)))

	require.NoError(t, env.db.Model(&models.IcsFeed{}).Where("user_id = ?", env.userA.ID).Update("last_fetched_at", old).Error)
	require.Equal(t, http.StatusNotModified, doReq(env.public, "GET", url, "If-None-Match", etag).Code)
	assert.True(t, storedLastFetched(t, env, env.userA.ID).After(old.Add(time.Minute)))
}

func TestIcsFeedFailedFetchDoesNotRecord(t *testing.T) {
	env := newICSEnv(t)
	resp := createFeed(t, env.manageA)
	url := "/ics/" + resp.Token + ".ics"

	// Failed build: the calendar is not served, so no fetch is recorded.
	fail := true
	require.NoError(t, env.db.Callback().Query().Before("gorm:query").Register("test:ics-build-fail", func(tx *gorm.DB) {
		if fail && tx.Statement.Table == "contacts" {
			tx.AddError(fmt.Errorf("boom"))
		}
	}))
	assert.Equal(t, http.StatusInternalServerError, doReq(env.public, "GET", url).Code)
	assert.Nil(t, storedLastFetched(t, env, env.userA.ID))

	// Owner gone: 404, nothing recorded.
	fail = false
	require.NoError(t, env.db.Delete(&env.userA).Error)
	assert.Equal(t, http.StatusNotFound, doReq(env.public, "GET", url).Code)
	assert.Nil(t, storedLastFetched(t, env, env.userA.ID))
}

func TestIcsFeedLastFetchedIsPerUserAndResetOnRegenerate(t *testing.T) {
	env := newICSEnv(t)
	a := createFeed(t, env.manageA)
	createFeed(t, env.manageB)

	require.Equal(t, http.StatusOK, doReq(env.public, "GET", "/ics/"+a.Token+".ics").Code)
	assert.NotNil(t, feedStatus(t, env.manageA).LastFetchedAt)
	assert.Nil(t, feedStatus(t, env.manageB).LastFetchedAt, "another user's fetch must not show")

	// A regenerated URL has not been fetched yet.
	createFeed(t, env.manageA)
	assert.Nil(t, feedStatus(t, env.manageA).LastFetchedAt)
}
