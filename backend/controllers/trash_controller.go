package controllers

import (
	"errors"
	"meerkat/middleware"
	"net/http"
	"os"

	apperrors "meerkat/errors"
	"meerkat/idutil"
	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// trashRetentionDays is the configured retention, or the default when unset.
func trashRetentionDays(c *gin.Context) int {
	if days := currentConfig(c).TrashRetentionDays; days >= 1 {
		return days
	}
	return services.DefaultTrashRetentionDays
}

// trashPhotoDir is where profile photos live; the environment is the fallback
// for contexts without a loaded config.
func trashPhotoDir(c *gin.Context) string {
	if dir := currentConfig(c).ProfilePhotoDir; dir != "" {
		return dir
	}
	return os.Getenv("PROFILE_PHOTO_DIR")
}

func trashContactID(c *gin.Context) (uint, bool) {
	id, err := idutil.Parse(c.Param("id"))
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact").WithDetails("id", c.Param("id")))
		return 0, false
	}
	return id, true
}

// GetTrash lists the current user's deleted contacts.
func GetTrash(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	contacts, err := services.ListTrashedContacts(db, userID)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve trash").WithError(err))
		return
	}
	days := trashRetentionDays(c)
	resp := models.TrashResponse{Contacts: make([]models.TrashedContact, len(contacts)), RetentionDays: days}
	for i, ct := range contacts {
		resp.Contacts[i] = models.TrashedContact{
			ID:             ct.ID,
			Firstname:      ct.Firstname,
			Lastname:       ct.Lastname,
			Nickname:       ct.Nickname,
			Email:          ct.Email,
			PhotoThumbnail: models.ThumbnailURL(ct.ID, ct.PhotoThumbnailFile),
			DeletedAt:      ct.DeletedAt.Time,
			PurgeAt:        services.TrashPurgeAt(ct.DeletedAt.Time, days),
		}
	}
	c.JSON(http.StatusOK, resp)
}

// RestoreTrashedContact brings a deleted contact back with its notes, reminders and relationships.
func RestoreTrashedContact(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := trashContactID(c)
	if !ok {
		return
	}
	contact, err := services.RestoreTrashedContact(db, userID, id)
	if errors.Is(err, services.ErrTrashedContactNotFound) {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact").WithDetails("id", c.Param("id")))
		return
	}
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to restore contact").WithError(err))
		return
	}
	go services.TriggerWebhooks(db, currentConfig(c), userID, "contact.restored", contact, middleware.RequestClock(c))
	c.JSON(http.StatusOK, gin.H{"message": "Contact restored", "id": contact.ID})
}

// PurgeTrashedContact permanently deletes one contact from the trash.
func PurgeTrashedContact(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := trashContactID(c)
	if !ok {
		return
	}
	n, err := services.PurgeTrashedContacts(db, userID, []uint{id}, trashPhotoDir(c), middleware.RequestNow(c))
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to delete contact permanently").WithError(err))
		return
	}
	if n == 0 {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact").WithDetails("id", c.Param("id")))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Contact deleted permanently"})
}

// EmptyTrash permanently deletes every contact in the current user's trash.
func EmptyTrash(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	n, err := services.PurgeTrashedContacts(db, userID, nil, trashPhotoDir(c), middleware.RequestNow(c))
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to empty trash").WithError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Trash emptied", "purged": n})
}
