DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS sessions;
ALTER TABLE users DROP COLUMN token_version;
