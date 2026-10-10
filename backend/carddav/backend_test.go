package carddav

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-vcard"

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

func putPhotoBackend(t *testing.T) (*Backend, context.Context, *gorm.DB, models.User, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Contact{}))
	user := models.User{Username: "u", Password: "password123!A", Email: "u@example.com"}
	require.NoError(t, db.Create(&user).Error)
	dir := t.TempDir()
	return NewBackend(db, dir), ContextWithUser(context.Background(), user.ID, "u", db, dir), db, user, dir
}

func photoCard(t *testing.T) vcard.Card {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 60, 60))))
	src := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:uid-1\r\nFN:Al Smith\r\nN:Smith;Al;;;\r\nPHOTO;ENCODING=b;TYPE=PNG:" +
		base64.StdEncoding.EncodeToString(buf.Bytes()) + "\r\nEND:VCARD\r\n"
	card, err := vcard.NewDecoder(strings.NewReader(src)).Decode()
	require.NoError(t, err)
	return card
}

func TestPutAddressObjectReplacingAPhotoDeletesTheOldFilesAndTheLegacyCopy(t *testing.T) {
	b, ctx, db, user, dir := putPhotoBackend(t)
	for _, n := range []string{"old_photo.jpg", "old_thumb.jpg"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600))
	}
	contact := models.Contact{UserID: user.ID, Firstname: "Al", VCardUID: "uid-1", Photo: "old_photo.jpg",
		PhotoThumbnailFile: "old_thumb.jpg", LegacyThumbnail: "data:image/jpeg;base64,AAAA"}
	require.NoError(t, db.Create(&contact).Error)

	_, err := b.PutAddressObject(ctx, "/carddav/addressbooks/u/contacts/uid-1.vcf", photoCard(t), nil)
	require.NoError(t, err)

	var got models.Contact
	require.NoError(t, db.First(&got, contact.ID).Error)
	assert.NotEqual(t, "old_photo.jpg", got.Photo)
	assert.NotEmpty(t, got.PhotoThumbnailFile)
	assert.Empty(t, got.LegacyThumbnail)
	assert.NoFileExists(t, filepath.Join(dir, "old_photo.jpg"))
	assert.NoFileExists(t, filepath.Join(dir, "old_thumb.jpg"))
	assert.FileExists(t, filepath.Join(dir, got.Photo))
	assert.FileExists(t, filepath.Join(dir, got.PhotoThumbnailFile))
}

func TestPutAddressObjectDeletesTheNewFilesWhenTheSaveFails(t *testing.T) {
	b, ctx, db, user, dir := putPhotoBackend(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "old_photo.jpg"), []byte("x"), 0o600))
	contact := models.Contact{UserID: user.ID, Firstname: "Al", VCardUID: "uid-1", Photo: "old_photo.jpg"}
	require.NoError(t, db.Create(&contact).Error)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_update BEFORE UPDATE ON contacts BEGIN SELECT RAISE(ABORT, 'no'); END").Error)

	_, err := b.PutAddressObject(ctx, "/carddav/addressbooks/u/contacts/uid-1.vcf", photoCard(t), nil)
	require.Error(t, err)

	entries, _ := os.ReadDir(dir)
	require.Len(t, entries, 1, "only the old photo is left")
	assert.Equal(t, "old_photo.jpg", entries[0].Name())
}

func TestPhotoSwapCommitAndRollback(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"old_photo.jpg", "old_thumb.jpg", "new_photo.jpg", "new_thumb.jpg"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600))
	}
	c := &models.Contact{Photo: "old_photo.jpg", PhotoThumbnailFile: "old_thumb.jpg", LegacyThumbnail: "data:image/jpeg;base64,AA"}
	swap := NewPhotoSwap(c)
	swap.Apply(c, "new_photo.jpg", "new_thumb.jpg")
	assert.Empty(t, c.LegacyThumbnail)

	swap.Rollback(dir, c)
	assert.Equal(t, "old_photo.jpg", c.Photo)
	assert.Equal(t, "data:image/jpeg;base64,AA", c.LegacyThumbnail)
	assert.NoFileExists(t, filepath.Join(dir, "new_photo.jpg"))
	assert.FileExists(t, filepath.Join(dir, "old_photo.jpg"))

	swap.Commit(dir)
	assert.NoFileExists(t, filepath.Join(dir, "old_photo.jpg"))
	assert.NoFileExists(t, filepath.Join(dir, "old_thumb.jpg"))

	// Nothing replaced: Commit and Rollback leave everything alone.
	touch := filepath.Join(dir, "keep_photo.jpg")
	require.NoError(t, os.WriteFile(touch, []byte("x"), 0o600))
	kept := &models.Contact{Photo: "keep_photo.jpg"}
	none := NewPhotoSwap(kept)
	none.Commit(dir)
	none.Rollback(dir, kept)
	assert.FileExists(t, touch)
}
