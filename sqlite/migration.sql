CREATE TABLE IF NOT EXISTS workouts
(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  short_description TEXT NOT NULL,
  long_description TEXT NOT NULL,
  workout_date TEXT NOT NULL,
  workout_type TEXT NOT NULL DEFAULT 'other',
  details TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- create indexes
-- idx_workouts_type is created separately, by sqlite.EnsureSchema, after
-- it has made sure the workout_type column exists — a database predating
-- that column would fail this statement if it ran here.
CREATE INDEX IF NOT EXISTS idx_workouts_date ON workouts(workout_date);
