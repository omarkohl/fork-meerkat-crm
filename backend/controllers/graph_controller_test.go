package controllers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"meerkat/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGraphEdgesOnlyConnectVisibleNodes(t *testing.T) {
	db, router := setupRouter()
	router.GET("/graph", GetGraph)
	var me models.User
	require.NoError(t, db.First(&me).Error)
	other := models.User{Username: "other", Email: "o@example.com", Password: "x"}
	require.NoError(t, db.Create(&other).Error)

	a := models.Contact{UserID: me.ID, Firstname: "Ann"}
	b := models.Contact{UserID: me.ID, Firstname: "Bob"}
	archived := models.Contact{UserID: me.ID, Firstname: "Old", Archived: true}
	foreign := models.Contact{UserID: other.ID, Firstname: "Zed"}
	for _, c := range []*models.Contact{&a, &b, &archived, &foreign} {
		require.NoError(t, db.Create(c).Error)
	}
	rel := func(from, to uint) {
		require.NoError(t, db.Create(&models.Relationship{UserID: me.ID, ContactID: from, RelatedContactID: &to, Name: "n", Type: "Friend"}).Error)
	}
	rel(a.ID, b.ID)
	rel(a.ID, archived.ID) // archived contacts are not drawn, so neither is the edge

	// Two visible people make an activity node; a visible and an archived one do not.
	both := models.Activity{UserID: me.ID, Title: "Dinner", Date: time.Now(), Contacts: []models.Contact{a, b}}
	mixed := models.Activity{UserID: me.ID, Title: "Hike", Date: time.Now(), Contacts: []models.Contact{a, archived}}
	require.NoError(t, db.Create(&both).Error)
	require.NoError(t, db.Create(&mixed).Error)

	w := doJSON(router, "GET", "/graph", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp models.GraphResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	ids := map[string]bool{}
	for _, n := range resp.Nodes {
		ids[n.ID] = true
	}
	assert.True(t, ids["c-"+itoa(a.ID)] && ids["c-"+itoa(b.ID)])
	assert.False(t, ids["c-"+itoa(archived.ID)])
	assert.False(t, ids["c-"+itoa(foreign.ID)], "other users' contacts never appear")
	assert.True(t, ids["a-"+itoa(both.ID)])
	for _, e := range resp.Edges {
		assert.True(t, ids[e.Source], "edge %s has no source node %s", e.ID, e.Source)
		assert.True(t, ids[e.Target], "edge %s has no target node %s", e.ID, e.Target)
	}
	assert.False(t, ids["a-"+itoa(mixed.ID)], "an activity with one visible participant is not a node")
}
