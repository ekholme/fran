package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/ekholme/fran"
)

const testSchema = `
CREATE TABLE workouts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  short_description TEXT NOT NULL,
  long_description TEXT NOT NULL,
  workout_date TEXT NOT NULL,
  workout_type TEXT NOT NULL DEFAULT 'other',
  details TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

// newTestStorage returns a Storage backed by a fresh, on-disk temp SQLite
// database with the current schema already applied.
func newTestStorage(t *testing.T) *Storage {
	t.Helper()

	return NewStorage(openTestDB(t, testSchema))
}

func TestCreateAndGetWorkoutByID(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	w := &fran.Workout{
		ShortDescription: "Morning run",
		LongDescription:  "Easy pace",
		WorkoutDate:      "2026-09-12",
		WorkoutType:      fran.WorkoutTypeCardio,
		Cardio: &fran.CardioDetails{
			Modality:     "run",
			DurationMin:  32,
			Distance:     4.2,
			DistanceUnit: "mi",
		},
	}

	id, err := s.CreateWorkout(ctx, w)
	if err != nil {
		t.Fatalf("CreateWorkout() returned error: %v", err)
	}
	if id == 0 {
		t.Fatalf("CreateWorkout() returned id 0, want a positive id")
	}

	got, err := s.GetWorkoutByID(ctx, id)
	if err != nil {
		t.Fatalf("GetWorkoutByID(%d) returned error: %v", id, err)
	}

	if got.ShortDescription != w.ShortDescription {
		t.Errorf("ShortDescription = %q, want %q", got.ShortDescription, w.ShortDescription)
	}
	if got.WorkoutType != fran.WorkoutTypeCardio {
		t.Errorf("WorkoutType = %q, want %q", got.WorkoutType, fran.WorkoutTypeCardio)
	}
	if got.Cardio == nil {
		t.Fatalf("Cardio = nil, want populated details")
	}
	if *got.Cardio != *w.Cardio {
		t.Errorf("Cardio = %+v, want %+v", got.Cardio, w.Cardio)
	}
	if got.AMRAP != nil || got.EMOM != nil || got.Strength != nil {
		t.Errorf("expected only Cardio to be populated, got AMRAP=%+v EMOM=%+v Strength=%+v", got.AMRAP, got.EMOM, got.Strength)
	}
}

func TestCreateWorkout_OtherTypeHasNoDetails(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	w := &fran.Workout{
		ShortDescription: "Rest day mobility",
		LongDescription:  "20 min foam rolling",
		WorkoutDate:      "2026-09-12",
		WorkoutType:      fran.WorkoutTypeOther,
	}

	id, err := s.CreateWorkout(ctx, w)
	if err != nil {
		t.Fatalf("CreateWorkout() returned error: %v", err)
	}

	got, err := s.GetWorkoutByID(ctx, id)
	if err != nil {
		t.Fatalf("GetWorkoutByID(%d) returned error: %v", id, err)
	}

	if got.AMRAP != nil || got.EMOM != nil || got.Cardio != nil || got.Strength != nil {
		t.Errorf("expected no detail structs populated for WorkoutTypeOther, got AMRAP=%+v EMOM=%+v Cardio=%+v Strength=%+v", got.AMRAP, got.EMOM, got.Cardio, got.Strength)
	}
}

func TestGetWorkoutByDate(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	w := &fran.Workout{
		ShortDescription: "EMOM 20",
		LongDescription:  "5 burpees / 10 KBS",
		WorkoutDate:      "2026-09-10",
		WorkoutType:      fran.WorkoutTypeEMOM,
		EMOM:             &fran.EMOMDetails{IntervalSeconds: 60, DurationMinutes: 20},
	}

	if _, err := s.CreateWorkout(ctx, w); err != nil {
		t.Fatalf("CreateWorkout() returned error: %v", err)
	}

	got, err := s.GetWorkoutByDate(ctx, "2026-09-10")
	if err != nil {
		t.Fatalf("GetWorkoutByDate() returned error: %v", err)
	}

	if got.EMOM == nil || *got.EMOM != *w.EMOM {
		t.Errorf("EMOM = %+v, want %+v", got.EMOM, w.EMOM)
	}

	if _, err := s.GetWorkoutByDate(ctx, "1999-01-01"); err == nil {
		t.Error("GetWorkoutByDate() for a missing date: expected error, got nil")
	}
}

func TestGetLatestWorkouts(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	dates := []string{"2026-09-01", "2026-09-03", "2026-09-02"}
	for _, d := range dates {
		w := &fran.Workout{
			ShortDescription: "Workout " + d,
			LongDescription:  "notes",
			WorkoutDate:      d,
			WorkoutType:      fran.WorkoutTypeOther,
		}
		if _, err := s.CreateWorkout(ctx, w); err != nil {
			t.Fatalf("CreateWorkout(%s) returned error: %v", d, err)
		}
	}

	got, err := s.GetLatestWorkouts(ctx, 2)
	if err != nil {
		t.Fatalf("GetLatestWorkouts() returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("GetLatestWorkouts() returned %d workouts, want 2", len(got))
	}
	if got[0].WorkoutDate != "2026-09-03" || got[1].WorkoutDate != "2026-09-02" {
		t.Errorf("GetLatestWorkouts() order = [%s, %s], want [2026-09-03, 2026-09-02]", got[0].WorkoutDate, got[1].WorkoutDate)
	}
}

func TestGetWorkoutsByShortDesc(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	entries := []string{"KB ABC", "Cardio - run", "KB DEF"}
	for _, sd := range entries {
		w := &fran.Workout{
			ShortDescription: sd,
			LongDescription:  "notes",
			WorkoutDate:      "2026-09-01",
			WorkoutType:      fran.WorkoutTypeOther,
		}
		if _, err := s.CreateWorkout(ctx, w); err != nil {
			t.Fatalf("CreateWorkout(%s) returned error: %v", sd, err)
		}
	}

	got, err := s.GetWorkoutsByShortDesc(ctx, "KB")
	if err != nil {
		t.Fatalf("GetWorkoutsByShortDesc() returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("GetWorkoutsByShortDesc(\"KB\") returned %d workouts, want 2", len(got))
	}
}

func TestGetWorkoutsByDateRange(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	dates := []string{"2026-08-31", "2026-09-15", "2026-09-01", "2026-09-30", "2026-10-01", ""}
	for _, d := range dates {
		w := &fran.Workout{
			ShortDescription: "workout " + d,
			WorkoutDate:      d,
			WorkoutType:      fran.WorkoutTypeOther,
		}
		if _, err := s.CreateWorkout(ctx, w); err != nil {
			t.Fatalf("CreateWorkout(%q) returned error: %v", d, err)
		}
	}

	got, err := s.GetWorkoutsByDateRange(ctx, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("GetWorkoutsByDateRange() returned error: %v", err)
	}

	want := []string{"2026-09-01", "2026-09-15", "2026-09-30"}
	if len(got) != len(want) {
		t.Fatalf("GetWorkoutsByDateRange() returned %d workouts, want %d", len(got), len(want))
	}
	for i, w := range got {
		if w.WorkoutDate != want[i] {
			t.Errorf("workout %d date = %q, want %q (inclusive range, ascending order)", i, w.WorkoutDate, want[i])
		}
	}
}

func TestUpdateWorkout(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	w := &fran.Workout{
		ShortDescription: "5x5",
		LongDescription:  "Squat day",
		WorkoutDate:      "2026-09-05",
		WorkoutType:      fran.WorkoutTypeStrength,
		Strength: &fran.StrengthDetails{Exercises: []fran.StrengthExercise{
			{Name: "Squat", Sets: 5, Reps: 5, Weight: 185, WeightUnit: "lbs"},
		}},
	}

	id, err := s.CreateWorkout(ctx, w)
	if err != nil {
		t.Fatalf("CreateWorkout() returned error: %v", err)
	}

	updated := &fran.Workout{
		ID:               id,
		ShortDescription: "5x5 (heavier)",
		LongDescription:  "Squat day, felt strong",
		WorkoutDate:      "2026-09-05",
		WorkoutType:      fran.WorkoutTypeStrength,
		Strength: &fran.StrengthDetails{Exercises: []fran.StrengthExercise{
			{Name: "Squat", Sets: 5, Reps: 5, Weight: 205, WeightUnit: "lbs"},
			{Name: "Bench", Sets: 3, Reps: 8, Weight: 135, WeightUnit: "lbs"},
		}},
	}

	if err := s.UpdateWorkout(ctx, updated); err != nil {
		t.Fatalf("UpdateWorkout() returned error: %v", err)
	}

	got, err := s.GetWorkoutByID(ctx, id)
	if err != nil {
		t.Fatalf("GetWorkoutByID(%d) returned error: %v", id, err)
	}

	if got.ShortDescription != "5x5 (heavier)" {
		t.Errorf("ShortDescription = %q, want %q", got.ShortDescription, "5x5 (heavier)")
	}
	if got.Strength == nil || len(got.Strength.Exercises) != 2 || got.Strength.Exercises[0].Weight != 205 {
		t.Errorf("Strength = %+v, want 2 exercises with first Weight 205", got.Strength)
	}
}

func TestUpdateWorkout_NotFound(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	w := &fran.Workout{
		ID:               999,
		ShortDescription: "does not exist",
		LongDescription:  "notes",
		WorkoutDate:      "2026-09-05",
		WorkoutType:      fran.WorkoutTypeOther,
	}

	err := s.UpdateWorkout(ctx, w)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateWorkout() for a missing ID = %v, want sql.ErrNoRows", err)
	}
}

func TestDeleteWorkout(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	w := &fran.Workout{
		ShortDescription: "to delete",
		LongDescription:  "notes",
		WorkoutDate:      "2026-09-05",
		WorkoutType:      fran.WorkoutTypeOther,
	}

	id, err := s.CreateWorkout(ctx, w)
	if err != nil {
		t.Fatalf("CreateWorkout() returned error: %v", err)
	}

	if err := s.DeleteWorkout(ctx, id); err != nil {
		t.Fatalf("DeleteWorkout(%d) returned error: %v", id, err)
	}

	if _, err := s.GetWorkoutByID(ctx, id); err == nil {
		t.Error("GetWorkoutByID() after delete: expected error, got nil")
	}
}

func TestDeleteWorkout_NotFound(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	err := s.DeleteWorkout(ctx, 999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("DeleteWorkout() for a missing ID = %v, want sql.ErrNoRows", err)
	}
}
