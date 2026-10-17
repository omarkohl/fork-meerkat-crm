package config

import "testing"

func hasBackupTimeError(c *Config) bool {
	for _, e := range c.Validate() {
		if e.Field == "BACKUP_TIME" {
			return true
		}
	}
	return false
}

func TestBackupTimeValidatedOnlyWhenEnabled(t *testing.T) {
	if !hasBackupTimeError(&Config{BackupEnabled: true, BackupTime: "25:99"}) {
		t.Error("an invalid BACKUP_TIME must be reported while backups are enabled")
	}
	if hasBackupTimeError(&Config{BackupEnabled: false, BackupTime: "25:99"}) {
		t.Error("BACKUP_TIME is not checked while backups are disabled")
	}
	if hasBackupTimeError(&Config{BackupEnabled: true, BackupTime: "02:30"}) {
		t.Error("a valid BACKUP_TIME must pass")
	}
}

func TestFakeNowMustBeRFC3339(t *testing.T) {
	has := func(c *Config) bool {
		for _, e := range c.Validate() {
			if e.Field == "FAKE_NOW" {
				return true
			}
		}
		return false
	}
	if has(&Config{}) || has(&Config{FakeNow: "2026-12-24T12:00:00Z"}) {
		t.Error("an empty or valid FAKE_NOW must pass")
	}
	if !has(&Config{FakeNow: "tomorrow"}) {
		t.Error("an invalid FAKE_NOW must be reported")
	}
}
