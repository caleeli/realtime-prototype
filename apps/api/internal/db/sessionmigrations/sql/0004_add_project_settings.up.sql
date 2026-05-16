CREATE TABLE IF NOT EXISTS project_settings(
	project_id TEXT PRIMARY KEY,
	design_style TEXT DEFAULT '',
	color_palette TEXT DEFAULT '',
	brand_guidelines TEXT DEFAULT '',
	component_examples TEXT DEFAULT '',
	technical_constraints TEXT DEFAULT '',
	layout_preferences TEXT DEFAULT '',
	image_generation_notes TEXT DEFAULT '',
	generation_context TEXT DEFAULT '',
	updated_at TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_settings_project ON project_settings(project_id);
