CREATE TABLE IF NOT EXISTS workouts 
(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  short_description TEXT NOT NULL,
  long_description TEXT NOT NULL,
  workout_date TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- create indexes
CREATE INDEX idx_workouts_date ON workouts(workout_date);
