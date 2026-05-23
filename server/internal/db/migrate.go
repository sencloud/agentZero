package db

import "database/sql"

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	apple_sub  TEXT NOT NULL UNIQUE,
	email      TEXT,
	nickname   TEXT NOT NULL,
	avatar_url TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS categories (
	id    INTEGER PRIMARY KEY AUTOINCREMENT,
	slug  TEXT NOT NULL UNIQUE,
	name  TEXT NOT NULL,
	icon  TEXT NOT NULL,
	color TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agents (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	slug           TEXT NOT NULL UNIQUE,
	name           TEXT NOT NULL,
	tagline        TEXT NOT NULL,
	description    TEXT NOT NULL,
	icon_url       TEXT NOT NULL,
	cover_url      TEXT NOT NULL,
	screenshots    TEXT NOT NULL DEFAULT '[]',
	category_id    INTEGER NOT NULL REFERENCES categories(id),
	developer      TEXT NOT NULL,
	version        TEXT NOT NULL,
	size_bytes     INTEGER NOT NULL DEFAULT 0,
	rating         REAL NOT NULL DEFAULT 0,
	rating_count   INTEGER NOT NULL DEFAULT 0,
	install_count  INTEGER NOT NULL DEFAULT 0,
	is_free        INTEGER NOT NULL DEFAULT 1,
	price_cents    INTEGER NOT NULL DEFAULT 0,
	is_featured    INTEGER NOT NULL DEFAULT 0,
	feature_badge  TEXT,
	capabilities   TEXT NOT NULL DEFAULT '[]',
	updated_notes  TEXT,
	released_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agents_category ON agents(category_id);
CREATE INDEX IF NOT EXISTS idx_agents_featured ON agents(is_featured);

CREATE TABLE IF NOT EXISTS reviews (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	agent_id   INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	rating     INTEGER NOT NULL,
	title      TEXT NOT NULL,
	body       TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(agent_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_reviews_agent ON reviews(agent_id);

CREATE TABLE IF NOT EXISTS installs (
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	agent_id   INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	installed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(user_id, agent_id)
);

CREATE TABLE IF NOT EXISTS today_cards (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	kind        TEXT NOT NULL,
	eyebrow     TEXT NOT NULL,
	title       TEXT NOT NULL,
	subtitle    TEXT NOT NULL,
	cover_url   TEXT NOT NULL,
	agent_id    INTEGER REFERENCES agents(id),
	sort_order  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chat_sessions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	agent_id   INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	title      TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chat_messages (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id INTEGER NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
	role       TEXT NOT NULL,
	content    TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_session ON chat_messages(session_id);
`

func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}
