CREATE TABLE admin_user (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  csrf_secret TEXT NOT NULL,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);

CREATE TABLE login_attempts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ip TEXT NOT NULL,
  username TEXT NOT NULL,
  success INTEGER NOT NULL,
  attempted_at TEXT NOT NULL
);

CREATE INDEX idx_login_attempts_ip ON login_attempts(ip, attempted_at);
CREATE INDEX idx_login_attempts_user ON login_attempts(username, attempted_at);

CREATE TABLE sources (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  db_type TEXT NOT NULL CHECK (db_type IN ('postgres', 'mysql', 'sqlite')),
  enabled INTEGER NOT NULL DEFAULT 1,
  config_json TEXT NOT NULL,
  password_ciphertext TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE schedules (
  source_id TEXT PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
  enabled INTEGER NOT NULL DEFAULT 0,
  mode TEXT NOT NULL DEFAULT 'cron' CHECK (mode IN ('interval', 'cron')),
  interval_seconds INTEGER,
  cron_expr TEXT,
  timezone TEXT NOT NULL DEFAULT 'UTC',
  upload INTEGER NOT NULL DEFAULT 0,
  local_retention_count INTEGER,
  local_retention_days INTEGER,
  remote_retention_count INTEGER,
  remote_retention_days INTEGER,
  last_run_at TEXT,
  next_run_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE backups (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  db_type TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'expired')),
  format TEXT NOT NULL,
  tool_version TEXT NOT NULL DEFAULT '',
  relative_path TEXT NOT NULL DEFAULT '',
  size_bytes INTEGER NOT NULL DEFAULT 0,
  checksum_sha256 TEXT NOT NULL DEFAULT '',
  error_summary TEXT NOT NULL DEFAULT '',
  trigger TEXT NOT NULL CHECK (trigger IN ('manual', 'schedule')),
  upload_requested INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  started_at TEXT,
  completed_at TEXT
);

CREATE INDEX idx_backups_source_created ON backups(source_id, created_at DESC);
CREATE INDEX idx_backups_status_created ON backups(status, created_at);

CREATE TABLE uploads (
  id TEXT PRIMARY KEY,
  backup_id TEXT NOT NULL UNIQUE REFERENCES backups(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'expired')),
  object_key TEXT NOT NULL DEFAULT '',
  attempts INTEGER NOT NULL DEFAULT 0,
  error_summary TEXT NOT NULL DEFAULT '',
  started_at TEXT,
  completed_at TEXT,
  updated_at TEXT NOT NULL
);

CREATE INDEX idx_uploads_status ON uploads(status);

CREATE TABLE settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  s3_endpoint TEXT NOT NULL DEFAULT '',
  s3_region TEXT NOT NULL DEFAULT '',
  s3_bucket TEXT NOT NULL DEFAULT '',
  s3_prefix TEXT NOT NULL DEFAULT '',
  s3_path_style INTEGER NOT NULL DEFAULT 0,
  s3_tls_verify INTEGER NOT NULL DEFAULT 1,
  s3_access_key_ciphertext TEXT NOT NULL DEFAULT '',
  s3_secret_key_ciphertext TEXT NOT NULL DEFAULT '',
  local_retention_count INTEGER NOT NULL DEFAULT 7,
  local_retention_days INTEGER NOT NULL DEFAULT 30,
  remote_retention_count INTEGER NOT NULL DEFAULT 14,
  remote_retention_days INTEGER NOT NULL DEFAULT 90,
  max_concurrent_backups INTEGER NOT NULL DEFAULT 2,
  updated_at TEXT NOT NULL
);

INSERT INTO settings (id, updated_at) VALUES (1, '1970-01-01T00:00:00.000000000Z');

CREATE TABLE audit_events (
  id TEXT PRIMARY KEY,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL DEFAULT '',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);

CREATE INDEX idx_audit_created ON audit_events(created_at DESC);
