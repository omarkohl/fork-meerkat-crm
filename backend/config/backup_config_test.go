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
