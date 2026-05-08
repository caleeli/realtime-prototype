CREATE TABLE IF NOT EXISTS flow_diagrams (
	project_id TEXT PRIMARY KEY,
	diagram_payload_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
