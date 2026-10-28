// Package sqlite handles interactions with an sqlite3 database
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ekholme/fran"
)

// Storage provides a repository for workout-related data using an SQLite database.
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new storage object from an existing database connection.
func NewStorage(db *sql.DB) *Storage {
	return &Storage{
		db: db,
	}
}

// nullableDetails marshals w's type-specific details (if any) into a
// sql.NullString suitable for binding to the nullable `details` column.
func nullableDetails(w *fran.Workout) (sql.NullString, error) {
	b, err := w.MarshalDetails()
	if err != nil {
		return sql.NullString{}, fmt.Errorf("failed to marshal workout details: %w", err)
	}

	return sql.NullString{String: string(b), Valid: b != nil}, nil
}

// applyDetails unmarshals a scanned `details` column value into w, if present.
func applyDetails(w *fran.Workout, detailsRaw sql.NullString) error {
	if !detailsRaw.Valid {
		return nil
	}

	if err := w.UnmarshalDetails([]byte(detailsRaw.String)); err != nil {
		return fmt.Errorf("failed to unmarshal workout details for ID %d: %w", w.ID, err)
	}

	return nil
}

// CreateWorkout inserts a new workout into the database and returns its newly generated ID.
func (s *Storage) CreateWorkout(ctx context.Context, w *fran.Workout) (int, error) {
	details, err := nullableDetails(w)
	if err != nil {
		return 0, err
	}

	qry := `
		INSERT INTO workouts (
		short_description,
		long_description,
		workout_date,
		workout_type,
		details
	)
	VALUES (?, ?, ?, ?, ?)
	`

	res, err := s.db.ExecContext(ctx, qry, w.ShortDescription, w.LongDescription, w.WorkoutDate, w.WorkoutType, details)
	if err != nil {
		return 0, fmt.Errorf("failed to create workout: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get workout ID after creation: %w", err)
	}

	return int(id), nil
}

// GetWorkoutByID retrieves a single workout from the database by its primary key ID.
func (s *Storage) GetWorkoutByID(ctx context.Context, id int) (*fran.Workout, error) {
	qry := `
		SELECT id,
		short_description,
		long_description,
		workout_date,
		workout_type,
		details,
		created_at
		FROM workouts
		WHERE id = ?
	`

	var w fran.Workout
	var detailsRaw sql.NullString

	if err := s.db.QueryRowContext(ctx, qry, id).Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.WorkoutType, &detailsRaw, &w.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to get workout by ID %d: %w", id, err)
	}

	if err := applyDetails(&w, detailsRaw); err != nil {
		return nil, err
	}

	return &w, nil
}

// GetWorkoutByDate retrieves a single workout from the database that matches the given date string.
func (s *Storage) GetWorkoutByDate(ctx context.Context, date string) (*fran.Workout, error) {
	qry := `
	SELECT id,
	short_description,
	long_description,
	workout_date,
	workout_type,
	details,
	created_at
	FROM workouts
	WHERE workout_date = ?
`

	var w fran.Workout
	var detailsRaw sql.NullString

	if err := s.db.QueryRowContext(ctx, qry, date).Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.WorkoutType, &detailsRaw, &w.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to get workout by date %s: %w", date, err)
	}

	if err := applyDetails(&w, detailsRaw); err != nil {
		return nil, err
	}

	return &w, nil
}

// GetLatestWorkouts retrieves the N most recent workouts, ordered by workout date in descending order.
func (s *Storage) GetLatestWorkouts(ctx context.Context, n int) ([]*fran.Workout, error) {
	qry := `
	SELECT id,
	short_description,
	long_description,
	workout_date,
	workout_type,
	details,
	created_at
	FROM workouts
	ORDER BY workout_date DESC
	LIMIT ?;
`

	rows, err := s.db.QueryContext(ctx, qry, n)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest workouts: %w", err)
	}

	defer rows.Close()

	var workouts []*fran.Workout

	for rows.Next() {
		var w fran.Workout
		var detailsRaw sql.NullString

		err = rows.Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.WorkoutType, &detailsRaw, &w.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workout row: %w", err)
		}

		if err := applyDetails(&w, detailsRaw); err != nil {
			return nil, err
		}

		workouts = append(workouts, &w)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over workouts: %w", err)
	}

	return workouts, nil
}

// GetWorkoutsByShortDesc retrieves all workouts whose short description matches the provided search string.
func (s *Storage) GetWorkoutsByShortDesc(ctx context.Context, desc string) ([]*fran.Workout, error) {
	qry := `
		SELECT id,
		short_description,
		long_description,
		workout_date,
		workout_type,
		details,
		created_at
		FROM workouts
		WHERE short_description LIKE ?
		ORDER BY workout_date DESC
	`

	rows, err := s.db.QueryContext(ctx, qry, "%"+desc+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to search workouts by description: %w", err)
	}

	defer rows.Close()

	var workouts []*fran.Workout

	for rows.Next() {
		var w fran.Workout
		var detailsRaw sql.NullString

		err = rows.Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.WorkoutType, &detailsRaw, &w.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workout row: %w", err)
		}

		if err := applyDetails(&w, detailsRaw); err != nil {
			return nil, err
		}

		workouts = append(workouts, &w)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over workouts: %w", err)
	}

	return workouts, nil
}

// GetWorkoutsByDateRange retrieves all workouts whose workout date falls between start and end (inclusive, both YYYY-MM-DD), ordered by workout date in ascending order.
func (s *Storage) GetWorkoutsByDateRange(ctx context.Context, start, end string) ([]*fran.Workout, error) {
	qry := `
		SELECT id,
		short_description,
		long_description,
		workout_date,
		workout_type,
		details,
		created_at
		FROM workouts
		WHERE workout_date BETWEEN ? AND ?
		ORDER BY workout_date ASC, id ASC
	`

	rows, err := s.db.QueryContext(ctx, qry, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get workouts by date range: %w", err)
	}

	defer rows.Close()

	var workouts []*fran.Workout

	for rows.Next() {
		var w fran.Workout
		var detailsRaw sql.NullString

		err = rows.Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.WorkoutType, &detailsRaw, &w.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workout row: %w", err)
		}

		if err := applyDetails(&w, detailsRaw); err != nil {
			return nil, err
		}

		workouts = append(workouts, &w)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over workouts: %w", err)
	}

	return workouts, nil
}

// UpdateWorkout updates the details of an existing workout in the database based on its ID.
func (s *Storage) UpdateWorkout(ctx context.Context, w *fran.Workout) error {
	details, err := nullableDetails(w)
	if err != nil {
		return err
	}

	qry := `
		UPDATE workouts
		SET short_description = ?,
		  long_description = ?,
		  workout_date = ?,
		  workout_type = ?,
		  details = ?
		WHERE id = ?
	`

	res, err := s.db.ExecContext(ctx, qry, w.ShortDescription, w.LongDescription, w.WorkoutDate, w.WorkoutType, details, w.ID)
	if err != nil {
		return fmt.Errorf("failed to update workout %d: %w", w.ID, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected after update: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteWorkout removes a workout from the database by its primary key ID.
func (s *Storage) DeleteWorkout(ctx context.Context, id int) error {
	qry := `
		DELETE FROM workouts WHERE id = ?
	`

	res, err := s.db.ExecContext(ctx, qry, id)
	if err != nil {
		return fmt.Errorf("failed to delete workout %d: %w", id, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected after deletion: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
