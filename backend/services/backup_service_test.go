package services

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"meerkat/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newWALDB opens a file-backed database in WAL mode, like the app does.
func newWALDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "meerkat.db")
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Contact{}))
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db, path
}

// readBackup unpacks a backup archive into a map of entry name to content.
func readBackup(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(tr)
		require.NoError(t, err)
		out[h.Name] = data
	}
	return out
}

func TestCreateBackupSnapshotsWALDatabaseAndPhotos(t *testing.T) {
	db, dbPath := newWALDB(t)
	u1 := models.User{Username: "a", Password: "password123!A", Email: "a@example.com"}
	u2 := models.User{Username: "b", Password: "password123!A", Email: "b@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	require.NoError(t, db.Create(&u2).Error)
	require.NoError(t, db.Create(&models.Contact{UserID: u1.ID, Firstname: "Alice"}).Error)
	require.NoError(t, db.Create(&models.Contact{UserID: u2.ID, Firstname: "Bob"}).Error)
	// The new rows live in the -wal file; a plain copy of the main file would miss them.
	info, err := os.Stat(dbPath + "-wal")
	require.NoError(t, err)
	require.Positive(t, info.Size())

	photos := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(photos, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(photos, "a.jpg"), []byte("photo-a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(photos, "sub", "b.jpg"), []byte("photo-b"), 0o644))
	outside := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(photos, "link.jpg")))

	dir := filepath.Join(t.TempDir(), "backups")
	b, err := CreateBackup(db, dir, photos, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, "meerkat-backup-20300102-030405.tar.gz", b.Name)
	assert.Positive(t, b.Size)

	// Private permissions: personal data of every user is in there.
	if runtime.GOOS != "windows" {
		dirInfo, err := os.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
		fileInfo, err := os.Stat(filepath.Join(dir, b.Name))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "temporary files must not be left behind")

	files := readBackup(t, filepath.Join(dir, b.Name))
	assert.Equal(t, []byte("photo-a"), files["photos/a.jpg"])
	assert.Equal(t, []byte("photo-b"), files["photos/sub/b.jpg"])
	assert.NotContains(t, files, "photos/link.jpg", "symlinks are not followed")
	require.Contains(t, files, "meerkat.db")

	restored := filepath.Join(t.TempDir(), "restored.db")
	require.NoError(t, os.WriteFile(restored, files["meerkat.db"], 0o600))
	rdb, err := gorm.Open(sqlite.Open(restored), &gorm.Config{})
	require.NoError(t, err)
	defer func() { sqlDB, _ := rdb.DB(); _ = sqlDB.Close() }()
	var check string
	require.NoError(t, rdb.Raw("PRAGMA integrity_check").Scan(&check).Error)
	assert.Equal(t, "ok", check)
	var users, contacts int64
	require.NoError(t, rdb.Model(&models.User{}).Count(&users).Error)
	require.NoError(t, rdb.Model(&models.Contact{}).Count(&contacts).Error)
	assert.EqualValues(t, 2, users, "all users' data is in the backup")
	assert.EqualValues(t, 2, contacts)
}

func TestCreateBackupWithoutPhotoDir(t *testing.T) {
	db, _ := newWALDB(t)
	dir := t.TempDir()
	for _, photoDir := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		b, err := CreateBackup(db, dir, photoDir, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
		require.NoError(t, err)
		files := readBackup(t, filepath.Join(dir, b.Name))
		assert.Contains(t, files, "meerkat.db")
		require.NoError(t, DeleteBackup(dir, b.Name))
	}
}

func TestCreateBackupSkipsBackupDirInsidePhotoDir(t *testing.T) {
	db, _ := newWALDB(t)
	photos := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(photos, "a.jpg"), []byte("x"), 0o600))
	dir := filepath.Join(photos, "backups")
	_, err := CreateBackup(db, dir, photos, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	b, err := CreateBackup(db, dir, photos, time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	for name := range readBackup(t, filepath.Join(dir, b.Name)) {
		assert.NotContains(t, name, "backups", "earlier backups must not be nested")
	}
}

func TestCreateBackupSameSecondDoesNotOverwrite(t *testing.T) {
	db, _ := newWALDB(t)
	dir := t.TempDir()
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	a, err := CreateBackup(db, dir, "", now)
	require.NoError(t, err)
	b, err := CreateBackup(db, dir, "", now)
	require.NoError(t, err)
	assert.NotEqual(t, a.Name, b.Name)
	list, err := ListBackups(dir)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestListBackupsNewestFirstIgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{
		"meerkat-backup-20300101-000000.tar.gz",
		"meerkat-backup-20300103-000000.tar.gz",
		"meerkat-backup-20300102-000000.tar.gz",
		"notes.txt",
		"meerkat-backup-20300104-000000.tar.gz.tmp",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600))
	}
	list, err := ListBackups(dir)
	require.NoError(t, err)
	var names []string
	for _, b := range list {
		names = append(names, b.Name)
	}
	assert.Equal(t, []string{
		"meerkat-backup-20300103-000000.tar.gz",
		"meerkat-backup-20300102-000000.tar.gz",
		"meerkat-backup-20300101-000000.tar.gz",
	}, names)
	assert.Equal(t, time.Date(2030, 1, 3, 0, 0, 0, 0, time.UTC), list[0].CreatedAt.UTC())
}

func TestListBackupsMissingDirIsEmpty(t *testing.T) {
	list, err := ListBackups(filepath.Join(t.TempDir(), "nope"))
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestCreateBackupPrunesOldestBeyondRetention(t *testing.T) {
	db, _ := newWALDB(t)
	dir := t.TempDir()
	other := filepath.Join(dir, "keep-me.txt")
	require.NoError(t, os.WriteFile(other, []byte("x"), 0o600))
	for day := 1; day <= 5; day++ {
		_, err := CreateBackup(db, dir, "", time.Date(2030, 1, day, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
	}
	removed, err := PruneBackups(dir, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, removed)
	list, err := ListBackups(dir)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "meerkat-backup-20300105-000000.tar.gz", list[0].Name)
	assert.Equal(t, "meerkat-backup-20300104-000000.tar.gz", list[1].Name)
	assert.FileExists(t, other, "files that are not backups are never touched")

	removed, err = PruneBackups(dir, 0)
	require.NoError(t, err)
	assert.Zero(t, removed, "a retention below one keeps everything")
}

func TestRunScheduledBackupCreatesAndPrunes(t *testing.T) {
	db, _ := newWALDB(t)
	dir := t.TempDir()
	for day := 1; day <= 3; day++ {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "meerkat-backup-2020010"+string(rune('0'+day))+"-000000.tar.gz"), []byte("x"), 0o600))
	}
	b, err := RunScheduledBackup(db, dir, "", 2, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
	require.NoError(t, err)
	list, err := ListBackups(dir)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, b.Name, list[0].Name)
}

func TestBackupNamesAreValidatedStrictly(t *testing.T) {
	dir := t.TempDir()
	good := "meerkat-backup-20300102-030405.tar.gz"
	require.NoError(t, os.WriteFile(filepath.Join(dir, good), []byte("x"), 0o600))
	secret := filepath.Join(filepath.Dir(dir), "secret.tar.gz")
	require.NoError(t, os.WriteFile(secret, []byte("s"), 0o600))

	p, err := BackupPath(dir, good)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, good), p)
	p, err = BackupPath(dir, "meerkat-backup-20300102-030405-2.tar.gz")
	require.NoError(t, err, "the same-second suffix is a valid name")
	assert.NotEmpty(t, p)

	for _, bad := range []string{
		"", ".", "..", "../secret.tar.gz", "/etc/passwd", "sub/" + good, "..\\" + good,
		good + "/..", good + "\x00.txt", good + ".tmp", "secret.tar.gz", "meerkat.db",
		"meerkat-backup-2030010-030405.tar.gz", "meerkat-backup-20300102-030405.tar.gz\n",
		"MEERKAT-BACKUP-20300102-030405.tar.gz", "meerkat-backup-20300102-030405.tar.gz ",
	} {
		_, err := BackupPath(dir, bad)
		assert.ErrorIs(t, err, ErrInvalidBackupName, "name %q must be rejected", bad)
		assert.ErrorIs(t, DeleteBackup(dir, bad), ErrInvalidBackupName, "delete %q", bad)
	}
	assert.FileExists(t, secret)
	assert.FileExists(t, filepath.Join(dir, good))
}

func TestDeleteBackup(t *testing.T) {
	dir := t.TempDir()
	good := "meerkat-backup-20300102-030405.tar.gz"
	require.NoError(t, os.WriteFile(filepath.Join(dir, good), []byte("x"), 0o600))
	require.NoError(t, DeleteBackup(dir, good))
	assert.NoFileExists(t, filepath.Join(dir, good))
	assert.ErrorIs(t, DeleteBackup(dir, good), ErrBackupNotFound)
}

func TestCreateBackupRemovesStaleTempDirs(t *testing.T) {
	db, _ := newWALDB(t)
	dir := filepath.Join(t.TempDir(), "backups")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tmp-crashed"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tmp-crashed", "meerkat.db"), []byte("copy of all data"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o600))

	_, err := CreateBackup(db, dir, "", time.Now())
	require.NoError(t, err)

	assert.NoDirExists(t, filepath.Join(dir, ".tmp-crashed"))
	assert.FileExists(t, filepath.Join(dir, "notes.txt"), "files that are not ours stay")
}

func TestCreateBackupTightensExistingDirMode(t *testing.T) {
	db, _ := newWALDB(t)
	dir := filepath.Join(t.TempDir(), "backups")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.Chmod(dir, 0o755))

	_, err := CreateBackup(db, dir, "", time.Now())
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}
