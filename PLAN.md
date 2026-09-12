# Improvement Plan for Fran

This document outlines a roadmap for improving and extending the Fran workout tracker. The goal is to maintain its minimalist nature while enhancing usability and functionality.

## Phase 0: Bug Fixes (Correctness)

Found during code review of the current `sqlite` and `ui` packages. Fix these before building further on top of them — in particular, `UpdateWorkout` and `GetWorkoutsByShortDesc` are broken as written and would fail as soon as something calls them.

[x] Fix `UpdateWorkout` (`sqlite/storage.go`): the query has 4 placeholders (`short_description`, `long_description`, `workout_date`, `id`) but only 3 args are bound in `ExecContext` — `w.ID` is missing, so every update fails at runtime.
[x] Fix the `rowsAffected` check in `UpdateWorkout` and `DeleteWorkout`: both currently do `if rowsAffected != 0 { return sql.ErrNoRows }`, which is inverted — it reports "no rows" on a successful write and silently returns nil when nothing was actually updated/deleted. Should be `== 0`.
[x] Fix `GetWorkoutsByShortDesc`'s SQL: `WHERE short_description like (%?%)` is invalid SQL. Use `WHERE short_description LIKE ?` and bind `"%" + desc + "%"` (or build the wildcard in SQL with `'%' || ? || '%'`).
[x] Block saving a workout with an empty short description in `updateWorkoutForm`'s `ctrl+s` handler — the form already renders a red "Short description is required" message but never actually prevents the save.
[x] Remove the stray `m.inputs.ShortDescriptionInput.Focus()` call inside the list-filter-reset `esc` handler in `updateWorkoutList` — it reaches into the create-form's input from the list state and looks like a leftover/copy-paste artifact.
[x] Deduplicate date validation: `isDateValid` already exists but `updateWorkoutForm`'s `ctrl+s` handler re-implements the same `time.Parse` check inline instead of calling it.

## Phase 1: Core Enhancements (UX & Robustness)

### 1.1 Input Validation & Feedback

[x] Implement validation for the `WorkoutDate` field (ensure it follows YYYY-MM-DD format).
[x] Add real-time validation feedback in the "Create Workout" view (e.g., highlight fields in red if invalid).
[x] Show specific error messages if a database operation fails instead of just "Error: ...".

### 1.2 Enhanced Navigation & Filtering

[] Wire `GetWorkoutsByShortDesc` into the main list view as a real DB-backed search. Unchecked because the list's `/` filter is currently bubbles/list's built-in client-side filter over the already-loaded page of workouts, not a database search — and the underlying query is broken (see Phase 0).
[x] Add a search bar or a toggleable filter at the top of the list to quickly find specific workouts.
[x] Add "Previous" and "Next" navigation between workouts in the "View Workout" state. Note: this navigates within `allWorkouts`, the currently loaded page of `listLength` workouts, not the full history — revisit once pagination or real search lands.

### 1.3 Polished UI/UX

[x] Use `lipgloss` to add borders and consistent spacing to the "Create Workout" form.
[] Improve the "View Workout" layout to better handle long descriptions (e.g., using a scrollable area if it exceeds terminal height).
[] Add a "loading" spinner or progress indicator for database fetches.

### 1.4 Configuration & Defaults

[x] Support overriding the database path via the `FRAN_DB_PATH` environment variable.
[] Support a configuration file (e.g., `~/.config/fran/config.toml`) to manage settings like default list size and theme preferences.

### 1.5 Editing & Deletion

[] Wire the existing `UpdateWorkout` storage method into the UI — e.g. an "edit" keybinding from the "View Workout" state that reopens the create-workout form pre-filled with the selected workout's data. Depends on the Phase 0 `UpdateWorkout` fix.
[] Wire the existing `DeleteWorkout` storage method into the UI — it's part of the `WorkoutStore` interface but is never called from anywhere. Add a "delete" keybinding with a confirmation step. Depends on the Phase 0 `rowsAffected` fix.

### 1.6 Testing

[] Add unit tests for `sqlite.Storage` (e.g. against an in-memory/temp sqlite DB) — there are currently no `*_test.go` files despite `task test`/`just test` running `go test -race ./...`.
[] Add unit tests for the `ui` package's `Update` logic (state transitions, validation) using a fake `WorkoutStore`.

## Phase 2: Data Model & Functionality Extensions

### 2.1 Richer Workout Data

[] **Categories/Tags:** Add a tagging system to categorize workouts (e.g., #strength, #cardio, #mobility).
[] **Sets and Reps:** Introduce a structured way to record sets, reps, and weight (e.g., `Bench Press: 3x10 @ 135lbs`).
[] **Duration Tracking:** Add a field to track how long a workout lasted.

### 2.2 Statistics & Analytics

[] **Summary View:** A new state to view aggregate statistics (e.g., workouts per week, total volume).
[] **Progress Tracking:** Simple visual indicators of progress over time for specific exercises.

### 2.3 Data Portability

[] **Export/Import:** Add commands to export workouts to JSON or CSV files for backup or sharing.

## Phase 3: Advanced Features (Optional)

### 3.1 Notifications

[] Integration with system notifications (e.g., `notify-send` or similar) for workout reminders (if a timer is added).

### 3.2 Multiple Profiles

[] Support for different profiles (e.g., "Powerlifting", "Yoga") to keep workout histories separate.
