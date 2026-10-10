package photofile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/gen2brain/heic" // registers the HEIC decoder like the server does
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

func TestCheckDimensionsRefusesDecompressionBombs(t *testing.T) {
	// A valid 1x1 PNG whose header claims 20000x20000 pixels (CRC recomputed).
	var small bytes.Buffer
	require.NoError(t, png.Encode(&small, image.NewGray(image.Rect(0, 0, 1, 1))))
	data := small.Bytes()
	binary.BigEndian.PutUint32(data[16:], 20000)
	binary.BigEndian.PutUint32(data[20:], 20000)
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))

	assert.ErrorIs(t, CheckDimensions(data), ErrTooLarge)

	var ok bytes.Buffer
	require.NoError(t, png.Encode(&ok, image.NewGray(image.Rect(0, 0, 100, 100))))
	assert.NoError(t, CheckDimensions(ok.Bytes()))
	assert.NoError(t, CheckDimensions([]byte("not an image")), "unknown formats are left to the decoder")
}

func TestCheckDimensionsAcceptsPhoneSizes(t *testing.T) {
	// 50 MP (8160x6120) header, valid PNG otherwise.
	var small bytes.Buffer
	require.NoError(t, png.Encode(&small, image.NewGray(image.Rect(0, 0, 1, 1))))
	data := small.Bytes()
	binary.BigEndian.PutUint32(data[16:], 8160)
	binary.BigEndian.PutUint32(data[20:], 6120)
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	assert.NoError(t, CheckDimensions(data))
}

func TestCheckDimensionsRefusesUnreadableHeaders(t *testing.T) {
	assert.ErrorIs(t, CheckDimensions([]byte("\x89PNG\r\n\x1a\n\x00\x00")), ErrUnreadable)
	assert.ErrorIs(t, CheckDimensions([]byte{0xff, 0xd8, 0xff}), ErrUnreadable)
}

func TestCheckDimensionsReadsHEIC(t *testing.T) {
	// A valid HEIC photo (from the decoder's test data) must pass the fail-closed check.
	data, err := os.ReadFile("testdata/test8.heic")
	require.NoError(t, err)
	assert.NoError(t, CheckDimensions(data))
}
