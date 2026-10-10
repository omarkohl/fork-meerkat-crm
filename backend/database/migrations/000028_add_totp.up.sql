-- TOTP two-factor. totp_secret is encrypted with the credential key (empty = none yet);
-- totp_enabled turns the second step on at login; totp_last_step is the last accepted
-- 30-second step, so a code cannot be used twice.
ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN totp_last_step INTEGER NOT NULL DEFAULT 0;

-- One-time recovery codes; only a hash is stored.
CREATE TABLE IF NOT EXISTS recovery_codes (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    user_id    INTEGER  NOT NULL,
    code_hash  TEXT     NOT NULL,
    used_at    DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_recovery_codes_user_id ON recovery_codes(user_id);
