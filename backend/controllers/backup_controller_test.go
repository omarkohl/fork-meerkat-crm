package controllers

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meerkat/config"
	"meerkat/middleware"
	"meerkat/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type backupEnv struct {
	db     *gorm.DB
	dir    string
	photos string
	admin  *gin.Engine
	user   *gin.Engine
}

func newBackupEnv(t *testing.T) *backupEnv {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Contact{}))
	admin := models.User{Username: "root", Email: "root@example.com", Password: "x", IsAdmin: true}
	user := models.User{Username: "plain", Email: "plain@example.com", Password: "x"}
	require.NoError(t, db.Create(&admin).Error)
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&models.Contact{UserID: user.ID, Firstname: "Secret"}).Error)

	e := &backupEnv{db: db, dir: filepath.Join(t.TempDir(), "backups"), photos: t.TempDir()}
	require.NoError(t, os.WriteFile(filepath.Join(e.photos, "p.jpg"), []byte("img"), 0o600))
	mk := func(uid uint) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("db", db)
			c.Set("userID", uid)
			c.Set("cfg", config.Config{BackupDir: e.dir, ProfilePhotoDir: e.photos, BackupEnabled: true, BackupTime: "02:30", BackupRetentionCount: 2})
			c.Next()
		})
		g := r.Group("/admin", middleware.AdminMiddleware())
		g.GET("/backups", ListBackups)
		g.POST("/backups", CreateBackup)
		g.GET("/backups/:name", DownloadBackup)
		g.DELETE("/backups/:name", DeleteBackup)
		return r
	}
	e.admin, e.user = mk(admin.ID), mk(user.ID)
	return e
}

func (e *backupEnv) do(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestBackupEndpointsRejectNonAdmins(t *testing.T) {
	e := newBackupEnv(t)
	created := e.do(e.admin, "POST", "/admin/backups")
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var b struct{ Name string }
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &b))

	for _, tc := range []struct{ method, path string }{
		{"GET", "/admin/backups"},
		{"POST", "/admin/backups"},
		{"GET", "/admin/backups/" + b.Name},
		{"DELETE", "/admin/backups/" + b.Name},
	} {
		w := e.do(e.user, tc.method, tc.path)
		assert.Equal(t, http.StatusForbidden, w.Code, "%s %s", tc.method, tc.path)
		assert.NotContains(t, w.Body.String(), "Secret")
	}
	list, err := os.ReadDir(e.dir)
	require.NoError(t, err)
	assert.Len(t, list, 1, "a non-admin neither created nor deleted anything")
}

func TestBackupCreateListDownloadDelete(t *testing.T) {
	e := newBackupEnv(t)

	w := e.do(e.admin, "GET", "/admin/backups")
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Backups []struct {
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			CreatedAt string `json:"created_at"`
		} `json:"backups"`
		Enabled        bool   `json:"enabled"`
		Time           string `json:"time"`
		RetentionCount int    `json:"retention_count"`
		Directory      string `json:"directory"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Empty(t, list.Backups)
	assert.True(t, list.Enabled)
	assert.Equal(t, "02:30", list.Time)
	assert.Equal(t, 2, list.RetentionCount)
	assert.Equal(t, e.dir, list.Directory)

	w = e.do(e.admin, "POST", "/admin/backups")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Regexp(t, `^meerkat-backup-\d{8}-\d{6}(-\d+)?\.tar\.gz$`, created.Name)
	assert.Positive(t, created.Size)

	w = e.do(e.admin, "GET", "/admin/backups/"+created.Name)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), created.Name)
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	gz, err := gzip.NewReader(w.Body)
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		names = append(names, h.Name)
	}
	assert.ElementsMatch(t, []string{"meerkat.db", "photos/p.jpg"}, names)

	w = e.do(e.admin, "DELETE", "/admin/backups/"+created.Name)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, http.StatusNotFound, e.do(e.admin, "GET", "/admin/backups/"+created.Name).Code)
	assert.Equal(t, http.StatusNotFound, e.do(e.admin, "DELETE", "/admin/backups/"+created.Name).Code)
}

func TestBackupCreateKeepsOnlyRetentionCount(t *testing.T) {
	e := newBackupEnv(t)
	for i := 0; i < 3; i++ {
		// Distinct names within the same second get a suffix; all three must exist before pruning.
		require.Equal(t, http.StatusCreated, e.do(e.admin, "POST", "/admin/backups").Code)
	}
	entries, err := os.ReadDir(e.dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestBackupRejectsPathTraversalAndForeignNames(t *testing.T) {
	e := newBackupEnv(t)
	outside := filepath.Join(filepath.Dir(e.dir), "secret.tar.gz")
	require.NoError(t, os.MkdirAll(e.dir, 0o700))
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, "notes.txt"), []byte("n"), 0o600))

	for _, name := range []string{"..", "..%2Fsecret.tar.gz", "%2e%2e%2fsecret.tar.gz", "secret.tar.gz", "notes.txt", "meerkat-backup-1.tar.gz"} {
		for _, method := range []string{"GET", "DELETE"} {
			w := e.do(e.admin, method, "/admin/backups/"+name)
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, w.Code, "%s %s", method, name)
			assert.False(t, strings.Contains(w.Body.String(), "secret"), "%s %s", method, name)
		}
	}
	assert.FileExists(t, outside)
	assert.FileExists(t, filepath.Join(e.dir, "notes.txt"))
}

// deadlineWriter records SetWriteDeadline calls, which httptest recorders do not support.
type deadlineWriter struct {
	gin.ResponseWriter
	deadlines *[]time.Time
}

func (w deadlineWriter) SetWriteDeadline(t time.Time) error {
	*w.deadlines = append(*w.deadlines, t)
	return nil
}

// Big backups take longer than the server's write timeout (default 15 s), so
// creating and downloading one must lift it for that request.
func TestBackupCreateAndDownloadLiftWriteDeadline(t *testing.T) {
	e := newBackupEnv(t)
	var deadlines []time.Time
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Writer = deadlineWriter{ResponseWriter: c.Writer, deadlines: &deadlines}
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		c.Set("db", e.db)
		c.Set("cfg", config.Config{BackupDir: e.dir, ProfilePhotoDir: e.photos, BackupRetentionCount: 2})
	})
	r.POST("/backups", CreateBackup)
	r.GET("/backups/:name", DownloadBackup)

	w := e.do(r, "POST", "/backups")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Len(t, deadlines, 1)
	assert.True(t, deadlines[0].IsZero(), "create removes the write deadline")

	var b struct{ Name string }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	w = e.do(r, "GET", "/backups/"+b.Name)
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, deadlines, 2)
	assert.True(t, deadlines[1].IsZero(), "download removes the write deadline")
}
