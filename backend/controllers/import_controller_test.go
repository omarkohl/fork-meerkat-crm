package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"meerkat/clock"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"meerkat/config"
	"meerkat/middleware"
	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func importRouter(t *testing.T) (*gorm.DB, *gin.Engine, models.User) {
	t.Helper()
	db, router := setupRouter()
	var me models.User
	require.NoError(t, db.First(&me).Error)
	cfg := &config.Config{ProfilePhotoDir: t.TempDir()}
	router.POST("/import/csv", UploadCSVForImport)
	router.POST("/import/vcf", func(c *gin.Context) { UploadVCFForImport(c, cfg) })
	router.POST("/import/preview", middleware.ValidateJSONMiddleware(&models.ImportPreviewRequest{}), PreviewImport)
	router.POST("/import/confirm", middleware.ValidateJSONMiddleware(&models.ImportConfirmRequest{}), ConfirmImport)
	return db, router, me
}

func upload(router *gin.Engine, path, filename, content string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", filename)
	_, _ = part.Write([]byte(content))
	_ = mw.Close()
	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestUploadCSVValidation(t *testing.T) {
	_, router, _ := importRouter(t)

	assert.Equal(t, http.StatusBadRequest, upload(router, "/import/csv", "contacts.exe", "a,b\n1,2\n").Code, "extension")
	assert.Equal(t, http.StatusBadRequest, upload(router, "/import/csv", "c.csv", "").Code, "empty")
	assert.Equal(t, http.StatusBadRequest, upload(router, "/import/csv", "c.csv", "only,a,header\n").Code, "no data rows")

	var many strings.Builder
	many.WriteString("First Name\n")
	for i := 0; i <= services.MaxCSVRows; i++ {
		fmt.Fprintf(&many, "p%d\n", i)
	}
	assert.Equal(t, http.StatusBadRequest, upload(router, "/import/csv", "c.csv", many.String()).Code, "row limit")

	// No file field at all.
	req := httptest.NewRequest("POST", "/import/csv", strings.NewReader("x"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = upload(router, "/import/csv", "C.CSV", "First Name,Email\nAda,ada@example.com\n")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp models.ImportUploadResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.SessionID)
	assert.Equal(t, 1, resp.RowCount)
	assert.Equal(t, "firstname", resp.SuggestedMappings[0].ContactField)
}

func TestImportSessionsBelongToTheirUser(t *testing.T) {
	db, router, _ := importRouter(t)
	w := upload(router, "/import/csv", "c.csv", "First Name\nAda\n")
	require.Equal(t, http.StatusOK, w.Code)
	var up models.ImportUploadResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &up))

	// A second user, same session id.
	other := models.User{Username: "other", Email: "o@example.com", Password: "x"}
	require.NoError(t, db.Create(&other).Error)
	gin.SetMode(gin.ReleaseMode)
	r2 := gin.New()
	r2.Use(func(c *gin.Context) { c.Set("db", db); c.Set("userID", other.ID); c.Next() })
	r2.POST("/import/preview", middleware.ValidateJSONMiddleware(&models.ImportPreviewRequest{}), PreviewImport)
	r2.POST("/import/confirm", middleware.ValidateJSONMiddleware(&models.ImportConfirmRequest{}), ConfirmImport)

	mapping := []models.ColumnMapping{{CSVColumn: "First Name", ContactField: "firstname"}}
	w = doJSON(r2, "POST", "/import/preview", models.ImportPreviewRequest{SessionID: up.SessionID, Mappings: mapping})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = doJSON(r2, "POST", "/import/confirm", models.ImportConfirmRequest{SessionID: up.SessionID, Actions: []models.RowImportAction{{RowIndex: 0, Action: "add"}}})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	var n int64
	db.Model(&models.Contact{}).Where("user_id = ?", other.ID).Count(&n)
	assert.Zero(t, n)

	// The owner can preview and import.
	w = doJSON(router, "POST", "/import/preview", models.ImportPreviewRequest{SessionID: up.SessionID, Mappings: mapping})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON(router, "POST", "/import/confirm", models.ImportConfirmRequest{SessionID: up.SessionID, Actions: []models.RowImportAction{{RowIndex: 0, Action: "add"}}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var created models.Contact
	require.NoError(t, db.Where("firstname = ?", "Ada").First(&created).Error)
	assert.NotEqual(t, other.ID, created.UserID)

	// Unknown session.
	w = doJSON(router, "POST", "/import/preview", models.ImportPreviewRequest{SessionID: "nope", Mappings: mapping})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUploadVCFValidation(t *testing.T) {
	_, router, _ := importRouter(t)
	assert.Equal(t, http.StatusBadRequest, upload(router, "/import/vcf", "c.txt", "BEGIN:VCARD\r\nEND:VCARD\r\n").Code)

	w := upload(router, "/import/vcf", "c.vcf", "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Ada Lovelace\r\nEMAIL:ada@example.com\r\nEND:VCARD\r\n")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp models.ImportPreviewResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.TotalRows)

	// Garbage is reported per row or as a client error, never as a server error.
	w = upload(router, "/import/vcf", "c.vcf", "this is not a vcard")
	assert.Less(t, w.Code, 500)
}

func TestSetImportClockIsUsedForSessionExpiry(t *testing.T) {
	fake := clock.NewFake(time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC))
	SetImportClock(fake)
	t.Cleanup(func() { SetImportClock(clock.Real()) })

	id := importSessions.CreateCSVSession(7, []string{"a"}, [][]string{{"x"}})
	fake.Advance(time.Hour)
	_, appErr := importSessions.PreviewCSV(nil, 7, models.ImportPreviewRequest{SessionID: id})
	require.NotNil(t, appErr)
	assert.Contains(t, appErr.Message, "expired")
}
