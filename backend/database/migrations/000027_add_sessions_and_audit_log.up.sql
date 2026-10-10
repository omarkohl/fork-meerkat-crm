-- Token version: raising it signs the user out everywhere (every JWT carries the version it was issued with).
ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;

-- Login sessions. public_id is the "sid" claim of the JWT and the id used in URLs.
CREATE TABLE IF NOT EXISTS sessions (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    user_id      INTEGER  NOT NULL,
    public_id    TEXT     NOT NULL UNIQUE,
    user_agent   TEXT     NOT NULL DEFAULT '',
    ip           TEXT     NOT NULL DEFAULT '',
    last_seen_at DATETIME,
    expires_at   DATETIME NOT NULL,
    revoked_at   DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

-- Security-relevant events. user_id has no foreign key and username is a copy: the record
-- of what a deleted account did stays until the retention period ends.
CREATE TABLE IF NOT EXISTS audit_logs (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    user_id    INTEGER,
    username   TEXT     NOT NULL DEFAULT '',
    event      TEXT     NOT NULL,
    ip         TEXT     NOT NULL DEFAULT '',
    user_agent TEXT     NOT NULL DEFAULT '',
    detail     TEXT     NOT NULL DEFAULT ''
);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_event ON audit_logs(event);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
