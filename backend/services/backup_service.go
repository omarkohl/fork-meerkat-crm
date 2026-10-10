package services

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	// DefaultBackupRetention is how many backups are kept when nothing is configured.
	DefaultBackupRetention = 7

	backupTimeLayout = "20060102-150405"
	backupDBName     = "meerkat.db"
	backupPhotosDir  = "photos"
)

// ErrInvalidBackupName means the name is not one this service ever creates.
var ErrInvalidBackupName = errors.New("invalid backup name")

// ErrBackupNotFound means no backup with that (valid) name exists.
var ErrBackupNotFound = errors.New("backup not found")

// backupNamePattern is the only shape of file name that is listed, served or
// deleted. It has no path separators, dots other than the extension, or room
// for anything but digits, so a name that matches cannot leave the backup directory.
var backupNamePattern = regexp.MustCompile(`^meerkat-backup-(\d{8}-\d{6})(?:-\d{1,3})?\.tar\.gz$`)

// backupMu serialises creating and pruning backups.
var backupMu sync.Mutex

// BackupInfo describes one backup file.
type BackupInfo struct {
	Name      string
	Size      int64
	CreatedAt time.Time
}

// CreateBackup writes one backup archive into backupDir: a consistent snapshot of
// the whole database (all users) taken with VACUUM INTO, which is safe while the
// database is in WAL mode and in use, plus the files under photoDir. The archive
// is a .tar.gz holding "meerkat.db" and "photos/...". The directory is created
// private (0700) and the archive is 0600, because it holds every user's data.
func CreateBackup(db *gorm.DB, backupDir, photoDir string, now time.Time) (BackupInfo, error) {
	backupMu.Lock()
	defer backupMu.Unlock()

	if err := ensurePrivateDir(backupDir); err != nil {
		return BackupInfo{}, fmt.Errorf("failed to prepare backup directory: %w", err)
	}
	removeStaleTempDirs(backupDir)
	work, err := os.MkdirTemp(backupDir, ".tmp-")
	if err != nil {
		return BackupInfo{}, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(work) }()

	snapshot := filepath.Join(work, backupDBName)
	if err := db.Exec("VACUUM INTO ?", snapshot).Error; err != nil {
		return BackupInfo{}, fmt.Errorf("failed to snapshot database: %w", err)
	}
	archive := filepath.Join(work, "backup.tar.gz")
	if err := writeBackupArchive(archive, snapshot, photoDir, backupDir, now); err != nil {
		return BackupInfo{}, err
	}

	stamp := now.UTC().Format(backupTimeLayout)
	name := fmt.Sprintf("meerkat-backup-%s.tar.gz", stamp)
	for i := 2; ; i++ {
		if _, err := os.Lstat(filepath.Join(backupDir, name)); errors.Is(err, fs.ErrNotExist) {
			break
		} else if err != nil {
			return BackupInfo{}, err
		}
		if i > 999 {
			return BackupInfo{}, errors.New("too many backups in the same second")
		}
		name = fmt.Sprintf("meerkat-backup-%s-%d.tar.gz", stamp, i)
	}
	final := filepath.Join(backupDir, name)
	if err := os.Rename(archive, final); err != nil {
		return BackupInfo{}, fmt.Errorf("failed to store backup: %w", err)
	}
	info, err := os.Stat(final)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{Name: name, Size: info.Size(), CreatedAt: now.UTC().Truncate(time.Second)}, nil
}

// removeStaleTempDirs deletes ".tmp-*" work directories left behind by a crash
// during an earlier backup; each may hold a full copy of the database. The
// caller holds backupMu, so none of them is in use.
func removeStaleTempDirs(backupDir string) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), ".tmp-") {
			_ = os.RemoveAll(filepath.Join(backupDir, e.Name()))
		}
	}
}

// ensurePrivateDir creates dir with mode 0700, or tightens an existing one to
// 0700: it holds every user's data, whatever mode it was created with.
func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700) //nolint:gosec // a directory needs the x bit; this is the private mode
}

func writeBackupArchive(dest, snapshot, photoDir, backupDir string, now time.Time) (err error) {
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // path built from our own temp dir
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	if err := addFileToTar(tw, snapshot, backupDBName, now); err != nil {
		return fmt.Errorf("failed to archive database: %w", err)
	}
	if photoDir != "" {
		if err := addPhotosToTar(tw, photoDir, backupDir, now); err != nil {
			return fmt.Errorf("failed to archive photos: %w", err)
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return out.Sync()
}

func addFileToTar(tw *tar.Writer, path, name string, now time.Time) error {
	f, err := os.Open(path) //nolint:gosec // caller passes paths it built or walked
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: info.Size(), ModTime: now.UTC()}); err != nil {
		return err
	}
	_, err = io.CopyN(tw, f, info.Size())
	return err
}

// addPhotosToTar adds the regular files under photoDir as "photos/<relative path>".
// Symlinks are skipped (they could point outside the directory) and so is the
// backup directory itself when it lives inside the photo directory.
func addPhotosToTar(tw *tar.Writer, photoDir, backupDir string, now time.Time) error {
	root, err := filepath.Abs(photoDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	skip, _ := filepath.Abs(backupDir)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil // removed while walking
			}
			return err
		}
		if d.IsDir() {
			if path == skip {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if err := addFileToTar(tw, path, backupPhotosDir+"/"+filepath.ToSlash(rel), now); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		return nil
	})
}

// ListBackups returns the backups in backupDir, newest first. A missing
// directory means no backups. Files that are not backups are ignored.
func ListBackups(backupDir string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(backupDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []BackupInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	backups := []BackupInfo{}
	for _, e := range entries {
		m := backupNamePattern.FindStringSubmatch(e.Name())
		if m == nil || !e.Type().IsRegular() {
			continue
		}
		created, err := time.ParseInLocation(backupTimeLayout, m[1], time.UTC)
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, BackupInfo{Name: e.Name(), Size: info.Size(), CreatedAt: created})
	}
	// Same-second backups carry a numeric suffix; the longer name is the later one.
	sort.Slice(backups, func(i, j int) bool {
		if !backups[i].CreatedAt.Equal(backups[j].CreatedAt) {
			return backups[i].CreatedAt.After(backups[j].CreatedAt)
		}
		if len(backups[i].Name) != len(backups[j].Name) {
			return len(backups[i].Name) > len(backups[j].Name)
		}
		return backups[i].Name > backups[j].Name
	})
	return backups, nil
}

// BackupPath returns the path of the named backup inside backupDir after checking
// that the name is exactly of the shape this service creates. It does not check
// that the file exists.
func BackupPath(backupDir, name string) (string, error) {
	if !backupNamePattern.MatchString(name) {
		return "", ErrInvalidBackupName
	}
	return filepath.Join(backupDir, name), nil
}

// DeleteBackup removes one backup.
func DeleteBackup(backupDir, name string) error {
	path, err := BackupPath(backupDir, name)
	if err != nil {
		return err
	}
	backupMu.Lock()
	defer backupMu.Unlock()
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrBackupNotFound
		}
		return err
	}
	return nil
}

// PruneBackups deletes all but the keep newest backups and returns how many it
// removed. A keep below one keeps everything.
func PruneBackups(backupDir string, keep int) (int, error) {
	if keep < 1 {
		return 0, nil
	}
	backupMu.Lock()
	defer backupMu.Unlock()
	backups, err := ListBackups(backupDir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for i := keep; i < len(backups); i++ {
		path, err := BackupPath(backupDir, backups[i].Name)
		if err != nil {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// RunScheduledBackup creates a backup and then prunes down to keep backups.
func RunScheduledBackup(db *gorm.DB, backupDir, photoDir string, keep int, now time.Time) (BackupInfo, error) {
	b, err := CreateBackup(db, backupDir, photoDir, now)
	if err != nil {
		return BackupInfo{}, err
	}
	if _, err := PruneBackups(backupDir, keep); err != nil {
		return b, fmt.Errorf("backup created but pruning old backups failed: %w", err)
	}
	return b, nil
}
