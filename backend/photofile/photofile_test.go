package photofile

import (
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	return img
}

func TestEnsureDirCreatesPrivateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "photos")
	require.NoError(t, EnsureDir(dir))
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	if runtime.GOOS != "windows" {
		assert.Zero(t, info.Mode().Perm()&0o007, "others must have no access")
	}
}

func TestEnsureDirLeavesExistingDirectoryAlone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o755))
	require.NoError(t, EnsureDir(dir))
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestSaveJPEGWritesFileWithPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.jpg")
	require.NoError(t, SaveJPEG(path, testImage()))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Positive(t, info.Size())
	assert.Zero(t, info.Mode().Perm()&0o137, "no execute, no group write, no other access")
}

func TestWriteRemovesPartialFileOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jpg")
	err := write(path, func(f *os.File) error {
		_, _ = f.WriteString("partial")
		return errors.New("encode failed")
	})
	require.EqualError(t, err, "encode failed")
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "a failed write must not leave a file behind")
}

func TestSaveJPEGReportsCreateError(t *testing.T) {
	err := SaveJPEG(filepath.Join(t.TempDir(), "missing", "p.jpg"), testImage())
	assert.Error(t, err)
}

func TestWriteReturnsCloseError(t *testing.T) {
	// A file closed before Write's own Close makes that Close fail.
	path := filepath.Join(t.TempDir(), "c.jpg")
	err := write(path, func(f *os.File) error {
		return f.Close() // the deferred-style close in write must now report an error
	})
	require.Error(t, err)
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr))
}

func TestSaveJPEGRefusesExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.jpg")
	require.NoError(t, os.WriteFile(path, []byte("keep"), 0o600))
	require.Error(t, SaveJPEG(path, testImage()))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(data), "an existing file must be neither overwritten nor removed")
}
