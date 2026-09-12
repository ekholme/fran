# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Fran is a minimalist terminal workout tracker written in Go, built with Bubble Tea (charmbracelet TUI framework) and backed by SQLite.

## Commands

Commands are run via the `justfile`; there is no other supported task runner.

```
just build            # go mod tidy, then build ./cmd/fran.go into bin/fran
just test             # go test -v -race ./...
just tidy             # go mod tidy
just clean            # remove bin/ and clear the go test cache
just create-tables    # apply sqlite/migration.sql to fran.db via sqlite3 CLI
```

To run a single test: `go test -v -race -run TestName ./...` (adjust the package path — tests live in `workout_test.go`, `sqlite/storage_test.go`, `sqlite/migrate_test.go`, and `ui/workout_test.go`).

There is no lint task defined; use `go vet ./...` / `gofmt` conventions.

To run the app directly: `go run ./cmd/fran.go`. The database path defaults to `./fran.db` and can be overridden with the `FRAN_DB_PATH` env var (the `justfile` loads a `.env` via `set dotenv-load`).

## Architecture

Three packages, each with a single, distinct responsibility:

- **`fran` (root package, `workout.go`)** — defines the core `Workout` domain struct (`ID`, `ShortDescription`, `LongDescription`, `WorkoutDate`, `WorkoutType`, `CreatedAt`) with `db` struct tags. `WorkoutType` is a string enum (`WorkoutTypeAMRAP`/`EMOM`/`Cardio`/`Strength`/`Other`, listed in cycling order in `WorkoutTypes`) that determines which single type-specific detail struct on `Workout` is populated: `AMRAP *AMRAPDetails`, `EMOM *EMOMDetails`, `Cardio *CardioDetails`, or `Strength *StrengthDetails` (a `StrengthDetails` holds a *list* of `StrengthExercise`, since one Strength session can cover several named movements — sets/reps/weight/unit each). `Other` has no structured details; its specifics live in the free-text `LongDescription`. `Workout.MarshalDetails`/`UnmarshalDetails` convert whichever detail struct applies to/from JSON (returning `nil, nil` / no-op for `Other`, and erroring on an unrecognized `WorkoutType`) — this is the shared type imported by both other packages.
- **`sqlite` (`sqlite/storage.go`, `sqlite/migrate.go`)** — the persistence layer. `Storage` wraps a `*sql.DB` and implements CRUD against a single `workouts` table (schema in `sqlite/migration.sql`), storing type-specific details as a JSON blob in a nullable `details` column (`nullableDetails`/`applyDetails` helpers wrap `Workout.MarshalDetails`/`UnmarshalDetails` for the nullable-column round trip). All methods take a `context.Context` and return `*fran.Workout` / `[]*fran.Workout`, wrapping errors with `fmt.Errorf("...: %w", err)`. `EnsureSchema(db)` runs at startup (from `cmd/fran.go`) to add the `workout_type`/`details` columns to a pre-existing database via `ALTER TABLE`, swallowing the resulting "duplicate column name" error on a database that's already current — this is the only migration mechanism in the app; there's no versioned migration runner, so a schema change means extending `EnsureSchema` and the canonical `CREATE TABLE` in `migration.sql` together.
- **`ui` (`ui/workout.go`)** — the Bubble Tea TUI. `WorkoutModel` implements `tea.Model` (`Init`/`Update`/`View`) and is the entire application UI.
- **`cmd/fran.go`** — the entrypoint (`package main`): opens the sqlite DB, calls `sqlite.EnsureSchema`, constructs `sqlite.NewStorage`, wraps it in `ui.NewWorkoutModel`, and runs the Bubble Tea program with `tea.WithAltScreen()`.

### UI state machine

`WorkoutModel` is a single Elm-architecture model with a `sessionState` enum driving three screens:

- `stateWorkoutList` — a `bubbles/list` of recent workouts (loaded via `GetLatestWorkouts`); each item's `Description()` shows the workout's type alongside its date. `n` starts workout creation, `/` filters, `enter` opens the selected workout.
- `stateCreateWorkout` — a form with the three fixed inputs (`ShortDescriptionInput`, `LongDescriptionInput` textarea, `WorkoutDateInput`) plus a dynamic `WorkoutType` selector and `inputs.DetailFields []detailField`, all tab-cycled via `inputFocusIndex` (modulus `3 + len(DetailFields)`). `ctrl+t` cycles `m.WorkoutType` through `fran.WorkoutTypes` and rebuilds `DetailFields` via `newDetailFields` to match the new type's field set (AMRAP: 3 fields, EMOM: 2, Cardio: 4, Strength: a repeatable group of 5 per exercise, Other: 0) — fixed fields are left untouched so in-progress typing survives a type change. For Strength specifically, `ctrl+a`/`ctrl+r` append/remove one 5-field exercise group (`strengthExerciseGroup`), tracked by `m.strengthExerciseCount` (floor of 1); a no-op outside Strength. `ctrl+s` validates (non-empty short description, date matches `YYYY-MM-DD` via `isDateValid`, each populated detail field valid for its `detailFieldKind`) and calls `applyDetailFieldsToWorkout` to parse `DetailFields` into the concrete typed struct for `w.WorkoutType` before saving — a fully-blank trailing Strength exercise slot is silently dropped rather than persisted. The same form doubles as the edit form: `m.isEditing`/`m.editingID` (set when entering from `stateViewWorkout` via `e`, which also pre-fills `WorkoutType` and `DetailFields` via `newDetailFieldsFromWorkout`) decide whether `ctrl+s` dispatches `createWorkoutCmd` or `updateWorkoutCmd`; both entry points (`n` from the list, `esc` from the form) reset `WorkoutType` to `Other`, clear `DetailFields`, and reset `strengthExerciseCount` to 1 so edit/type state never bleeds into a fresh create.
- `stateViewWorkout` — read-only detail view with `left/h` and `right/l` to step through `allWorkouts` (the in-memory list loaded for the list screen, not re-fetched from the DB) and jump to adjacent workouts by index; type-specific fields are rendered by `renderWorkoutTypeDetails` (a visible `"(missing structured data)"` placeholder if `WorkoutType` claims a structured type but its detail pointer is `nil`, rather than panicking). `e` jumps into `stateCreateWorkout` pre-filled for editing, `d` sets `m.confirmDelete` and the next keypress must be `y`/`n` (any other key while confirming is swallowed) before `deleteWorkoutCmd` runs.

Each state has a matching `update*` method (`updateWorkoutList`, `updateWorkoutForm`, `updateViewWorkout`) and `view*` method (`viewCreateWorkout`, `viewWorkout`), dispatched from the top-level `Update`/`View`. Async DB work goes through `tea.Cmd`-returning functions (`fetchLatestWorkoutsCmd`, `createWorkoutCmd`, `updateWorkoutCmd`, `deleteWorkoutCmd`, `fetchWorkoutCmd`) that resolve to typed messages (`workoutsLoadedMsg`, `workoutCreatedMsg`, `workoutUpdatedMsg`, `workoutDeletedMsg`, `workoutLoadedMsg`) handled in the top-level `Update`; any other error is returned as a bare `error` message and short-circuits into the error view.

### Store interfaces

`ui` defines narrow interfaces (`WorkoutGetter`, `WorkoutCreator`, `WorkoutDeleter`, `WorkoutUpdater`, composed into `WorkoutStore`) rather than depending on `*sqlite.Storage` directly, so `sqlite.Storage` is passed into `ui.NewWorkoutModel` as an interface value — keep new storage methods declared on the interface the UI actually needs, not assumed from the concrete type.

### Testing conventions

Each package tests against real dependencies rather than mocks where practical: `sqlite/storage_test.go` and `sqlite/migrate_test.go` run against a temp on-disk SQLite database (`openTestDB`, shared between both files) rather than mocking `*sql.DB`. `ui/workout_test.go` drives `WorkoutModel.Update` with real `tea.KeyMsg`s against a fake in-memory `WorkoutStore` (`fakeStore`); its `send`/`execCmd` helpers only chase a returned `tea.Cmd` when it resolves quickly *and* is one of the app's own message types (see `isDomainMsg`) — never invoke an arbitrary returned `tea.Cmd` and wait on it, since bubbles' cursor-blink command blocks on a real ~530ms timer by design.

## Roadmap

`PLAN.md` tracks a phased improvement roadmap (validation/UX, then data model extensions like tags and sets/reps, then advanced features) with checkboxes marking what's done — check it before starting new feature work to see what's already planned and its intended phase.
