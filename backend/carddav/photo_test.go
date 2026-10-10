package carddav

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
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

func TestSaveContactPhotoSizes(t *testing.T) {
	tests := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"large square is downscaled to 400", 800, 800, 400, 400},
		{"large landscape is cropped square then downscaled", 1000, 600, 400, 400},
		{"small square keeps its size", 200, 200, 200, 200},
		{"small portrait is cropped to the short side", 100, 300, 100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tt.w, tt.h))
			fillWithCentre(img)
			var in bytes.Buffer
			if err := png.Encode(&in, img); err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			photo, thumb, err := SaveContactPhoto(in.Bytes(), "image/png", dir)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, photo))
			if err != nil {
				t.Fatal(err)
			}
			out, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if b := out.Bounds(); b.Dx() != tt.wantW || b.Dy() != tt.wantH {
				t.Errorf("photo is %dx%d, want %dx%d", b.Dx(), b.Dy(), tt.wantW, tt.wantH)
			}
			assertCentre(t, "photo", out)

			const prefix = "data:image/jpeg;base64,"
			if !strings.HasPrefix(thumb, prefix) {
				t.Fatalf("thumbnail is not a JPEG data URL: %.30s", thumb)
			}
			raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(thumb, prefix))
			if err != nil {
				t.Fatal(err)
			}
			th, err := jpeg.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if b := th.Bounds(); b.Dx() != 48 || b.Dy() != 48 {
				t.Errorf("thumbnail is %dx%d, want 48x48", b.Dx(), b.Dy())
			}
			assertCentre(t, "thumbnail", th)
		})
	}
}
