package services

import (
	"testing"

	"meerkat/config"
	"meerkat/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func oidcDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	return db
}

func TestFindOrProvisionUserLinksOnlyVerifiedEmails(t *testing.T) {
	db := oidcDB(t)
	local := models.User{Username: "alice", Email: "alice@example.com", Password: "hash"}
	require.NoError(t, db.Create(&local).Error)
	cfg := &config.Config{}

	// An unverified email claim must not take over a local account.
	_, err := FindOrProvisionUser(db, &OIDCClaims{Subject: "evil", Email: "Alice@Example.com", Provider: "https://idp"}, cfg)
	assert.ErrorIs(t, err, ErrOIDCUserNotFound)

	// Verified email links the identity to the existing account.
	u, err := FindOrProvisionUser(db, &OIDCClaims{Subject: "sub-1", Email: "Alice@Example.com", EmailVerified: true, Provider: "https://idp"}, cfg)
	require.NoError(t, err)
	assert.Equal(t, local.ID, u.ID)

	// The next login finds it by subject, even if the email claim changed or is unverified.
	u, err = FindOrProvisionUser(db, &OIDCClaims{Subject: "sub-1", Email: "other@example.com", Provider: "https://idp"}, cfg)
	require.NoError(t, err)
	assert.Equal(t, local.ID, u.ID)

	// The same subject at another provider is a different identity.
	_, err = FindOrProvisionUser(db, &OIDCClaims{Subject: "sub-1", Provider: "https://other-idp"}, cfg)
	assert.ErrorIs(t, err, ErrOIDCUserNotFound)
}

func TestFindOrProvisionUserTrustEmail(t *testing.T) {
	db := oidcDB(t)
	require.NoError(t, db.Create(&models.User{Username: "bob", Email: "bob@example.com", Password: "hash"}).Error)
	cfg := &config.Config{}
	cfg.OIDC.TrustEmail = true
	u, err := FindOrProvisionUser(db, &OIDCClaims{Subject: "s", Email: "bob@example.com", Provider: "p"}, cfg)
	require.NoError(t, err)
	assert.Equal(t, "bob", u.Username)
}

func TestFindOrProvisionUserAutoProvision(t *testing.T) {
	db := oidcDB(t)
	cfg := &config.Config{}
	cfg.OIDC.AllowAutoProvision = true

	u, err := FindOrProvisionUser(db, &OIDCClaims{Subject: "s1", Email: "New@Example.com", PreferredUsername: "carol@corp", Provider: "p"}, cfg)
	require.NoError(t, err)
	assert.Equal(t, "carol", u.Username)
	assert.Equal(t, "new@example.com", u.Email)
	assert.Empty(t, u.Password, "OIDC accounts have no password")

	// A taken username gets a numeric suffix.
	u2, err := FindOrProvisionUser(db, &OIDCClaims{Subject: "s2", Email: "c2@example.com", PreferredUsername: "carol", Provider: "p"}, cfg)
	require.NoError(t, err)
	assert.Equal(t, "carol1", u2.Username)
}

// The unique index on username also covers soft-deleted users, so their names count as taken.
func TestFindOrProvisionUserSkipsUsernamesOfDeletedUsers(t *testing.T) {
	db := oidcDB(t)
	gone := models.User{Username: "dave", Email: "dave@example.com", Password: "hash"}
	require.NoError(t, db.Create(&gone).Error)
	require.NoError(t, db.Delete(&gone).Error)
	cfg := &config.Config{}
	cfg.OIDC.AllowAutoProvision = true

	u, err := FindOrProvisionUser(db, &OIDCClaims{Subject: "s", Email: "dave2@example.com", PreferredUsername: "dave", Provider: "p"}, cfg)
	require.NoError(t, err)
	assert.Equal(t, "dave1", u.Username)
}

func TestFindOrProvisionUserBackfillsEmail(t *testing.T) {
	db := oidcDB(t)
	sub, prov := "s", "p"
	require.NoError(t, db.Create(&models.User{Username: "erin", Password: "", OIDCSubject: &sub, OIDCProvider: &prov}).Error)
	u, err := FindOrProvisionUser(db, &OIDCClaims{Subject: "s", Email: "Erin@Example.com", Provider: "p"}, &config.Config{})
	require.NoError(t, err)
	assert.Equal(t, "erin@example.com", u.Email)
}
