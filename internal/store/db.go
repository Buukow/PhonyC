package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS admin_user (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  temp_disabled INTEGER NOT NULL DEFAULT 0,
  protocol TEXT NOT NULL,
  base_url TEXT NOT NULL,
  api_key TEXT NOT NULL DEFAULT '',
  priority INTEGER NOT NULL DEFAULT 0,
  extra_headers_json TEXT NOT NULL DEFAULT '{}',
  timeout_ms INTEGER NOT NULL DEFAULT 600000,
  test_model TEXT NOT NULL DEFAULT '',
  last_test_at TEXT NOT NULL DEFAULT '',
  last_test_status INTEGER NOT NULL DEFAULT 0,
  last_test_ms INTEGER NOT NULL DEFAULT 0,
  last_test_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS channel_models (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  client_model TEXT NOT NULL,
  upstream_model TEXT NOT NULL,
  rewrite_model INTEGER NOT NULL DEFAULT 0,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(channel_id, client_model)
);

CREATE TABLE IF NOT EXISTS user_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  key TEXT NOT NULL UNIQUE,
  enabled INTEGER NOT NULL DEFAULT 1,
  remark TEXT NOT NULL DEFAULT '',
  impersonation_mode TEXT NOT NULL DEFAULT 'passthrough',
  preset_id INTEGER,
  custom_headers_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS client_presets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  version_label TEXT NOT NULL DEFAULT '',
  headers_json TEXT NOT NULL DEFAULT '{}',
  remove_headers TEXT NOT NULL DEFAULT '[]',
  builtin INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS request_meta (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  request_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  user_key_id INTEGER,
  client_model TEXT NOT NULL DEFAULT '',
  upstream_model TEXT NOT NULL DEFAULT '',
  channel_id INTEGER,
  method TEXT NOT NULL DEFAULT '',
  path TEXT NOT NULL DEFAULT '',
  status_code INTEGER NOT NULL DEFAULT 0,
  ttfb_ms INTEGER NOT NULL DEFAULT 0,
  total_ms INTEGER NOT NULL DEFAULT 0,
  error_summary TEXT NOT NULL DEFAULT '',
  impersonation_mode TEXT NOT NULL DEFAULT '',
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  cached_tokens INTEGER NOT NULL DEFAULT 0,
  reasoning_tokens INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_request_meta_created ON request_meta(created_at);
CREATE INDEX IF NOT EXISTS idx_request_meta_key ON request_meta(user_key_id);
CREATE INDEX IF NOT EXISTS idx_request_meta_path ON request_meta(path);

CREATE TABLE IF NOT EXISTS key_stats_daily (
  user_key_id INTEGER NOT NULL,
  day TEXT NOT NULL,
  requests INTEGER NOT NULL DEFAULT 0,
  errors INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(user_key_id, day)
);

CREATE TABLE IF NOT EXISTS usage_stats_hourly (
  hour TEXT PRIMARY KEY,
  requests INTEGER NOT NULL DEFAULT 0,
  errors INTEGER NOT NULL DEFAULT 0,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  cached_tokens INTEGER NOT NULL DEFAULT 0,
  reasoning_tokens INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS model_stats_daily (
  day TEXT NOT NULL,
  client_model TEXT NOT NULL,
  requests INTEGER NOT NULL DEFAULT 0,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(day, client_model)
);

CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// additive migrations for existing DBs
	alters := []string{
		`ALTER TABLE channels ADD COLUMN temp_disabled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE channels ADD COLUMN last_test_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE channels ADD COLUMN last_test_status INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE channels ADD COLUMN last_test_ms INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE channels ADD COLUMN last_test_error TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE request_meta ADD COLUMN prompt_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE request_meta ADD COLUMN completion_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE request_meta ADD COLUMN total_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE request_meta ADD COLUMN cached_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE request_meta ADD COLUMN reasoning_tokens INTEGER NOT NULL DEFAULT 0`,
	}
	for _, q := range alters {
		_, _ = s.db.Exec(q) // ignore duplicate column errors
	}
	return nil
}

func nowUTC() time.Time { return time.Now().UTC() }

// nowLocal uses the process/system timezone for day/hour bucket boundaries.
func nowLocal() time.Time { return time.Now() }

func hourBucket(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02T15")
}

func dayBucket(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02")
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(v int) bool { return v != 0 }
