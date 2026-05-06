INSERT INTO projects (
	id,
	name,
	theme,
	active_screen_id,
	created_at,
	updated_at,
	last_opened_at
) SELECT
	'project-default',
	'Proyecto principal',
	'bootstrap',
	NULL,
	datetime('now'),
	datetime('now'),
	NULL
WHERE NOT EXISTS (SELECT 1 FROM projects WHERE id = 'project-default');
