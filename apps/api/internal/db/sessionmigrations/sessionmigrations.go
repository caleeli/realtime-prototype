package sessionmigrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"time"
)

//go:embed sql/*.up.sql
var migrationFiles embed.FS

var migrationFilenameRe = regexp.MustCompile(`^(\d+)_([A-Za-z0-9_]+)\.up\.sql$`)

type migration struct {
	version int
	name    string
	sqlText string
}

// RunMigrations applies all pending up migrations to the provided database.
func RunMigrations(ctx context.Context, db *sql.DB) error {
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return err
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	appliedVersions, err := loadAppliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if appliedVersions[migration.version] {
			continue
		}

		if err := applyMigration(ctx, db, migration); err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);`,
	)
	return err
}

func loadAppliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := map[int]bool{}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions[version] = true
	}
	return versions, rows.Err()
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "sql")
	if err != nil {
		return nil, err
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		matches := migrationFilenameRe.FindStringSubmatch(fileName)
		if len(matches) != 3 {
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("invalid migration version in file %q: %w", fileName, err)
		}

		sqlText, err := fs.ReadFile(migrationFiles, path.Join("sql", fileName))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", fileName, err)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    matches[2],
			sqlText: string(sqlText),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].version == migrations[j].version {
			return migrations[i].name < migrations[j].name
		}
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

func applyMigration(ctx context.Context, db *sql.DB, migration migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, migration.sqlText); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migration %d (%s): %w", migration.version, migration.name, err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?);`,
		migration.version,
		migration.name,
		now,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %d (%s): %w", migration.version, migration.name, err)
	}

	return tx.Commit()
}
