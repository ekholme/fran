package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
)

// EnsureSchema brings an existing database up to date with schema changes
// introduced after its initial creation, without a full migration-runner.
// ALTER TABLE ADD COLUMN is not idempotent in SQLite, so a "duplicate
// column name" error (meaning the database is already up to date) is
// swallowed; any other error is returned. Safe to call on every startup,
// against either a freshly created database (via migration.sql) or an
// existing one predating these columns.
func EnsureSchema(db *sql.DB) error {
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
