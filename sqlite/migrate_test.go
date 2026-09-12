package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// preMigrationSchema is the workouts table shape that predates the
// workout_type/details columns, used to simulate an existing user's
// database when testing EnsureSchema.
const preMigrationSchema = `
CREATE TABLE workouts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  short_description TEXT NOT NULL,
  long_description TEXT NOT NULL,
  workout_date TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

func openTestDB(t *testing.T, schema string) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "fran_test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}

	return db
}

func TestEnsureSchema_MigratesExistingDatabase(t *testing.T) {
	db := openTestDB(t, preMigrationSchema)

	res, err := db.Exec(`INSERT INTO workouts (short_description, long_description, workout_date) VALUES (?, ?, ?)`,
		"Pullup, Pushup, KBS", "3 rounds", "2026-08-10")
	if err != nil {
		t.Fatalf("failed to insert pre-migration row: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get inserted row id: %v", err)
	}

	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() returned error: %v", err)
	}

	var workoutType string
	var details sql.NullString
	row := db.QueryRow(`SELECT workout_type, details FROM workouts WHERE id = ?`, id)
	if err := row.Scan(&workoutType, &details); err != nil {
		t.Fatalf("failed to read back migrated row: %v", err)
	}

	if workoutType != "other" {
		t.Errorf("workout_type = %q, want %q for a pre-migration row", workoutType, "other")
	}
	if details.Valid {
		t.Errorf("details = %q, want NULL for a pre-migration row", details.String)
	}

	var indexName string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_workouts_type'`).Scan(&indexName)
	if err != nil {
		t.Errorf("expected idx_workouts_type index to exist after EnsureSchema, query returned: %v", err)
	}
}

func TestEnsureSchema_IdempotentOnAlreadyMigratedDatabase(t *testing.T) {
	db := openTestDB(t, preMigrationSchema)

	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() first run returned error: %v", err)
	}

	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() second run returned error: %v", err)
	}
}

func TestEnsureSchema_NoOpOnFreshCanonicalSchema(t *testing.T) {
	db := openTestDB(t, testSchema)

	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() on a fresh canonical schema returned error: %v", err)
	}
}
