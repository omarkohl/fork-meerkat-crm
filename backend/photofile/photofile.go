// Package photofile writes profile photos with private permissions and reports
// every error of the write path, including the one from Close.
package photofile

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // registers the GIF decoder for image.DecodeConfig
	"image/jpeg"
	_ "image/png" // registers the PNG decoder for image.DecodeConfig
	"os"
)

// Modes of new photo directories and files. Existing directories are not changed.
const (
	DirMode  os.FileMode = 0o750
	FileMode os.FileMode = 0o640
)

// EnsureDir creates the photo directory (and parents) if it does not exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, DirMode)
}

// SaveJPEG encodes img as JPEG (quality 85) into a new file at path. It refuses an
// existing path (os.O_EXCL: no overwrite, no following of symlinks). A failed
// write, including a failed Sync or Close, returns the error and removes the
// partial file.
func SaveJPEG(path string, img image.Image) error {
	return write(path, func(f *os.File) error {
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
	})
}

func write(path string, fill func(*os.File) error) (err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FileMode) //nolint:gosec // path built from the configured photo directory
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			err = f.Sync()
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	return fill(f)
}

// MaxPixels bounds the size of an image that is decoded. Decoders allocate from the
// dimensions in the header, so a few hundred bytes can ask for gigabytes. 100 megapixels
// leaves room for the largest phone sensors (48 to 50 MP, 200 MP only as binned output)
// while an RGBA decode stays at 400 MB at most.
const MaxPixels = 100_000_000

// ErrTooLarge is returned for an image with more than MaxPixels pixels.
var ErrTooLarge = errors.New("image dimensions are too large")

// ErrUnreadable is returned when the header of a known image format cannot be read.
var ErrUnreadable = errors.New("image header cannot be read")

// CheckSize refuses dimensions over MaxPixels.
func CheckSize(width, height int) error {
	if int64(width)*int64(height) > MaxPixels {
		return ErrTooLarge
	}
	return nil
}

// CheckDimensions reads only the header of data and refuses images over MaxPixels.
// A header that cannot be read in a format with a registered decoder is refused too
// (fail closed: the decoder would fail on it as well). Data in a format without a
// registered decoder passes; the real decode reports that.
func CheckDimensions(data []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if errors.Is(err, image.ErrFormat) {
			return nil
		}
		return fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	return CheckSize(cfg.Width, cfg.Height)
}
