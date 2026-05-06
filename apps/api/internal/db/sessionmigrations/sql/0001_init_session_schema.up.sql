CREATE TABLE IF NOT EXISTS projects(
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	theme TEXT NOT NULL,
	active_screen_id TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	last_opened_at TEXT
);

CREATE TABLE IF NOT EXISTS screens(
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	name TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	is_active INTEGER NOT NULL DEFAULT 0,
	is_deleted INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS screen_states(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	screen_id TEXT NOT NULL,
	revision INTEGER NOT NULL,
	screen_payload_json TEXT NOT NULL,
	conversation_json TEXT NOT NULL,
	recommendations_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE(screen_id, revision),
	FOREIGN KEY(screen_id) REFERENCES screens(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_screens_project ON screens(project_id, is_deleted, position, updated_at);
CREATE INDEX IF NOT EXISTS idx_screen_states_screen ON screen_states(screen_id, revision);
