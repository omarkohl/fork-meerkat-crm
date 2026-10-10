package services

import (
	"os"
	"path/filepath"
	"testing"

	"meerkat/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600))
	}
}

func TestReplaceContactPhotoSwapsFilesAndClearsTheLegacyCopy(t *testing.T) {
	db := thumbTestDB(t)
	dir := t.TempDir()
	touch(t, dir, "old_photo.jpg", "old_thumb.jpg", "new_photo.jpg", "new_thumb.jpg")
	c := models.Contact{UserID: 1, Firstname: "A", Photo: "old_photo.jpg", PhotoThumbnailFile: "old_thumb.jpg", LegacyThumbnail: "data:image/jpeg;base64,AAAA"}
	require.NoError(t, db.Create(&c).Error)

	require.NoError(t, replaceContactPhoto(db, dir, c.ID, "new_photo.jpg", "new_thumb.jpg"))

	var got models.Contact
	require.NoError(t, db.First(&got, c.ID).Error)
	assert.Equal(t, "new_photo.jpg", got.Photo)
	assert.Equal(t, "new_thumb.jpg", got.PhotoThumbnailFile)
	assert.Empty(t, got.LegacyThumbnail)
	assert.NoFileExists(t, filepath.Join(dir, "old_photo.jpg"))
	assert.NoFileExists(t, filepath.Join(dir, "old_thumb.jpg"))
	assert.FileExists(t, filepath.Join(dir, "new_photo.jpg"))
}

func TestReplaceContactPhotoDeletesTheNewFilesWhenTheUpdateFails(t *testing.T) {
	db := thumbTestDB(t)
	dir := t.TempDir()
	touch(t, dir, "old_photo.jpg", "new_photo.jpg", "new_thumb.jpg")
	c := models.Contact{UserID: 1, Firstname: "A", Photo: "old_photo.jpg"}
	require.NoError(t, db.Create(&c).Error)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_update BEFORE UPDATE ON contacts BEGIN SELECT RAISE(ABORT, 'no'); END").Error)

	require.Error(t, replaceContactPhoto(db, dir, c.ID, "new_photo.jpg", "new_thumb.jpg"))

	assert.FileExists(t, filepath.Join(dir, "old_photo.jpg"))
	assert.NoFileExists(t, filepath.Join(dir, "new_photo.jpg"))
	assert.NoFileExists(t, filepath.Join(dir, "new_thumb.jpg"))
}
