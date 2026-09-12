package fran

import (
	"reflect"
	"testing"
)

func TestWorkoutDetailsRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		w    *Workout
	}{
		{
			name: "amrap",
			w: &Workout{
				WorkoutType: WorkoutTypeAMRAP,
				AMRAP:       &AMRAPDetails{TimeCapMinutes: 20, RoundsCompleted: 5, ExtraReps: 12},
			},
		},
		{
			name: "emom",
			w: &Workout{
				WorkoutType: WorkoutTypeEMOM,
				EMOM:        &EMOMDetails{IntervalSeconds: 60, DurationMinutes: 15},
			},
		},
		{
			name: "cardio",
			w: &Workout{
				WorkoutType: WorkoutTypeCardio,
				Cardio:      &CardioDetails{Modality: "run", DurationMin: 32, Distance: 4.2, DistanceUnit: "mi"},
			},
		},
		{
			name: "strength",
			w: &Workout{
				WorkoutType: WorkoutTypeStrength,
				Strength: &StrengthDetails{Exercises: []StrengthExercise{
					{Name: "Squat", Sets: 5, Reps: 5, Weight: 225, WeightUnit: "lbs"},
					{Name: "Bench", Sets: 3, Reps: 8, Weight: 135, WeightUnit: "lbs"},
				}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.w.MarshalDetails()
			if err != nil {
				t.Fatalf("MarshalDetails() returned error: %v", err)
			}

			got := &Workout{WorkoutType: tc.w.WorkoutType}
			if err := got.UnmarshalDetails(data); err != nil {
				t.Fatalf("UnmarshalDetails() returned error: %v", err)
			}

			switch tc.w.WorkoutType {
			case WorkoutTypeAMRAP:
				if *got.AMRAP != *tc.w.AMRAP {
					t.Errorf("AMRAP round-trip mismatch: got %+v, want %+v", got.AMRAP, tc.w.AMRAP)
				}
			case WorkoutTypeEMOM:
				if *got.EMOM != *tc.w.EMOM {
					t.Errorf("EMOM round-trip mismatch: got %+v, want %+v", got.EMOM, tc.w.EMOM)
				}
			case WorkoutTypeCardio:
				if *got.Cardio != *tc.w.Cardio {
					t.Errorf("Cardio round-trip mismatch: got %+v, want %+v", got.Cardio, tc.w.Cardio)
				}
			case WorkoutTypeStrength:
				if !reflect.DeepEqual(got.Strength, tc.w.Strength) {
					t.Errorf("Strength round-trip mismatch: got %+v, want %+v", got.Strength, tc.w.Strength)
				}
			}
		})
	}
}

func TestWorkoutDetailsOtherHasNoDetails(t *testing.T) {
	w := &Workout{WorkoutType: WorkoutTypeOther}

	data, err := w.MarshalDetails()
	if err != nil {
		t.Fatalf("MarshalDetails() returned error: %v", err)
	}
	if data != nil {
		t.Errorf("MarshalDetails() for WorkoutTypeOther = %q, want nil", data)
	}
}

func TestWorkoutDetailsUnknownTypeErrors(t *testing.T) {
	w := &Workout{WorkoutType: WorkoutType("bogus")}

	if _, err := w.MarshalDetails(); err == nil {
		t.Error("MarshalDetails() with unknown WorkoutType: expected error, got nil")
	}

	if err := w.UnmarshalDetails([]byte(`{}`)); err == nil {
		t.Error("UnmarshalDetails() with unknown WorkoutType: expected error, got nil")
	}
}
