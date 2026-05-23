package db

import (
	"database/sql"
	"fmt"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  apple_sub   TEXT NOT NULL UNIQUE,
  email       TEXT,
  nickname    TEXT NOT NULL,
  avatar_url  TEXT,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS missions (
  id             TEXT PRIMARY KEY,
  user_id        INTEGER NOT NULL REFERENCES users(id),
  codename       TEXT NOT NULL,
  brief          TEXT NOT NULL,
  tier           TEXT NOT NULL,
  status         TEXT NOT NULL,
  loadout_json   TEXT NOT NULL DEFAULT '[]',
  workspace_dir  TEXT NOT NULL,
  input_tokens   INTEGER NOT NULL DEFAULT 0,
  output_tokens  INTEGER NOT NULL DEFAULT 0,
  started_at     TIMESTAMP,
  ended_at       TIMESTAMP,
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_missions_user_created ON missions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_missions_status        ON missions(status);

CREATE TABLE IF NOT EXISTS steps (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  mission_id         TEXT NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
  seq                INTEGER NOT NULL,
  ts                 TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  type               TEXT NOT NULL,
  payload_json       TEXT NOT NULL DEFAULT '{}',
  reasoning_content  TEXT NOT NULL DEFAULT '',
  UNIQUE(mission_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_steps_mission_seq ON steps(mission_id, seq);

CREATE TABLE IF NOT EXISTS artifacts (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  mission_id  TEXT NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
  kind        TEXT NOT NULL,
  name        TEXT NOT NULL,
  path        TEXT NOT NULL,
  mime        TEXT NOT NULL DEFAULT '',
  size_bytes  INTEGER NOT NULL DEFAULT 0,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_artifacts_mission ON artifacts(mission_id, created_at DESC);
`

// Migrate 应用 schema。SQLite 的 CREATE IF NOT EXISTS 是幂等的，可以反复调用。
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}
