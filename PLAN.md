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
[] Add a way to filter the list by `WorkoutType` (e.g. "show only Cardio") now that every workout carries one — natural extension of the DB-backed search above once it exists.
[x] Add a search bar or a toggleable filter at the top of the list to quickly find specific workouts.
[x] Add "Previous" and "Next" navigation between workouts in the "View Workout" state. Note: this navigates within `allWorkouts`, the currently loaded page of `listLength` workouts, not the full history — revisit once pagination or real search lands.

### 1.3 Polished UI/UX

[x] Use `lipgloss` to add borders and consistent spacing to the "Create Workout" form.
[] Improve the "View Workout" layout to better handle long descriptions (e.g., using a scrollable area if it exceeds terminal height).
[] Add a "loading" spinner or progress indicator for database fetches.
[] Show visible labels next to each type-specific detail field in the create/edit form (`ui/workout.go`'s `detailField`) — today the only cue is placeholder text inside the `textinput`, which disappears once a value is typed, making a filled-in AMRAP/Cardio/Strength form harder to scan than the fixed fields above it.

### 1.4 Configuration & Defaults

[x] Support overriding the database path via the `FRAN_DB_PATH` environment variable.
[] Support a configuration file (e.g., `~/.config/fran/config.toml`) to manage settings like default list size and theme preferences.

### 1.5 Editing & Deletion

[x] Wire the existing `UpdateWorkout` storage method into the UI — an "e" keybinding from the "View Workout" state reopens the create-workout form pre-filled with the selected workout's data and saves via `UpdateWorkout` instead of `CreateWorkout`.
[x] Wire the existing `DeleteWorkout` storage method into the UI — a "d" keybinding from the "View Workout" state prompts "Delete this workout? (y/n)" before calling `DeleteWorkout`.

### 1.6 Testing

[x] Add unit tests for the JSON marshal/unmarshal round-trip of per-type workout details (`workout_test.go`) — covers all 5 `WorkoutType`s plus the unknown-type error path.
[x] Add unit tests for `sqlite.Storage` against a temp on-disk SQLite DB (`sqlite/storage_test.go`) — covers create/get-by-id/get-by-date/get-latest/search-by-description/update/delete, including the nullable-`details` marshal/scan path for both a populated type (Cardio/EMOM/Strength) and `WorkoutTypeOther`, and the not-found error path for update/delete.
[x] Add unit tests for `sqlite.EnsureSchema` (`sqlite/migrate_test.go`) — covers migrating a pre-migration schema (backfills `workout_type = 'other'`, `details` stays NULL, `idx_workouts_type` gets created), idempotent re-runs against an already-migrated database, and a no-op run against a freshly created canonical schema.
[x] Add unit tests for the `ui` package's `Update` logic (`ui/workout_test.go`) using a fake in-memory `WorkoutStore` — covers create validation (short description/date/numeric detail fields), `ctrl+t` type cycling rebuilding `DetailFields` while preserving the fixed fields, Strength's `ctrl+a`/`ctrl+r` add/remove-exercise flow (including the no-op on non-Strength types and the floor of 1 exercise), the edit flow's pre-fill via `newDetailFieldsFromWorkout`, and the delete-confirmation and quit key paths. Test helpers drive real `tea.KeyMsg`s through `Update` and only chase a command's result when it resolves quickly and is one of the app's own message types — bubbles' cursor-blink command blocks on a real ~530ms timer and must never be waited on.

## Phase 2: Data Model & Functionality Extensions

### 2.1 Richer Workout Data

[x] **Categories/Tags:** Added a `WorkoutType` (AMRAP/EMOM/Cardio/Strength/Other) instead of free-form tags, selectable in the create/edit form and shown in the list/detail views.
[x] **Sets and Reps:** Strength workouts now record structured sets/reps/weight/unit per exercise (see the multi-exercise item below); other types get their own structured fields (AMRAP time cap/rounds/extra reps, EMOM interval/duration, Cardio modality/duration/distance/unit) — all stored as JSON in a `details` column.
[] **Duration Tracking:** Add a field to track how long a workout lasted. Note: EMOM/Cardio now carry a type-specific duration and AMRAP has a time cap; a general duration field for Strength/Other is still open.
[x] **Multiple exercises per Strength entry:** `StrengthDetails` now holds a list of `StrengthExercise` (name/sets/reps/weight/unit) instead of one flat set of fields, so a session like "Pullup, Pushup, KBS" can be logged as three named movements. The create/edit form gets a repeatable sub-form (`ctrl+a` adds an exercise, `ctrl+r` removes the last). AMRAP/EMOM were left as-is — their numeric fields describe the whole timed circuit rather than a single movement, and the movement list for those still belongs in the free-text long description. Note: this changed `StrengthDetails`'s JSON shape; harmless here since the workouts table was cleared before this landed, but would need a data migration if applied after real Strength entries existed.

### 2.2 Statistics & Analytics

[] **Summary View:** A new state to view aggregate statistics (e.g., workouts per week, total volume). Now more tractable per-type — e.g. total Cardio distance, or Strength volume by summing `sets * reps * weight` across each entry's `Exercises` — straight from the structured `details` fields instead of parsing free text, though it still requires a query/aggregation layer that doesn't exist yet.
[] **Progress Tracking:** Simple visual indicators of progress over time for specific exercises. Feasible now that each `StrengthExercise` has a `Name` (e.g. chart weight over time for every entry named "Squat") — still needs a query/aggregation layer, and matching is an exact string match on `Name` with no normalization, so "Squat" and "squat" would count as different exercises.

### 2.3 Data Portability

[] **Export/Import:** Add commands to export workouts to JSON or CSV files for backup or sharing.

## Phase 3: Advanced Features (Optional)

### 3.1 Notifications

[] Integration with system notifications (e.g., `notify-send` or similar) for workout reminders (if a timer is added).

### 3.2 Multiple Profiles

[] Support for different profiles (e.g., "Powerlifting", "Yoga") to keep workout histories separate.
