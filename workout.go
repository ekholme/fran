// Package fran defines types needed for the application
package fran

import "time"

type Workout struct {
	ID               int       `db:"id"`
	ShortDescription string    `db:"short_description"`
	LongDescription  string    `db:"long_description"`
	WorkoutDate      string    `db:"workout_date"`
	CreatedAt        time.Time `db:"created_at"`
}
