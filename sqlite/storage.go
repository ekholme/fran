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

// CreateWorkout inserts a new workout into the database and returns its newly generated ID.
func (s *Storage) CreateWorkout(ctx context.Context, w *fran.Workout) (int, error) {
	qry := `
		INSERT INTO workouts (
		short_description,
		long_description,
		workout_date
	)
	VALUES (?, ?, ?)
	`

	res, err := s.db.ExecContext(ctx, qry, w.ShortDescription, w.LongDescription, w.WorkoutDate)
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
		created_at
		FROM workouts
		WHERE id = ?
	`

	var w fran.Workout

	if err := s.db.QueryRowContext(ctx, qry, id).Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to get workout by ID %d: %w", id, err)
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
	created_at
	FROM workouts
	WHERE workout_date = ?
`

	var w fran.Workout

	if err := s.db.QueryRowContext(ctx, qry, date).Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to get workout by date %s: %w", date, err)
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

		err = rows.Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workout row: %w", err)
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
		created_at
		FROM workouts
		WHERE short_description LIKE ?
	`

	rows, err := s.db.QueryContext(ctx, qry, "%"+desc+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to search workouts by description: %w", err)
	}

	defer rows.Close()

	var workouts []*fran.Workout

	for rows.Next() {
		var w fran.Workout

		err = rows.Scan(&w.ID, &w.ShortDescription, &w.LongDescription, &w.WorkoutDate, &w.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workout row: %w", err)
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
	qry := `
		UPDATE workouts
		SET short_description = ?,
		  long_description = ?,
		  workout_date = ?
		WHERE id = ?
	`

	res, err := s.db.ExecContext(ctx, qry, w.ShortDescription, w.LongDescription, w.WorkoutDate, w.ID)
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
