package controllers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"meerkat/config"
	"meerkat/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type trashEnv struct {
	db      *gorm.DB
	userA   models.User
	userB   models.User
	routerA *gin.Engine
	routerB *gin.Engine
	photos  string
}

func newTrashEnv(t *testing.T) *trashEnv {
	t.Helper()
	db, routerA := setupRouter()
	require.NoError(t, db.AutoMigrate(&models.ReminderCompletion{}))
	var a models.User
	db.First(&a)
	b := models.User{Username: "other", Email: "other@example.com", Password: "x"}
	require.NoError(t, db.Create(&b).Error)
	photos := t.TempDir()

	routerA.Use(func(c *gin.Context) {
		c.Set("cfg", config.Config{ProfilePhotoDir: photos, TrashRetentionDays: 30})
		c.Next()
	})
	routerB := gin.New()
	routerB.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("userID", b.ID)
		c.Set("cfg", config.Config{ProfilePhotoDir: photos, TrashRetentionDays: 30})
		c.Next()
	})
	for _, r := range []*gin.Engine{routerA, routerB} {
		r.DELETE("/contacts/:id", DeleteContact)
		r.GET("/contacts", GetContacts)
		r.GET("/contacts/circles", GetCircles)
		r.GET("/trash/contacts", GetTrash)
		r.POST("/trash/contacts/:id/restore", RestoreTrashedContact)
		r.DELETE("/trash/contacts/:id", PurgeTrashedContact)
		r.DELETE("/trash/contacts", EmptyTrash)
	}
	return &trashEnv{db: db, userA: a, userB: b, routerA: routerA, routerB: routerB, photos: photos}
}

func (e *trashEnv) contact(t *testing.T, userID uint, name string) models.Contact {
	t.Helper()
	c := models.Contact{UserID: userID, Firstname: name, Circles: []string{"Circle-" + name}}
	require.NoError(t, e.db.Create(&c).Error)
	return c
}

func (e *trashEnv) trashed(t *testing.T, r *gin.Engine) models.TrashResponse {
	t.Helper()
	w := doReq(r, "GET", "/trash/contacts")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp models.TrashResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func (e *trashEnv) liveContacts(t *testing.T, r *gin.Engine) int {
	t.Helper()
	w := doReq(r, "GET", "/contacts")
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp.Total
}

func idPath(prefix string, id uint, suffix string) string {
	return prefix + strconv.Itoa(int(id)) + suffix
}

func TestDeleteMovesContactToTrashAndKeepsItsData(t *testing.T) {
	e := newTrashEnv(t)
	c := e.contact(t, e.userA.ID, "Alice")
	cid := c.ID
	note := models.Note{UserID: e.userA.ID, Content: "hi", Date: time.Now(), ContactID: &cid}
	require.NoError(t, e.db.Create(&note).Error)
	act := models.Activity{UserID: e.userA.ID, Title: "Lunch", Date: time.Now()}
	require.NoError(t, e.db.Create(&act).Error)
	require.NoError(t, e.db.Model(&act).Association("Contacts").Append(&c))
	require.NoError(t, os.WriteFile(filepath.Join(e.photos, "p.jpg"), []byte("x"), 0o600))
	require.NoError(t, e.db.Model(&c).Update("photo", "p.jpg").Error)

	require.Equal(t, http.StatusOK, doReq(e.routerA, "DELETE", idPath("/contacts/", c.ID, "")).Code)

	assert.Equal(t, 0, e.liveContacts(t, e.routerA))
	resp := e.trashed(t, e.routerA)
	require.Len(t, resp.Contacts, 1)
	assert.Equal(t, "Alice", resp.Contacts[0].Firstname)
	assert.Equal(t, 30, resp.RetentionDays)
	assert.WithinDuration(t, resp.Contacts[0].DeletedAt.Add(30*24*time.Hour), resp.Contacts[0].PurgeAt, time.Second)
	// The activity link and the photo wait in the trash.
	var joins int64
	require.NoError(t, e.db.Table("activity_contacts").Where("contact_id = ?", c.ID).Count(&joins).Error)
	assert.EqualValues(t, 1, joins)
	_, err := os.Stat(filepath.Join(e.photos, "p.jpg"))
	assert.NoError(t, err)
}

func TestDeletedContactIsHiddenFromCircles(t *testing.T) {
	e := newTrashEnv(t)
	c := e.contact(t, e.userA.ID, "Alice")
	e.contact(t, e.userA.ID, "Bob")
	require.Equal(t, http.StatusOK, doReq(e.routerA, "DELETE", idPath("/contacts/", c.ID, "")).Code)

	w := doReq(e.routerA, "GET", "/contacts/circles")
	var circles []string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &circles))
	assert.Equal(t, []string{"Circle-Bob"}, circles)
}

func TestRestoreFromTrashMakesContactVisibleAgain(t *testing.T) {
	e := newTrashEnv(t)
	c := e.contact(t, e.userA.ID, "Alice")
	doReq(e.routerA, "DELETE", idPath("/contacts/", c.ID, ""))

	w := doReq(e.routerA, "POST", idPath("/trash/contacts/", c.ID, "/restore"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, 1, e.liveContacts(t, e.routerA))
	assert.Empty(t, e.trashed(t, e.routerA).Contacts)
	// A second restore finds nothing in the trash.
	assert.Equal(t, http.StatusNotFound, doReq(e.routerA, "POST", idPath("/trash/contacts/", c.ID, "/restore")).Code)
}

func TestPurgeSingleContact(t *testing.T) {
	e := newTrashEnv(t)
	c1 := e.contact(t, e.userA.ID, "Alice")
	c2 := e.contact(t, e.userA.ID, "Bob")
	require.NoError(t, os.WriteFile(filepath.Join(e.photos, "p.jpg"), []byte("x"), 0o600))
	require.NoError(t, e.db.Model(&c1).Update("photo", "p.jpg").Error)
	doReq(e.routerA, "DELETE", idPath("/contacts/", c1.ID, ""))
	doReq(e.routerA, "DELETE", idPath("/contacts/", c2.ID, ""))

	require.Equal(t, http.StatusOK, doReq(e.routerA, "DELETE", idPath("/trash/contacts/", c1.ID, "")).Code)

	resp := e.trashed(t, e.routerA)
	require.Len(t, resp.Contacts, 1)
	assert.Equal(t, "Bob", resp.Contacts[0].Firstname)
	_, err := os.Stat(filepath.Join(e.photos, "p.jpg"))
	assert.True(t, os.IsNotExist(err))
	assert.Equal(t, http.StatusNotFound, doReq(e.routerA, "DELETE", idPath("/trash/contacts/", c1.ID, "")).Code)
}

func TestPurgeRefusesLiveContact(t *testing.T) {
	e := newTrashEnv(t)
	c := e.contact(t, e.userA.ID, "Alice")
	assert.Equal(t, http.StatusNotFound, doReq(e.routerA, "DELETE", idPath("/trash/contacts/", c.ID, "")).Code)
	assert.Equal(t, 1, e.liveContacts(t, e.routerA))
}

func TestEmptyTrash(t *testing.T) {
	e := newTrashEnv(t)
	e.contact(t, e.userA.ID, "Live")
	for _, n := range []string{"One", "Two"} {
		c := e.contact(t, e.userA.ID, n)
		doReq(e.routerA, "DELETE", idPath("/contacts/", c.ID, ""))
	}
	w := doReq(e.routerA, "DELETE", "/trash/contacts")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Purged int `json:"purged"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 2, body.Purged)
	assert.Empty(t, e.trashed(t, e.routerA).Contacts)
	assert.Equal(t, 1, e.liveContacts(t, e.routerA))
}

func TestTrashIsolationBetweenUsers(t *testing.T) {
	e := newTrashEnv(t)
	ca := e.contact(t, e.userA.ID, "Alice")
	cb := e.contact(t, e.userB.ID, "Bob")
	doReq(e.routerA, "DELETE", idPath("/contacts/", ca.ID, ""))
	doReq(e.routerB, "DELETE", idPath("/contacts/", cb.ID, ""))

	// Each user sees only their own trash.
	ra, rb := e.trashed(t, e.routerA), e.trashed(t, e.routerB)
	require.Len(t, ra.Contacts, 1)
	require.Len(t, rb.Contacts, 1)
	assert.Equal(t, "Alice", ra.Contacts[0].Firstname)
	assert.Equal(t, "Bob", rb.Contacts[0].Firstname)

	// B cannot restore or purge A's contact.
	assert.Equal(t, http.StatusNotFound, doReq(e.routerB, "POST", idPath("/trash/contacts/", ca.ID, "/restore")).Code)
	assert.Equal(t, http.StatusNotFound, doReq(e.routerB, "DELETE", idPath("/trash/contacts/", ca.ID, "")).Code)
	// B emptying the trash leaves A's alone.
	require.Equal(t, http.StatusOK, doReq(e.routerB, "DELETE", "/trash/contacts").Code)
	assert.Len(t, e.trashed(t, e.routerA).Contacts, 1)
	assert.Empty(t, e.trashed(t, e.routerB).Contacts)
	assert.EqualValues(t, 1, func() int64 {
		var n int64
		e.db.Unscoped().Model(&models.Contact{}).Where("id = ?", ca.ID).Count(&n)
		return n
	}())
}

func TestTrashedContactIsNotFoundBySearchAndActivities(t *testing.T) {
	e := newTrashEnv(t)
	e.routerA.GET("/activities", GetActivities)
	c := e.contact(t, e.userA.ID, "Zebulon")
	act := models.Activity{UserID: e.userA.ID, Title: "Picnic", Date: time.Now()}
	require.NoError(t, e.db.Create(&act).Error)
	require.NoError(t, e.db.Model(&act).Association("Contacts").Append(&c))

	search := func() int {
		w := doReq(e.routerA, "GET", "/activities?search=zebulon")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			Activities []models.Activity `json:"activities"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return len(resp.Activities)
	}
	assert.Equal(t, 1, search())
	doReq(e.routerA, "DELETE", idPath("/contacts/", c.ID, ""))
	assert.Equal(t, 0, search(), "activity search must not match a trashed contact's name")
	doReq(e.routerA, "POST", idPath("/trash/contacts/", c.ID, "/restore"))
	assert.Equal(t, 1, search())
}
