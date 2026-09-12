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
CREATE INDEX IF NOT EXISTS idx_workouts_date ON workouts(workout_date);
CREATE INDEX IF NOT EXISTS idx_workouts_type ON workouts(workout_type);
