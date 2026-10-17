package contacts_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"meerkat/api"
	"meerkat/clock"
	"meerkat/config"
	"meerkat/contacts"
	"meerkat/middleware"
	"meerkat/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type env struct {
	db     *gorm.DB
	a, b   models.User
	asA    *gin.Engine
	asB    *gin.Engine
	anon   *gin.Engine
	photos string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:?_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Contact{}, &models.Activity{}, &models.Note{}, &models.Relationship{},
		&models.Reminder{}, &models.Webhook{}, &models.WebhookDelivery{}))
	e := &env{db: db, photos: t.TempDir()}
	e.a = models.User{Username: "a", Email: "a@example.com", Password: "x"}
	e.b = models.User{Username: "b", Email: "b@example.com", Password: "x"}
	require.NoError(t, db.Create(&e.a).Error)
	require.NoError(t, db.Create(&e.b).Error)

	cfg := &config.Config{ProfilePhotoDir: e.photos}
	mount := func(userID uint) *gin.Engine {
		r := gin.New()
		r.Use(middleware.ClockMiddleware(clock.Real()))
		authenticated := r.Group("")
		if userID != 0 {
			authenticated.Use(func(c *gin.Context) { c.Set("userID", userID); c.Next() })
		}
		contacts.New(contacts.Deps{DB: db, Config: cfg}).Register(api.New(r.Group(""), r), authenticated)
		return r
	}
	e.asA, e.asB, e.anon = mount(e.a.ID), mount(e.b.ID), mount(0)
	return e
}

func (e *env) contact(t *testing.T, owner uint, name string, circles ...string) models.Contact {
	t.Helper()
	c := models.Contact{UserID: owner, Firstname: name, Circles: circles}
	require.NoError(t, e.db.Create(&c).Error)
	return c
}

func do(r http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func idPath(c models.Contact, suffix string) string {
	return "/contacts/" + strconv.Itoa(int(c.ID)) + suffix
}

func TestArchiveHidesTheContactAndDeletesItsReminders(t *testing.T) {
	e := newEnv(t)
	c := e.contact(t, e.a.ID, "Alice")
	require.NoError(t, e.db.Create(&models.Reminder{UserID: e.a.ID, ContactID: &c.ID, Message: "call", RemindAt: time.Now(), Recurrence: "once"}).Error)

	w := do(e.asA, http.MethodPost, idPath(c, "/archive"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["archived"])
	assert.Equal(t, "Alice", body["firstname"])
	// The JSON names are those of the old model, not new ones.
	for _, key := range []string{"ID", "CreatedAt", "UpdatedAt", "DeletedAt", "firstname", "relationships", "custom_fields", "how_we_met"} {
		assert.Contains(t, body, key)
	}
	assert.Nil(t, body["DeletedAt"])

	var stored models.Contact
	require.NoError(t, e.db.First(&stored, c.ID).Error)
	assert.True(t, stored.Archived)
	var reminders int64
	e.db.Model(&models.Reminder{}).Where("contact_id = ?", c.ID).Count(&reminders)
	assert.Zero(t, reminders, "archiving deletes the reminders")
}

func TestUnarchive(t *testing.T) {
	e := newEnv(t)
	c := e.contact(t, e.a.ID, "Alice")
	require.NoError(t, e.db.Model(&c).Update("archived", true).Error)

	w := do(e.asA, http.MethodPost, idPath(c, "/unarchive"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, false, body["archived"])
	var stored models.Contact
	require.NoError(t, e.db.First(&stored, c.ID).Error)
	assert.False(t, stored.Archived)
}

func TestDeleteMovesTheContactAndItsDependantsToTheTrash(t *testing.T) {
	e := newEnv(t)
	c := e.contact(t, e.a.ID, "Alice")
	other := e.contact(t, e.a.ID, "Bob")
	require.NoError(t, e.db.Create(&models.Note{UserID: e.a.ID, ContactID: &c.ID, Content: "n"}).Error)
	require.NoError(t, e.db.Create(&models.Reminder{UserID: e.a.ID, ContactID: &c.ID, Message: "r", RemindAt: time.Now(), Recurrence: "once"}).Error)
	require.NoError(t, e.db.Create(&models.Relationship{UserID: e.a.ID, ContactID: c.ID, RelatedContactID: &other.ID, Name: "Bob", Type: "friend"}).Error)

	w := do(e.asA, http.MethodDelete, idPath(c, ""))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"message":"Contact deleted"}`, w.Body.String())

	var live int64
	e.db.Model(&models.Contact{}).Where("id = ?", c.ID).Count(&live)
	assert.Zero(t, live)
	var trashed int64
	e.db.Unscoped().Model(&models.Contact{}).Where("id = ? AND deleted_at IS NOT NULL", c.ID).Count(&trashed)
	assert.EqualValues(t, 1, trashed, "soft-deleted, so it can be restored")
	for _, m := range []any{&models.Note{}, &models.Reminder{}, &models.Relationship{}} {
		var n int64
		e.db.Model(m).Where("contact_id = ?", c.ID).Count(&n)
		assert.Zero(t, n, "dependants are soft-deleted with the contact")
	}
	e.db.Model(&models.Contact{}).Where("id = ?", other.ID).Count(&live)
	assert.EqualValues(t, 1, live, "other contacts stay")
}

func TestDeleteKeepsPhotoFilesForTheTrash(t *testing.T) {
	e := newEnv(t)
	photo := filepath.Join(e.photos, "p.jpg")
	require.NoError(t, os.WriteFile(photo, []byte("x"), 0o600))
	c := models.Contact{UserID: e.a.ID, Firstname: "Alice", Photo: "p.jpg"}
	require.NoError(t, e.db.Create(&c).Error)

	require.Equal(t, http.StatusOK, do(e.asA, http.MethodDelete, idPath(c, "")).Code)

	_, err := os.Stat(photo)
	assert.NoError(t, err, "a restore from the trash must find the photo")
}

func TestCirclesAreDistinctAndOnlyTheActorsLiveContacts(t *testing.T) {
	e := newEnv(t)
	e.contact(t, e.a.ID, "Alice", "Friends", "Family")
	e.contact(t, e.a.ID, "Bob", "Friends", "Work")
	e.contact(t, e.b.ID, "Mallory", "Secret")
	gone := e.contact(t, e.a.ID, "Gone", "Trashed")
	require.NoError(t, e.db.Delete(&gone).Error)

	w := do(e.asA, http.MethodGet, "/contacts/circles")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var names []string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &names))
	assert.ElementsMatch(t, []string{"Friends", "Family", "Work"}, names)
}

func TestCirclesWithoutContactsIsNullLikeBefore(t *testing.T) {
	e := newEnv(t)
	w := do(e.asA, http.MethodGet, "/contacts/circles")
	require.Equal(t, http.StatusOK, w.Code)
	t.Logf("empty circles body: %q", w.Body.String())
	var names []string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &names))
	assert.Empty(t, names)
}

// Isolation: another user's contact is a 404 for every operation, and is left untouched.
func TestOtherUsersContactsAreNotFound(t *testing.T) {
	e := newEnv(t)
	victim := e.contact(t, e.a.ID, "Alice")

	for _, call := range []struct{ method, suffix string }{
		{http.MethodPost, "/archive"}, {http.MethodPost, "/unarchive"}, {http.MethodDelete, ""},
	} {
		w := do(e.asB, call.method, idPath(victim, call.suffix))
		assert.Equal(t, http.StatusNotFound, w.Code, call.method+call.suffix+": "+w.Body.String())
		var b struct {
			Error struct {
				Code    string
				Details map[string]any
			}
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, "NOT_FOUND", b.Error.Code)
		assert.Equal(t, strconv.Itoa(int(victim.ID)), b.Error.Details["id"])
	}
	var stored models.Contact
	require.NoError(t, e.db.First(&stored, victim.ID).Error)
	assert.False(t, stored.Archived, "untouched")
}

func TestMissingContactAndBadIDs(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, http.StatusNotFound, do(e.asA, http.MethodPost, "/contacts/9999/archive").Code)
	assert.Equal(t, http.StatusBadRequest, do(e.asA, http.MethodPost, "/contacts/abc/archive").Code)
	assert.Equal(t, http.StatusBadRequest, do(e.asA, http.MethodDelete, "/contacts/0").Code)
}

func TestWithoutAnAuthenticatedUserTheOperationsRefuse(t *testing.T) {
	e := newEnv(t)
	c := e.contact(t, e.a.ID, "Alice")
	for _, call := range []struct{ method, path string }{
		{http.MethodPost, idPath(c, "/archive")}, {http.MethodDelete, idPath(c, "")}, {http.MethodGet, "/contacts/circles"},
	} {
		assert.Equal(t, http.StatusUnauthorized, do(e.anon, call.method, call.path).Code, call.method+" "+call.path)
	}
	var stored models.Contact
	require.NoError(t, e.db.First(&stored, c.ID).Error)
	assert.False(t, stored.Archived)
}
