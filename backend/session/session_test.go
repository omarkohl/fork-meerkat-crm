package session

import (
	"testing"
	"time"

	"meerkat/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.Session{}))
	return db
}

func TestFind(t *testing.T) {
	db := testDB(t)
	revoked := time.Now()
	require.NoError(t, db.Create(&[]models.Session{
		{UserID: 1, PublicID: "live", ExpiresAt: time.Now().Add(time.Hour)},
		{UserID: 1, PublicID: "revoked", ExpiresAt: time.Now().Add(time.Hour), RevokedAt: &revoked},
		{UserID: 1, PublicID: "expired", ExpiresAt: time.Now().Add(-time.Minute)},
	}).Error)

	s, err := Find(db, 1, "live")
	require.NoError(t, err)
	assert.Equal(t, "live", s.PublicID)
	for _, id := range []string{"revoked", "expired", "unknown"} {
		_, err := Find(db, 1, id)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound, id)
	}
	_, err = Find(db, 2, "live")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "another user's id never matches")
}

func TestTouchWritesOnlyWhenTheStampIsOld(t *testing.T) {
	db := testDB(t)
	old := time.Now().Add(-time.Hour)
	require.NoError(t, db.Create(&models.Session{UserID: 1, PublicID: "s", ExpiresAt: time.Now().Add(time.Hour), LastSeenAt: old}).Error)

	s, _ := Find(db, 1, "s")
	require.NoError(t, Touch(db, s))
	var after models.Session
	require.NoError(t, db.First(&after).Error)
	assert.WithinDuration(t, time.Now(), after.LastSeenAt, time.Minute)

	// Fresh stamp: no write at all. Prove it by making the row unwritable for this id.
	s, _ = Find(db, 1, "s")
	require.NoError(t, db.Exec("CREATE TRIGGER no_writes BEFORE UPDATE ON sessions BEGIN SELECT RAISE(ABORT, 'write'); END").Error)
	assert.NoError(t, Touch(db, s), "no UPDATE is issued while the stamp is fresh")

	// With an old stamp the same trigger makes the write visible
	s.LastSeenAt = old
	assert.Error(t, Touch(db, s))
}
