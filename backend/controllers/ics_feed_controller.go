package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	apperrors "meerkat/errors"
	"meerkat/logger"
	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// icsFetchedThrottle bounds how often a fetch writes last_fetched_at, so a calendar
// app polling every few minutes costs at most one small write a minute.
const icsFetchedThrottle = time.Minute

// icsTokenPattern matches a base64url-encoded 32-byte token. Anything else cannot
// be a valid token, so the database is not queried for it.
var icsTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func hashIcsToken(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}

func icsFeedStatus(feed *models.IcsFeed) models.IcsFeedStatus {
	if feed == nil {
		return models.IcsFeedStatus{}
	}
	created := feed.CreatedAt
	return models.IcsFeedStatus{Enabled: true, CreatedAt: &created, LastFetchedAt: feed.LastFetchedAt}
}

// GetIcsFeed reports whether the current user has an active feed. It never reveals the token.
func GetIcsFeed(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var feed models.IcsFeed
	err := db.Where("user_id = ?", userID).First(&feed).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, icsFeedStatus(nil))
		return
	}
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("query").WithError(err))
		return
	}
	c.JSON(http.StatusOK, icsFeedStatus(&feed))
}

// CreateIcsFeed creates the user's feed or, if one exists, replaces its token so the
// old URL stops working. The plaintext token is returned only here.
func CreateIcsFeed(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		apperrors.AbortWithError(c, apperrors.ErrInternal("token generation failed"))
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	// Replace the row. Two concurrent requests can both pass the delete and then
	// collide on the unique user_id; retrying once lets the later one win.
	var feed models.IcsFeed
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		feed = models.IcsFeed{UserID: userID, TokenHash: hashIcsToken(token)}
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("user_id = ?", userID).Delete(&models.IcsFeed{}).Error; err != nil {
				return err
			}
			return tx.Create(&feed).Error
		})
		if err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			break
		}
	}
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("insert").WithError(err))
		return
	}

	audit(c, db, models.AuditICSFeedCreated, "")

	c.JSON(http.StatusCreated, models.IcsFeedCreateResponse{
		IcsFeedStatus: icsFeedStatus(&feed),
		Token:         token,
	})
}

// DeleteIcsFeed revokes the feed. It is idempotent.
func DeleteIcsFeed(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	if err := db.Where("user_id = ?", userID).Delete(&models.IcsFeed{}).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("delete").WithError(err))
		return
	}
	audit(c, db, models.AuditICSFeedRevoked, "")
	c.JSON(http.StatusOK, gin.H{"message": "Calendar feed revoked successfully"})
}

// ServeIcsFeed is the public feed endpoint (GET /ics/<token>.ics). The token is the
// only credential: the session user, if any, is ignored and the data is always the
// feed owner's. Every failure answers 404 so tokens cannot be probed.
func ServeIcsFeed(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex")
	notFound := func() {
		c.Header("Cache-Control", "no-store")
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Calendar feed"))
	}

	token, hasSuffix := strings.CutSuffix(c.Param("file"), ".ics")
	if !hasSuffix || !icsTokenPattern.MatchString(token) {
		notFound()
		return
	}

	hash := hashIcsToken(token)
	var feed models.IcsFeed
	if err := db.Where("token_hash = ?", hash).First(&feed).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("query").WithError(err))
			return
		}
		notFound()
		return
	}
	// The lookup is by the hash of a 256-bit random token; the explicit comparison
	// keeps the check constant-time regardless of how the database matched.
	if subtle.ConstantTimeCompare([]byte(feed.TokenHash), []byte(hash)) != 1 {
		notFound()
		return
	}

	// Users are soft-deleted, so a feed can outlive its owner: no owner, no feed.
	var user models.User
	if err := db.Select("id", "language").First(&user, feed.UserID).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("query").WithError(err))
			return
		}
		notFound()
		return
	}
	lang := "en"
	if user.Language != "" {
		lang = user.Language
	}

	body, err := services.BuildICSFeed(db, feed.UserID, lang)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("build calendar feed").WithError(err))
		return
	}

	recordIcsFetch(db, feed.ID)

	sum := sha256.Sum256([]byte(body))
	etag := fmt.Sprintf(`"%x"`, sum[:16])
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, max-age=900")
	if match := c.GetHeader("If-None-Match"); match != "" && strings.Contains(match, etag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.Header("Content-Disposition", `inline; filename="meerkat.ics"`)
	c.Data(http.StatusOK, "text/calendar; charset=utf-8", []byte(body))
}

// recordIcsFetch stamps last_fetched_at unless it was stamped less than a minute ago.
// The condition lives in the UPDATE, so concurrent fetches write at most once. A failure
// is logged and ignored: serving the calendar matters more than the timestamp.
func recordIcsFetch(db *gorm.DB, feedID uint) {
	now := time.Now().UTC()
	err := db.Model(&models.IcsFeed{}).
		Where("id = ? AND (last_fetched_at IS NULL OR last_fetched_at < ?)", feedID, now.Add(-icsFetchedThrottle)).
		UpdateColumn("last_fetched_at", now).Error
	if err != nil {
		logger.Logger.Warn().Err(err).Uint("ics_feed_id", feedID).Msg("Failed to update ics feed last_fetched_at")
	}
}
