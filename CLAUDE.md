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

To run the app directly: `go run ./cmd/fran.go`. The database path defaults to the XDG data directory (`$XDG_DATA_HOME/fran/fran.db`, or `~/.local/share/fran/fran.db` if `XDG_DATA_HOME` isn't set — see `defaultDBPath` in `cmd/fran.go`) and can be overridden with the `FRAN_DB_PATH` env var (the `justfile` loads a `.env` via `set dotenv-load`, handy for pointing local dev at the repo-root `fran.db` that `just create-tables` creates). `cmd/fran.go` creates the database's parent directory if missing before opening it.

## Architecture

Three packages, each with a single, distinct responsibility:

- **`fran` (root package, `workout.go`)** — defines the core `Workout` domain struct (`ID`, `ShortDescription`, `LongDescription`, `WorkoutDate`, `WorkoutType`, `CreatedAt`) with `db` struct tags. `WorkoutType` is a string enum (`WorkoutTypeAMRAP`/`EMOM`/`Cardio`/`Strength`/`Other`, listed in cycling order in `WorkoutTypes`) that determines which single type-specific detail struct on `Workout` is populated: `AMRAP *AMRAPDetails`, `EMOM *EMOMDetails`, `Cardio *CardioDetails`, or `Strength *StrengthDetails` (a `StrengthDetails` holds a *list* of `StrengthExercise`, since one Strength session can cover several named movements — sets/reps/weight/unit each). `Other` has no structured details; its specifics live in the free-text `LongDescription`. `Workout.MarshalDetails`/`UnmarshalDetails` convert whichever detail struct applies to/from JSON (returning `nil, nil` / no-op for `Other`, and erroring on an unrecognized `WorkoutType`) — this is the shared type imported by both other packages.
- **`sqlite` (`sqlite/storage.go`, `sqlite/migrate.go`)** — the persistence layer. `Storage` wraps a `*sql.DB` and implements CRUD against a single `workouts` table (schema in `sqlite/migration.sql`), storing type-specific details as a JSON blob in a nullable `details` column (`nullableDetails`/`applyDetails` helpers wrap `Workout.MarshalDetails`/`UnmarshalDetails` for the nullable-column round trip). All methods take a `context.Context` and return `*fran.Workout` / `[]*fran.Workout`, wrapping errors with `fmt.Errorf("...: %w", err)`. `EnsureSchema(db)` runs at startup (from `cmd/fran.go`) and handles every starting state in one idempotent call: it first executes `migration.sql` (embedded via `go:embed`) to bootstrap the table/indexes on a brand-new database (a no-op on an existing one, since it's all `IF NOT EXISTS`), then adds the `workout_type`/`details` columns via `ALTER TABLE`, swallowing the resulting "duplicate column name" error on a database that already has them — this is the only migration mechanism in the app; there's no versioned migration runner, so a schema change means extending `EnsureSchema` and the canonical `CREATE TABLE` in `migration.sql` together. Note `idx_workouts_type` is created by `EnsureSchema` itself (in Go), not inside `migration.sql`, specifically because it must run *after* the `ALTER TABLE` — a database predating the `workout_type` column doesn't have that column yet when `migration.sql` runs.
- **`ui` (`ui/workout.go`, `ui/calendar.go`)** — the Bubble Tea TUI. `WorkoutModel` implements `tea.Model` (`Init`/`Update`/`View`) and is the entire application UI; the calendar screen's update/view/commands live in `calendar.go`.
- **`cmd/fran.go`** — the entrypoint (`package main`): opens the sqlite DB, calls `sqlite.EnsureSchema`, constructs `sqlite.NewStorage`, wraps it in `ui.NewWorkoutModel`, and runs the Bubble Tea program with `tea.WithAltScreen()`.

### UI state machine

`WorkoutModel` is a single Elm-architecture model with a `sessionState` enum driving four screens:

- `stateWorkoutList` — a `bubbles/list` of recent workouts (loaded via `GetLatestWorkouts`); each item's `Description()` shows the workout's type alongside its date. `n` starts workout creation, `enter` opens the selected workout. `/` opens a dedicated search box (`m.searchInput`, driven by `updateSearchInput`) rather than the list's own built-in filtering, which is explicitly disabled (`SetFilteringEnabled(false)`) since it only ever matches the already-loaded page; `enter` while searching dispatches `searchWorkoutsCmd` (a real `GetWorkoutsByShortDesc` query, resolving to `workoutsSearchedMsg`) — an empty query is treated as "show all" and re-fetches latest instead. `esc` while browsing search results (`m.searchActive`) clears back to the latest-workouts list; `esc` while still typing a query just cancels the search box.
- `stateCreateWorkout` — a form with the three fixed inputs (`ShortDescriptionInput`, `LongDescriptionInput` textarea, `WorkoutDateInput`) plus a dynamic `WorkoutType` selector and `inputs.DetailFields []detailField`, all tab-cycled via `inputFocusIndex` (modulus `3 + len(DetailFields)`). `ctrl+t` cycles `m.WorkoutType` through `fran.WorkoutTypes` and rebuilds `DetailFields` via `newDetailFields` to match the new type's field set (AMRAP: 3 fields, EMOM: 2, Cardio: 4, Strength: a repeatable group of 5 per exercise, Other: 0) — fixed fields are left untouched so in-progress typing survives a type change. For Strength specifically, `ctrl+a`/`ctrl+r` append/remove one 5-field exercise group (`strengthExerciseGroup`), tracked by `m.strengthExerciseCount` (floor of 1); a no-op outside Strength. `ctrl+s` validates (non-empty short description, date matches `YYYY-MM-DD` via `isDateValid`, each populated detail field valid for its `detailFieldKind`) and calls `applyDetailFieldsToWorkout` to parse `DetailFields` into the concrete typed struct for `w.WorkoutType` before saving — a fully-blank trailing Strength exercise slot is silently dropped rather than persisted. The same form doubles as the edit form: `m.isEditing`/`m.editingID` (set when entering from `stateViewWorkout` via `e`, which also pre-fills `WorkoutType` and `DetailFields` via `newDetailFieldsFromWorkout`) decide whether `ctrl+s` dispatches `createWorkoutCmd` or `updateWorkoutCmd`; both entry points (`n` from the list, `esc` from the form) reset `WorkoutType` to `Other`, clear `DetailFields`, and reset `strengthExerciseCount` to 1 so edit/type state never bleeds into a fresh create.
- `stateCalendar` — a Monday-first month grid (`viewCalendar`/`updateCalendar` in `ui/calendar.go`) opened with `c` from the list. Each day cell shows single-letter type badges (`workoutTypeBadge`) for its workouts, and the selected day's workouts are listed below the grid. The month is loaded via `fetchCalendarMonthCmd` (a `GetWorkoutsByDateRange` query for the first→last day, resolving to `calendarLoadedMsg`, which is dropped if it's for a month the user has already navigated away from) into `m.calendarWorkouts`; `m.calendarMonth` is the first of the shown month and `m.calendarCursor` the selected day. `hjkl`/arrows move by day/week (refetching only when crossing a month boundary), `[`/`]` change month, `t` jumps to today, `tab` cycles `m.calendarDayIndex` through the selected day's workouts, `enter` opens that workout, `n` opens the create form pre-filled with the selected date, and `esc`/`c` return to the list (re-fetching latest, since list data isn't refreshed while in the calendar). Its clock is the package-level `now` var so tests can pin it.
- `stateViewWorkout` — read-only detail view with `left/h` and `right/l` to step through `browseWorkouts()` — `allWorkouts` (the in-memory list loaded for the list screen, not re-fetched from the DB) or `calendarWorkouts` when opened from the calendar — and jump to adjacent workouts by index; type-specific fields are rendered by `renderWorkoutTypeDetails` (a visible `"(missing structured data)"` placeholder if `WorkoutType` claims a structured type but its detail pointer is `nil`, rather than panicking). `e` jumps into `stateCreateWorkout` pre-filled for editing, `d` sets `m.confirmDelete` and the next keypress must be `y`/`n` (any other key while confirming is swallowed) before `deleteWorkoutCmd` runs.

`m.returnState` records which browse screen (list or calendar) the view and form screens were entered from; leaving them (`esc`, or after a save/delete) goes back there, re-fetching that screen's data. `resetWorkoutForm` is the single place the form's fields and edit/type state get cleared.

Each state has a matching `update*` method (`updateWorkoutList`, `updateWorkoutForm`, `updateViewWorkout`, `updateCalendar`) and `view*` method (`viewCreateWorkout`, `viewWorkout`, `viewCalendar`), dispatched from the top-level `Update`/`View`. Async DB work goes through `tea.Cmd`-returning functions (`fetchLatestWorkoutsCmd`, `createWorkoutCmd`, `updateWorkoutCmd`, `deleteWorkoutCmd`, `fetchWorkoutCmd`, `fetchCalendarMonthCmd`) that resolve to typed messages (`workoutsLoadedMsg`, `workoutCreatedMsg`, `workoutUpdatedMsg`, `workoutDeletedMsg`, `workoutLoadedMsg`, `calendarLoadedMsg`) handled in the top-level `Update`; any other error is returned as a bare `error` message and short-circuits into the error view.

### Store interfaces

`ui` defines narrow interfaces (`WorkoutGetter`, `WorkoutCreator`, `WorkoutDeleter`, `WorkoutUpdater`, composed into `WorkoutStore`) rather than depending on `*sqlite.Storage` directly, so `sqlite.Storage` is passed into `ui.NewWorkoutModel` as an interface value — keep new storage methods declared on the interface the UI actually needs, not assumed from the concrete type.

### Testing conventions

Each package tests against real dependencies rather than mocks where practical: `sqlite/storage_test.go` and `sqlite/migrate_test.go` run against a temp on-disk SQLite database (`openTestDB`, shared between both files) rather than mocking `*sql.DB`. `ui/workout_test.go` drives `WorkoutModel.Update` with real `tea.KeyMsg`s against a fake in-memory `WorkoutStore` (`fakeStore`); its `send`/`execCmd` helpers only chase a returned `tea.Cmd` when it resolves quickly *and* is one of the app's own message types (see `isDomainMsg`) — never invoke an arbitrary returned `tea.Cmd` and wait on it, since bubbles' cursor-blink command blocks on a real ~530ms timer by design.

## Roadmap

`PLAN.md` tracks a phased improvement roadmap (validation/UX, then data model extensions like tags and sets/reps, then advanced features) with checkboxes marking what's done — check it before starting new feature work to see what's already planned and its intended phase.
