package sqlite

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed migration.sql
var migrationSQL string

// EnsureSchema brings a database up to date with the current schema,
// without a full migration-runner. It's safe to call on every startup
// against any of: a brand-new (even empty-file) database, one already on
// the canonical schema, or one predating the workout_type/details columns.
//
// It first applies migration.sql's CREATE TABLE/INDEX IF NOT EXISTS
// statements, which bootstrap a brand-new database but are no-ops against
// an existing one. Then, since SQLite has no ADD COLUMN IF NOT EXISTS, it
// attempts to add workout_type/details directly, swallowing the resulting
// "duplicate column name" error on a database that already has them (that
// error is the only signal SQLite gives for "already up to date" here).
func EnsureSchema(db *sql.DB) error {
	if _, err := db.Exec(migrationSQL); err != nil {
		return fmt.Errorf("failed to apply base schema: %w", err)
	}

	stmts := []string{
		`ALTER TABLE workouts ADD COLUMN workout_type TEXT NOT NULL DEFAULT 'other'`,
		`ALTER TABLE workouts ADD COLUMN details TEXT`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("failed to apply schema update %q: %w", stmt, err)
		}
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_workouts_type ON workouts(workout_type)`); err != nil {
		return fmt.Errorf("failed to create workout_type index: %w", err)
	}

	return nil
}
