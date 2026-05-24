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

CREATE TABLE IF NOT EXISTS mission_reviews (
  mission_id  TEXT PRIMARY KEY REFERENCES missions(id) ON DELETE CASCADE,
  rating      INTEGER NOT NULL,
  comment     TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS skills (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id            INTEGER NOT NULL REFERENCES users(id),
  name               TEXT NOT NULL,
  description        TEXT NOT NULL DEFAULT '',
  trigger_hint       TEXT NOT NULL DEFAULT '',
  prompt_template    TEXT NOT NULL DEFAULT '',
  source_mission_id  TEXT,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_skills_user ON skills(user_id, created_at DESC);

-- ========================= 事件流图谱（v0.2.0） =========================

CREATE TABLE IF NOT EXISTS topics (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id),
  name        TEXT NOT NULL,
  weight      REAL NOT NULL DEFAULT 1.0,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, name)
);
CREATE INDEX IF NOT EXISTS idx_topics_user ON topics(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS news_sources (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL,
  url         TEXT NOT NULL UNIQUE,   -- RSS / feed URL
  kind        TEXT NOT NULL DEFAULT 'rss',
  region      TEXT NOT NULL DEFAULT 'cn',  -- cn / intl_zh / intl_en
  lang        TEXT NOT NULL DEFAULT 'zh',
  enabled     INTEGER NOT NULL DEFAULT 1,
  last_fetch_at TIMESTAMP,
  last_error  TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS news_events (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  source_id     INTEGER NOT NULL REFERENCES news_sources(id),
  url           TEXT NOT NULL UNIQUE,
  title         TEXT NOT NULL,
  summary       TEXT NOT NULL DEFAULT '',
  content       TEXT NOT NULL DEFAULT '',
  lang          TEXT NOT NULL DEFAULT 'zh',
  published_at  TIMESTAMP,
  fetched_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  extracted     INTEGER NOT NULL DEFAULT 0   -- 0=未抽取实体 1=已抽取
);
CREATE INDEX IF NOT EXISTS idx_news_events_pub ON news_events(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_news_events_src_pub ON news_events(source_id, published_at DESC);

CREATE TABLE IF NOT EXISTS entities (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  type          TEXT NOT NULL,  -- person / org / place / concept / event
  name          TEXT NOT NULL,
  weight        REAL NOT NULL DEFAULT 1.0,
  first_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(type, name)
);
CREATE INDEX IF NOT EXISTS idx_entities_last_seen ON entities(last_seen_at DESC);

CREATE TABLE IF NOT EXISTS event_entities (
  event_id   INTEGER NOT NULL REFERENCES news_events(id) ON DELETE CASCADE,
  entity_id  INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
  salience   REAL NOT NULL DEFAULT 1.0,
  PRIMARY KEY(event_id, entity_id)
);
CREATE INDEX IF NOT EXISTS idx_event_entities_entity ON event_entities(entity_id);

CREATE TABLE IF NOT EXISTS entity_relations (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  src_id        INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
  dst_id        INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
  label         TEXT NOT NULL DEFAULT 'related',
  weight        REAL NOT NULL DEFAULT 1.0,
  last_seen_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(src_id, dst_id, label)
);
CREATE INDEX IF NOT EXISTS idx_relations_last_seen ON entity_relations(last_seen_at DESC);

CREATE TABLE IF NOT EXISTS user_event_subs (
  user_id        INTEGER NOT NULL REFERENCES users(id),
  event_id       INTEGER NOT NULL REFERENCES news_events(id) ON DELETE CASCADE,
  relevance      REAL NOT NULL DEFAULT 0,
  matched_topics TEXT NOT NULL DEFAULT '[]',  -- JSON array of topic ids
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(user_id, event_id)
);
CREATE INDEX IF NOT EXISTS idx_user_event_subs ON user_event_subs(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS feed_state (
  k         TEXT PRIMARY KEY,
  v         TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// 增量列：SQLite 不支持 ADD COLUMN IF NOT EXISTS，需要先 PRAGMA table_info 判一下。
type colDecl struct {
	table, col, ddl string
}

var addColumns = []colDecl{
	{"missions", "series_id", "TEXT NOT NULL DEFAULT ''"},
	{"missions", "series_seq", "INTEGER NOT NULL DEFAULT 1"},
	{"missions", "parent_id", "TEXT"},
}

// Migrate 应用 schema。SQLite 的 CREATE IF NOT EXISTS 是幂等的，可以反复调用。
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	for _, c := range addColumns {
		exists, err := columnExists(db, c.table, c.col)
		if err != nil {
			return fmt.Errorf("inspect %s.%s: %w", c.table, c.col, err)
		}
		if exists {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", c.table, c.col, c.ddl)); err != nil {
			return fmt.Errorf("alter %s add %s: %w", c.table, c.col, err)
		}
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_missions_series ON missions(series_id, series_seq)`); err != nil {
		return fmt.Errorf("create idx_missions_series: %w", err)
	}
	return nil
}

func columnExists(db *sql.DB, table, col string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}
