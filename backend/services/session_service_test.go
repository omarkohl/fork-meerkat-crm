package services

import (
	"testing"
	"time"

	"meerkat/config"
	"meerkat/models"

	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func sessionTestDB(t *testing.T) (*gorm.DB, models.User, models.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Session{}, &models.AuditLog{}))
	a := models.User{Username: "a", Email: "a@example.com", Password: "x"}
	b := models.User{Username: "b", Email: "b@example.com", Password: "x"}
	require.NoError(t, db.Create(&a).Error)
	require.NoError(t, db.Create(&b).Error)
	return db, a, b
}

var sessionCfg = &config.Config{JWTSecretKey: "test-secret-key-for-sessions-0123456789", JWTExpiryHours: 24}

func claimsOf(t *testing.T, token string) jwt.MapClaims {
	t.Helper()
	parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return []byte(sessionCfg.JWTSecretKey), nil })
	require.NoError(t, err)
	return parsed.Claims.(jwt.MapClaims)
}

func TestIssueSessionStoresASessionAndPutsItsIdInTheToken(t *testing.T) {
	db, a, _ := sessionTestDB(t)
	token, err := IssueSession(db, a, sessionCfg, "203.0.113.5", "Mozilla/5.0 Firefox")
	require.NoError(t, err)

	var s models.Session
	require.NoError(t, db.Where("user_id = ?", a.ID).First(&s).Error)
	assert.Equal(t, s.PublicID, claimsOf(t, token)["sid"])
	assert.EqualValues(t, 0, claimsOf(t, token)["tv"])
	assert.Equal(t, "203.0.113.5", s.IP)
	assert.Equal(t, "Mozilla/5.0 Firefox", s.UserAgent)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), s.ExpiresAt, time.Minute)
	assert.Len(t, s.PublicID, 32)

	// Two logins, two sessions with different ids
	_, err = IssueSession(db, a, sessionCfg, "", "")
	require.NoError(t, err)
	var n int64
	db.Model(&models.Session{}).Where("user_id = ?", a.ID).Count(&n)
	assert.EqualValues(t, 2, n)
}

func TestIssueSessionTruncatesLongUserAgents(t *testing.T) {
	db, a, _ := sessionTestDB(t)
	_, err := IssueSession(db, a, sessionCfg, "", string(make([]byte, 5000)))
	require.NoError(t, err)
	var s models.Session
	require.NoError(t, db.First(&s).Error)
	assert.LessOrEqual(t, len(s.UserAgent), 255)
}

func TestCheckSession(t *testing.T) {
	db, a, b := sessionTestDB(t)
	token, _ := IssueSession(db, a, sessionCfg, "", "")
	sid := claimsOf(t, token)["sid"].(string)

	assert.True(t, SessionActive(db, a.ID, sid))
	assert.False(t, SessionActive(db, b.ID, sid), "another user's id never matches")
	assert.False(t, SessionActive(db, a.ID, "nope"))

	ok, err := RevokeSession(db, a.ID, sid)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, SessionActive(db, a.ID, sid))

	ok, _ = RevokeSession(db, a.ID, sid)
	assert.False(t, ok, "already revoked")
}

func TestExpiredSessionIsNotActive(t *testing.T) {
	db, a, _ := sessionTestDB(t)
	token, _ := IssueSession(db, a, sessionCfg, "", "")
	sid := claimsOf(t, token)["sid"].(string)
	require.NoError(t, db.Model(&models.Session{}).Where("public_id = ?", sid).Update("expires_at", time.Now().Add(-time.Minute)).Error)
	assert.False(t, SessionActive(db, a.ID, sid))
	list, _ := ListSessions(db, a.ID)
	assert.Empty(t, list)
}

func TestTouchSessionIsThrottled(t *testing.T) {
	db, a, _ := sessionTestDB(t)
	token, _ := IssueSession(db, a, sessionCfg, "", "")
	sid := claimsOf(t, token)["sid"].(string)
	old := time.Now().Add(-time.Hour)
	require.NoError(t, db.Model(&models.Session{}).Where("public_id = ?", sid).Update("last_seen_at", old).Error)

	TouchSession(db, sid)
	var s models.Session
	require.NoError(t, db.Where("public_id = ?", sid).First(&s).Error)
	assert.WithinDuration(t, time.Now(), s.LastSeenAt, time.Minute)

	seen := s.LastSeenAt
	TouchSession(db, sid) // within a minute: no write
	require.NoError(t, db.Where("public_id = ?", sid).First(&s).Error)
	assert.True(t, seen.Equal(s.LastSeenAt))
}

func TestListSessionsIsPerUserAndNewestFirst(t *testing.T) {
	db, a, b := sessionTestDB(t)
	first, _ := IssueSession(db, a, sessionCfg, "", "one")
	time.Sleep(10 * time.Millisecond)
	_, _ = IssueSession(db, a, sessionCfg, "", "two")
	_, _ = IssueSession(db, b, sessionCfg, "", "other user")
	revoked, _ := IssueSession(db, a, sessionCfg, "", "revoked")
	_, _ = RevokeSession(db, a.ID, claimsOf(t, revoked)["sid"].(string))

	list, err := ListSessions(db, a.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "two", list[0].UserAgent)
	assert.Equal(t, claimsOf(t, first)["sid"], list[1].PublicID)
}

func TestSignOutEverywhereBumpsTheVersionAndRevokesEverySession(t *testing.T) {
	db, a, b := sessionTestDB(t)
	_, _ = IssueSession(db, a, sessionCfg, "", "")
	_, _ = IssueSession(db, a, sessionCfg, "", "")
	other, _ := IssueSession(db, b, sessionCfg, "", "")

	require.NoError(t, RevokeAllSessions(db, a.ID))

	var got models.User
	require.NoError(t, db.First(&got, a.ID).Error)
	assert.Equal(t, 1, got.TokenVersion)
	list, _ := ListSessions(db, a.ID)
	assert.Empty(t, list)

	// The other user is untouched
	var gotB models.User
	require.NoError(t, db.First(&gotB, b.ID).Error)
	assert.Equal(t, 0, gotB.TokenVersion)
	assert.True(t, SessionActive(db, b.ID, claimsOf(t, other)["sid"].(string)))

	// A token issued afterwards carries the new version
	token, _ := IssueSession(db, got, sessionCfg, "", "")
	assert.EqualValues(t, 1, claimsOf(t, token)["tv"])
}

func TestSessionIDFromToken(t *testing.T) {
	db, a, _ := sessionTestDB(t)
	token, _ := IssueSession(db, a, sessionCfg, "", "")
	assert.Equal(t, claimsOf(t, token)["sid"], SessionIDFromToken(token, sessionCfg))
	assert.Empty(t, SessionIDFromToken("garbage", sessionCfg))
	assert.Empty(t, SessionIDFromToken(token, &config.Config{JWTSecretKey: "another-secret-another-secret-12345"}))
	legacy, _ := GenerateToken(a, sessionCfg)
	assert.Empty(t, SessionIDFromToken(legacy, sessionCfg))
}

func TestPurgeSessionsAndAuditLog(t *testing.T) {
	db, a, _ := sessionTestDB(t)
	now := time.Now()
	old := now.Add(-400 * 24 * time.Hour)
	// Sessions: one expired long ago, one revoked long ago, one live
	live := models.Session{UserID: a.ID, PublicID: "live", ExpiresAt: now.Add(time.Hour)}
	expired := models.Session{UserID: a.ID, PublicID: "expired", ExpiresAt: now.Add(-48 * time.Hour)}
	revokedAt := now.Add(-48 * time.Hour)
	revoked := models.Session{UserID: a.ID, PublicID: "revoked", ExpiresAt: now.Add(time.Hour), RevokedAt: &revokedAt}
	require.NoError(t, db.Create(&[]models.Session{live, expired, revoked}).Error)
	require.NoError(t, db.Create(&[]models.AuditLog{
		{Event: models.AuditLogin, CreatedAt: old}, {Event: models.AuditLogin, CreatedAt: now.Add(-24 * time.Hour)},
	}).Error)

	n, err := PurgeAuditLog(db, 365, now)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	var left int64
	db.Model(&models.AuditLog{}).Count(&left)
	assert.EqualValues(t, 1, left)

	// 0 keeps everything
	n, _ = PurgeAuditLog(db, 0, now.Add(1000*24*time.Hour))
	assert.EqualValues(t, 0, n)

	n, err = PurgeSessions(db, now)
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
	var ids []string
	db.Model(&models.Session{}).Pluck("public_id", &ids)
	assert.Equal(t, []string{"live"}, ids)
}

func TestWriteAuditNeverFails(t *testing.T) {
	db, a, _ := sessionTestDB(t)
	uid := a.ID
	WriteAudit(db, models.AuditLog{UserID: &uid, Username: "a", Event: models.AuditLogin, IP: "1.2.3.4", UserAgent: string(make([]byte, 1000)), Detail: string(make([]byte, 5000))})
	var e models.AuditLog
	require.NoError(t, db.First(&e).Error)
	assert.Equal(t, "a", e.Username)
	assert.LessOrEqual(t, len(e.UserAgent), 255)
	assert.LessOrEqual(t, len(e.Detail), 1000)

	// A database without the table: logged, not a panic
	broken, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NotPanics(t, func() { WriteAudit(broken, models.AuditLog{Event: models.AuditLogin}) })
}
