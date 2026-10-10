package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"meerkat/middleware"
	"meerkat/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func adminRouter(t *testing.T) (*gorm.DB, *gin.Engine, models.User) {
	t.Helper()
	db, router := setupRouter()
	var me models.User
	require.NoError(t, db.First(&me).Error)
	grp := router.Group("/admin", middleware.AdminMiddleware())
	grp.GET("/users", ListUsers)
	grp.GET("/users/:id", GetUser)
	grp.PATCH("/users/:id", middleware.ValidateJSONMiddleware(&models.AdminUserUpdateInput{}), UpdateUser)
	grp.DELETE("/users/:id", DeleteUser)
	return db, router, me
}

func doJSON(router *gin.Engine, method, target string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAdminEndpointsRefuseNonAdmins(t *testing.T) {
	db, router, me := adminRouter(t)
	other := models.User{Username: "other", Email: "o@example.com", Password: "x"}
	require.NoError(t, db.Create(&other).Error)
	id := strconv.Itoa(int(other.ID))

	for _, tc := range []struct{ method, path string }{
		{"GET", "/admin/users"}, {"GET", "/admin/users/" + id},
		{"PATCH", "/admin/users/" + id}, {"DELETE", "/admin/users/" + id},
	} {
		w := doJSON(router, tc.method, tc.path, map[string]any{"is_admin": true})
		assert.Equal(t, http.StatusForbidden, w.Code, tc.method+" "+tc.path)
	}
	var reloaded models.User
	require.NoError(t, db.First(&reloaded, other.ID).Error)
	assert.False(t, reloaded.IsAdmin, "a non-admin cannot promote anyone")

	// After promotion the same calls work.
	require.NoError(t, db.Model(&me).Update("is_admin", true).Error)
	assert.Equal(t, http.StatusOK, doJSON(router, "GET", "/admin/users", nil).Code)
}

func TestAdminListUsersHidesSecrets(t *testing.T) {
	db, router, me := adminRouter(t)
	require.NoError(t, db.Model(&me).Update("is_admin", true).Error)
	w := doJSON(router, "GET", "/admin/users?limit=1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, strings.ToLower(w.Body.String()), "password")
	assert.Contains(t, w.Body.String(), `"total":1`)
}

func TestAdminUpdateUserGuards(t *testing.T) {
	db, router, me := adminRouter(t)
	require.NoError(t, db.Model(&me).Update("is_admin", true).Error)
	second := models.User{Username: "second", Email: "s@example.com", Password: "x", IsAdmin: true}
	plain := models.User{Username: "plain", Email: "p@example.com", Password: "x"}
	require.NoError(t, db.Create(&second).Error)
	require.NoError(t, db.Create(&plain).Error)
	path := func(u models.User) string { return "/admin/users/" + strconv.Itoa(int(u.ID)) }

	// Own admin status stays.
	w := doJSON(router, "PATCH", path(me), map[string]any{"is_admin": false})
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Demoting another admin is fine while one remains.
	w = doJSON(router, "PATCH", path(second), map[string]any{"is_admin": false})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Deleting yourself is refused; deleting the last admin cannot happen either.
	assert.Equal(t, http.StatusForbidden, doJSON(router, "DELETE", path(me), nil).Code)

	// Username and email are stored lowercase; a duplicate is a conflict.
	w = doJSON(router, "PATCH", path(plain), map[string]any{"username": "NewName", "email": "New@Example.COM"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var u models.User
	require.NoError(t, db.First(&u, plain.ID).Error)
	assert.Equal(t, "newname", u.Username)
	assert.Equal(t, "new@example.com", u.Email)
	w = doJSON(router, "PATCH", path(plain), map[string]any{"username": "tester"})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	// A password is stored hashed and weak ones are rejected by validation.
	w = doJSON(router, "PATCH", path(plain), map[string]any{"password": "short"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = doJSON(router, "PATCH", path(plain), map[string]any{"password": "A-Str0ng-Passw0rd!x"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&u, plain.ID).Error)
	assert.NotContains(t, u.Password, "Passw0rd")
	assert.NotEqual(t, "x", u.Password)

	// Unknown ids and malformed ids.
	assert.Equal(t, http.StatusNotFound, doJSON(router, "GET", "/admin/users/9999", nil).Code)
	assert.Equal(t, http.StatusBadRequest, doJSON(router, "GET", "/admin/users/abc", nil).Code)
}
