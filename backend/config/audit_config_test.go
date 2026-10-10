package config

import "testing"

func TestAuditLogRetentionDays(t *testing.T) {
	for value, want := range map[string]int{"": 365, "30": 30, "0": 0, "-5": 365, "abc": 365} {
		t.Run(value, func(t *testing.T) {
			unsetEnv(t, "AUDIT_LOG_RETENTION_DAYS")
			if value != "" {
				t.Setenv("AUDIT_LOG_RETENTION_DAYS", value)
			}
			if got := LoadConfig().AuditLogRetentionDays; got != want {
				t.Errorf("AUDIT_LOG_RETENTION_DAYS=%q: got %d, want %d", value, got, want)
			}
		})
	}
}
