// Package photofile writes profile photos with private permissions and reports
// every error of the write path, including the one from Close.
package photofile

import (
	"image"
	"image/jpeg"
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
