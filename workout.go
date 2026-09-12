// Package fran defines types needed for the application
package fran

import (
	"encoding/json"
	"fmt"
	"time"
)

// WorkoutType identifies which kind of workout a Workout represents, which
// in turn determines which of its detail pointer fields (if any) is set.
type WorkoutType string

const (
	WorkoutTypeAMRAP    WorkoutType = "amrap"
	WorkoutTypeEMOM     WorkoutType = "emom"
	WorkoutTypeCardio   WorkoutType = "cardio"
	WorkoutTypeStrength WorkoutType = "strength"
	WorkoutTypeOther    WorkoutType = "other"
)

// WorkoutTypes lists every valid WorkoutType, in the order they should be
// cycled through in the UI.
var WorkoutTypes = []WorkoutType{
	WorkoutTypeAMRAP,
	WorkoutTypeEMOM,
	WorkoutTypeCardio,
	WorkoutTypeStrength,
	WorkoutTypeOther,
}

// Label returns a human-readable name for the workout type.
func (t WorkoutType) Label() string {
	switch t {
	case WorkoutTypeAMRAP:
		return "AMRAP"
	case WorkoutTypeEMOM:
		return "EMOM"
	case WorkoutTypeCardio:
		return "Cardio"
	case WorkoutTypeStrength:
		return "Strength"
	case WorkoutTypeOther:
		return "Other"
	default:
		return string(t)
	}
}

// AMRAPDetails holds the fields specific to an "as many rounds/reps as
// possible" workout.
type AMRAPDetails struct {
	TimeCapMinutes  int `json:"time_cap_minutes"`
	RoundsCompleted int `json:"rounds_completed"`
	ExtraReps       int `json:"extra_reps"`
}

// EMOMDetails holds the fields specific to an "every minute on the minute"
// workout.
type EMOMDetails struct {
	IntervalSeconds int `json:"interval_seconds"`
	DurationMinutes int `json:"duration_minutes"`
}

// CardioDetails holds the fields specific to a cardio workout.
type CardioDetails struct {
	Modality     string  `json:"modality"`
	DurationMin  int     `json:"duration_minutes"`
	Distance     float64 `json:"distance"`
	DistanceUnit string  `json:"distance_unit"`
}

// StrengthExercise records a single movement within a strength workout
// (e.g. "Squat, 5x5 @ 225 lbs").
type StrengthExercise struct {
	Name       string  `json:"name"`
	Sets       int     `json:"sets"`
	Reps       int     `json:"reps"`
	Weight     float64 `json:"weight"`
	WeightUnit string  `json:"weight_unit"`
}

// StrengthDetails holds the fields specific to a strength workout, which
// may cover several movements (e.g. a squat/bench/row session).
type StrengthDetails struct {
	Exercises []StrengthExercise `json:"exercises"`
}

type Workout struct {
	ID               int         `db:"id"`
	ShortDescription string      `db:"short_description"`
	LongDescription  string      `db:"long_description"`
	WorkoutDate      string      `db:"workout_date"`
	WorkoutType      WorkoutType `db:"workout_type"`
	CreatedAt        time.Time   `db:"created_at"`

	// Exactly one of these is populated, matching WorkoutType. They are
	// serialized to/from the `details` column as JSON rather than mapped
	// to individual db columns.
	AMRAP    *AMRAPDetails
	EMOM     *EMOMDetails
	Cardio   *CardioDetails
	Strength *StrengthDetails
}

// MarshalDetails returns the JSON bytes to store in the details column for
// whichever type-specific struct is set on w. It returns (nil, nil) for
// WorkoutTypeOther, which has no structured details.
func (w *Workout) MarshalDetails() ([]byte, error) {
	switch w.WorkoutType {
	case WorkoutTypeAMRAP:
		return json.Marshal(w.AMRAP)
	case WorkoutTypeEMOM:
		return json.Marshal(w.EMOM)
	case WorkoutTypeCardio:
		return json.Marshal(w.Cardio)
	case WorkoutTypeStrength:
		return json.Marshal(w.Strength)
	case WorkoutTypeOther:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown workout type %q", w.WorkoutType)
	}
}

// UnmarshalDetails populates the typed detail field matching w.WorkoutType
// from raw JSON bytes. It is a no-op if data is empty.
func (w *Workout) UnmarshalDetails(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	switch w.WorkoutType {
	case WorkoutTypeAMRAP:
		w.AMRAP = &AMRAPDetails{}
		return json.Unmarshal(data, w.AMRAP)
	case WorkoutTypeEMOM:
		w.EMOM = &EMOMDetails{}
		return json.Unmarshal(data, w.EMOM)
	case WorkoutTypeCardio:
		w.Cardio = &CardioDetails{}
		return json.Unmarshal(data, w.Cardio)
	case WorkoutTypeStrength:
		w.Strength = &StrengthDetails{}
		return json.Unmarshal(data, w.Strength)
	case WorkoutTypeOther:
		return nil
	default:
		return fmt.Errorf("unknown workout type %q", w.WorkoutType)
	}
}
