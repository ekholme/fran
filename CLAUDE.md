# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Fran is a minimalist terminal workout tracker written in Go, built with Bubble Tea (charmbracelet TUI framework) and backed by SQLite.

## Commands

Both a `Taskfile.yml` (go-task) and a `justfile` define the same tasks; use whichever runner is available.

```
task build            # or: just build   — go mod tidy, then build ./cmd/fran.go into bin/fran
task test             # or: just test    — go test -v -race ./...
task tidy             # or: just tidy    — go mod tidy
task clean            # or: just clean   — remove bin/ and clear the go test cache
task create-tables    # or: just create-tables — apply sqlite/migration.sql to fran.db via sqlite3 CLI
```

To run a single test: `go test -v -race -run TestName ./...` (adjust the package path once tests exist; there are currently no `*_test.go` files in the repo).

There is no lint task defined; use `go vet ./...` / `gofmt` conventions.

To run the app directly: `go run ./cmd/fran.go`. The database path defaults to `./fran.db` and can be overridden with the `FRAN_DB_PATH` env var (the `justfile` loads a `.env` via `set dotenv-load`).

## Architecture

Three packages, each with a single, distinct responsibility:

- **`fran` (root package, `workout.go`)** — defines the core `Workout` domain struct (`ID`, `ShortDescription`, `LongDescription`, `WorkoutDate`, `CreatedAt`) with `db` struct tags. This is the shared data type imported by both other packages.
- **`sqlite` (`sqlite/storage.go`)** — the persistence layer. `Storage` wraps a `*sql.DB` and implements CRUD against a single `workouts` table (schema in `sqlite/migration.sql`). All methods take a `context.Context` and return `*fran.Workout` / `[]*fran.Workout`, wrapping errors with `fmt.Errorf("...: %w", err)`.
- **`ui` (`ui/workout.go`)** — the Bubble Tea TUI. `WorkoutModel` implements `tea.Model` (`Init`/`Update`/`View`) and is the entire application UI.
- **`cmd/fran.go`** — the entrypoint (`package main`): opens the sqlite DB, constructs `sqlite.NewStorage`, wraps it in `ui.NewWorkoutModel`, and runs the Bubble Tea program with `tea.WithAltScreen()`.

### UI state machine

`WorkoutModel` is a single Elm-architecture model with a `sessionState` enum driving three screens:

- `stateWorkoutList` — a `bubbles/list` of recent workouts (loaded via `GetLatestWorkouts`); `n` starts workout creation, `/` filters, `enter` opens the selected workout.
- `stateCreateWorkout` — a form with three inputs (`ShortDescriptionInput`, `LongDescriptionInput` textarea, `WorkoutDateInput`), tab-cycled via `inputFocusIndex`; `ctrl+s` validates the date (`YYYY-MM-DD` via `time.Parse("2006-01-02", ...)`) and saves.
- `stateViewWorkout` — read-only detail view with `left/h` and `right/l` to step through `allWorkouts` (the in-memory list loaded for the list screen, not re-fetched from the DB) and jump to adjacent workouts by index.

Each state has a matching `update*` method (`updateWorkoutList`, `updateWorkoutForm`, `updateViewWorkout`) and `view*` method (`viewCreateWorkout`, `viewWorkout`), dispatched from the top-level `Update`/`View`. Async DB work goes through `tea.Cmd`-returning functions (`fetchLatestWorkoutsCmd`, `createWorkoutCmd`, `fetchWorkoutCmd`) that resolve to typed messages (`workoutsLoadedMsg`, `workoutCreatedMsg`, `workoutLoadedMsg`) handled in the top-level `Update`; any other error is returned as a bare `error` message and short-circuits into the error view.

### Store interfaces

`ui` defines narrow interfaces (`WorkoutGetter`, `WorkoutCreator`, `WorkoutDeleter`, composed into `WorkoutStore`) rather than depending on `*sqlite.Storage` directly, so `sqlite.Storage` is passed into `ui.NewWorkoutModel` as an interface value — keep new storage methods declared on the interface the UI actually needs, not assumed from the concrete type.

## Roadmap

`PLAN.md` tracks a phased improvement roadmap (validation/UX, then data model extensions like tags and sets/reps, then advanced features) with checkboxes marking what's done — check it before starting new feature work to see what's already planned and its intended phase.
