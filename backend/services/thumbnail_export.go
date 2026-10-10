package services

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"meerkat/logger"
	"meerkat/models"
	"meerkat/photofile"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExportLegacyThumbnails moves thumbnails that are still stored as base64 data URLs in the
// database (contacts.photo_thumbnail) into files in photoDir and records the file name in
// photo_thumbnail_file. It returns how many thumbnails it exported.
//
// It is safe to run at every startup: only rows without a file are looked at, an existing
// file name is never replaced, a row that cannot be decoded is logged and skipped, and the
// legacy column is left as it is (the copy a rollback needs; a later migration drops it).
// Trashed contacts are included so that restoring them shows their thumbnail.
func ExportLegacyThumbnails(db *gorm.DB, photoDir string) (int, error) {
	if photoDir == "" {
		return 0, nil
	}
	exported := 0
	var lastID uint
	for {
		var batch []models.Contact
		err := db.Unscoped().Select("id", "photo_thumbnail").
			Where("id > ? AND photo_thumbnail LIKE 'data:%' AND photo_thumbnail_file = ''", lastID).
			Order("id").Limit(200).Find(&batch).Error
		if err != nil {
			return exported, err
		}
		if len(batch) == 0 {
			return exported, nil
		}
		if err := photofile.EnsureDir(photoDir); err != nil {
			return exported, err
		}
		for _, c := range batch {
			lastID = c.ID
			name, err := writeLegacyThumbnail(photoDir, c.LegacyThumbnail)
			if err != nil {
				logger.Warn().Err(err).Uint("contact_id", c.ID).Msg("Skipping a thumbnail that cannot be moved to a file")
				continue
			}
			// Conditional: a concurrent upload that set a file wins.
			res := db.Unscoped().Model(&models.Contact{}).
				Where("id = ? AND photo_thumbnail_file = ''", c.ID).
				UpdateColumn("photo_thumbnail_file", name)
			if res.Error != nil || res.RowsAffected != 1 {
				os.Remove(filepath.Join(photoDir, name))
				if res.Error != nil {
					return exported, res.Error
				}
				continue
			}
			exported++
		}
	}
}

var errBadThumbnail = errors.New("thumbnail is not a base64 data URL")

func writeLegacyThumbnail(photoDir, dataURL string) (string, error) {
	_, payload, ok := strings.Cut(dataURL, ",")
	if !ok {
		return "", errBadThumbnail
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(raw) == 0 {
		return "", errBadThumbnail
	}
	name := uuid.New().String() + "_thumb.jpg"
	if err := photofile.WriteFile(filepath.Join(photoDir, name), raw); err != nil {
		return "", err
	}
	return name, nil
}
