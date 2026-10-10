package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"meerkat/config"
	"meerkat/models"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var (
	bgColour     = color.RGBA{R: 200, G: 40, B: 40, A: 255}
	centreColour = color.RGBA{R: 30, G: 160, B: 60, A: 255}
)

// fillWithCentre paints the background and a centred block of a distinct colour
// covering half the short side, so resizing mistakes (shifted or wrong crop) show.
func fillWithCentre(img *image.RGBA) {
	b := img.Bounds()
	half := min(b.Dx(), b.Dy()) / 4
	cx, cy := b.Dx()/2, b.Dy()/2
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := bgColour
			if x >= cx-half && x < cx+half && y >= cy-half && y < cy+half {
				c = centreColour
			}
			img.Set(x, y, c)
		}
	}
}

func assertCentre(t *testing.T, what string, img image.Image) {
	t.Helper()
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA()
	got := [3]int{int(r >> 8), int(g >> 8), int(bl >> 8)}
	want := [3]int{int(centreColour.R), int(centreColour.G), int(centreColour.B)}
	for i := range got {
		if d := got[i] - want[i]; d < -10 || d > 10 {
			t.Errorf("%s centre pixel is %v, want %v +-10", what, got, want)
			return
		}
	}
}

func pngFileHeader(t *testing.T, w, h int) *multipart.FileHeader {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillWithCentre(img)
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, img); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="photo"; filename="p.png"`)
	hdr.Set("Content-Type", "image/png")
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(imgBuf.Bytes())
	mw.Close()
	form, err := multipart.NewReader(&body, mw.Boundary()).ReadForm(10 << 20)
	if err != nil {
		t.Fatal(err)
	}
	return form.File["photo"][0]
}

func decodeJPEG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestProcessAndSavePhotoSizes(t *testing.T) {
	tests := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"large square is downscaled to 400", 800, 800, 400, 400},
		{"large landscape is cropped square then downscaled", 1000, 600, 400, 400},
		{"large portrait is cropped square then downscaled", 500, 900, 400, 400},
		{"small square keeps its size", 200, 200, 200, 200},
		{"small landscape is cropped to the short side", 300, 100, 100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			photo, thumb, err := processAndSavePhoto(pngFileHeader(t, tt.w, tt.h), dir)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, photo))
			if err != nil {
				t.Fatal(err)
			}
			photoImg := decodeJPEG(t, data)
			if b := photoImg.Bounds(); b.Dx() != tt.wantW || b.Dy() != tt.wantH {
				t.Errorf("photo is %dx%d, want %dx%d", b.Dx(), b.Dy(), tt.wantW, tt.wantH)
			}
			assertCentre(t, "photo", photoImg)

			const prefix = "data:image/jpeg;base64,"
			if !strings.HasPrefix(thumb, prefix) {
				t.Fatalf("thumbnail is not a JPEG data URL: %.30s", thumb)
			}
			raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(thumb, prefix))
			if err != nil {
				t.Fatal(err)
			}
			thumbImg := decodeJPEG(t, raw)
			if b := thumbImg.Bounds(); b.Dx() != 48 || b.Dy() != 48 {
				t.Errorf("thumbnail is %dx%d, want 48x48", b.Dx(), b.Dy())
			}
			assertCentre(t, "thumbnail", thumbImg)
		})
	}
}

func TestProcessAndSavePhotoUsesPrivateModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "photos")
	photo, _, err := processAndSavePhoto(pngFileHeader(t, 50, 50), dir)
	if err != nil {
		t.Fatal(err)
	}
	// Written through photofile: private modes, not the process default.
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, photo))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm()&0o007 != 0 || fi.Mode().Perm()&0o137 != 0 {
		t.Errorf("dir %v, file %v: others must have no access, file must not be executable or group-writable", di.Mode().Perm(), fi.Mode().Perm())
	}
}

func photoRequest(t *testing.T, router *gin.Engine, contactID uint, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("photo", filename)
	require.NoError(t, err)
	_, _ = part.Write(content)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest("POST", "/contacts/"+strconv.Itoa(int(contactID))+"/photo", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAddPhotoValidation(t *testing.T) {
	db, router := setupRouter()
	cfg := &config.Config{ProfilePhotoDir: t.TempDir()}
	router.POST("/contacts/:id/photo", func(c *gin.Context) { AddPhotoToContact(c, cfg) })
	var me, other models.User
	require.NoError(t, db.First(&me).Error)
	other = models.User{Username: "other", Email: "o@example.com", Password: "x"}
	require.NoError(t, db.Create(&other).Error)
	mine := models.Contact{UserID: me.ID, Firstname: "A"}
	theirs := models.Contact{UserID: other.ID, Firstname: "B"}
	require.NoError(t, db.Create(&mine).Error)
	require.NoError(t, db.Create(&theirs).Error)

	var good bytes.Buffer
	require.NoError(t, png.Encode(&good, image.NewRGBA(image.Rect(0, 0, 10, 10))))

	// Input the user can fix is a client error, not a server error.
	for name, content := range map[string][]byte{
		"text file":       []byte("this is not an image at all"),
		"empty file":      {},
		"html with image": []byte("<html><script>alert(1)</script></html>"),
		"truncated png":   good.Bytes()[:40],
	} {
		w := photoRequest(t, router, mine.ID, "p.png", content)
		assert.Equal(t, http.StatusBadRequest, w.Code, name+": "+w.Body.String())
	}

	// A header that claims a gigantic image is refused before anything is allocated.
	bomb := append([]byte{}, good.Bytes()...)
	binary.BigEndian.PutUint32(bomb[16:], 30000)
	binary.BigEndian.PutUint32(bomb[20:], 30000)
	binary.BigEndian.PutUint32(bomb[29:], crc32.ChecksumIEEE(bomb[12:29]))
	w := photoRequest(t, router, mine.ID, "p.png", bomb)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// Other users' contacts are not found, and nothing was written for rejected uploads.
	w = photoRequest(t, router, theirs.ID, "p.png", good.Bytes())
	assert.Equal(t, http.StatusNotFound, w.Code)
	entries, _ := os.ReadDir(cfg.ProfilePhotoDir)
	assert.Empty(t, entries)

	// A good upload works and is stored as JPEG under a generated name.
	w = photoRequest(t, router, mine.ID, "../../evil.php", good.Bytes())
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	entries, _ = os.ReadDir(cfg.ProfilePhotoDir)
	require.Len(t, entries, 1)
	assert.True(t, strings.HasSuffix(entries[0].Name(), "_photo.jpg"))
}

func TestGetProfilePictureIsScopedAndSafe(t *testing.T) {
	db, router := setupRouter()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("jpeg-bytes"), 0o600))
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("secret"), 0o600))
	cfg := &config.Config{ProfilePhotoDir: dir}
	router.GET("/contacts/:id/photo", func(c *gin.Context) { GetProfilePicture(c, cfg) })

	var me models.User
	require.NoError(t, db.First(&me).Error)
	other := models.User{Username: "other", Email: "o@example.com", Password: "x"}
	require.NoError(t, db.Create(&other).Error)
	mine := models.Contact{UserID: me.ID, Firstname: "A", Photo: "a.jpg"}
	theirs := models.Contact{UserID: other.ID, Firstname: "B", Photo: "a.jpg"}
	traversal := models.Contact{UserID: me.ID, Firstname: "C", Photo: "../secret.txt"}
	absolute := models.Contact{UserID: me.ID, Firstname: "D", Photo: secret}
	for _, c := range []*models.Contact{&mine, &theirs, &traversal, &absolute} {
		require.NoError(t, db.Create(c).Error)
	}
	fetch := func(id uint) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/contacts/"+strconv.Itoa(int(id))+"/photo", nil))
		return w
	}

	assert.Equal(t, "jpeg-bytes", fetch(mine.ID).Body.String())
	assert.Equal(t, http.StatusNotFound, fetch(theirs.ID).Code, "another user's photo")
	for _, id := range []uint{traversal.ID, absolute.ID} {
		w := fetch(id)
		assert.NotEqual(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "secret")
	}
}

func fileHeaderFromBytes(t *testing.T, name string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="photo"; filename="`+name+`"`)
	part, err := mw.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = part.Write(content)
	require.NoError(t, mw.Close())
	form, err := multipart.NewReader(&body, mw.Boundary()).ReadForm(10 << 20)
	require.NoError(t, err)
	return form.File["photo"][0]
}

// Valid HEIC photos (brands heic and mif1) must still be accepted by the fail-closed
// header check; a truncated one is refused.
func TestProcessAndSavePhotoHEIC(t *testing.T) {
	data, err := os.ReadFile("testdata/test8.heic")
	require.NoError(t, err)
	mif1 := append([]byte(nil), data...)
	copy(mif1[8:12], "mif1")

	for name, content := range map[string][]byte{"heic": data, "mif1": mif1} {
		t.Run(name, func(t *testing.T) {
			_, _, err := processAndSavePhoto(fileHeaderFromBytes(t, "p.heic", content), t.TempDir())
			require.NoError(t, err)
		})
	}
	_, _, err = processAndSavePhoto(fileHeaderFromBytes(t, "p.heic", data[:40]), t.TempDir())
	assert.ErrorIs(t, err, errInvalidPhoto)
}
