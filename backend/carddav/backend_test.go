package carddav

import (
	"context"
	"testing"
	"time"

	"meerkat/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDeleteAddressObjectTrashesDependants(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Contact{}, &models.Note{}, &models.Relationship{}, &models.Reminder{}))
	user := models.User{Username: "u", Password: "password123!A", Email: "u@example.com"}
	require.NoError(t, db.Create(&user).Error)
	contact := models.Contact{UserID: user.ID, Firstname: "Al", VCardUID: "uid-1"}
	require.NoError(t, db.Create(&contact).Error)
	cid := contact.ID
	require.NoError(t, db.Create(&models.Reminder{UserID: user.ID, Message: "r", RemindAt: time.Now(), Recurrence: "once", ContactID: &cid}).Error)

	b := NewBackend(db, "")
	ctx := ContextWithUser(context.Background(), user.ID, "u", db, "")
	require.NoError(t, b.DeleteAddressObject(ctx, "/carddav/addressbooks/u/contacts/uid-1.vcf"))

	var live int64
	require.NoError(t, db.Model(&models.Reminder{}).Where("contact_id = ?", cid).Count(&live).Error)
	assert.Zero(t, live)
	require.NoError(t, db.Model(&models.Contact{}).Where("id = ?", cid).Count(&live).Error)
	assert.Zero(t, live)
}
