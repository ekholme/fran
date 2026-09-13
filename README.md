# fran

A minimalist terminal workout tracker, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and backed by SQLite.

Log AMRAP, EMOM, Cardio, and Strength workouts (plus a free-text "Other" catch-all), browse your history, and edit or delete past entries — all from the terminal.

## Requirements

- Go 1.25+
- [`just`](https://github.com/casey/just) as the command runner

## Setup

```sh
just build   # go mod tidy, then build ./cmd/fran.go into bin/fran
./bin/fran
```

Or run it directly without building a binary:

```sh
go run ./cmd/fran.go
```

The database and its schema are created automatically on first run — no separate setup step needed. The database path defaults to `$XDG_DATA_HOME/fran/fran.db` (or `~/.local/share/fran/fran.db` if `XDG_DATA_HOME` isn't set); override it with the `FRAN_DB_PATH` environment variable (or a `.env` file, which `just` loads automatically).

## Usage

- **List view**: `n` new workout, `/` search by short description (searches your full history, not just what's on screen — `enter` to run it, `esc` to cancel/clear), `enter` open a workout, `q` quit
- **View workout**: `h`/`left` and `l`/`right` step to the previous/next workout, `e` edit, `d` delete (then `y`/`n` to confirm), `esc`/`q` back to list
- **Create/edit form**: `tab` cycle fields, `ctrl+t` cycle workout type, `ctrl+s` save, `esc` cancel — Strength workouts also support `ctrl+a` to add an exercise and `ctrl+r` to remove the last one

## Other commands

```sh
just test            # run tests with the race detector
just tidy            # go mod tidy
just clean           # remove bin/ and clear the go test cache
just create-tables   # apply sqlite/migration.sql to ./fran.db via the sqlite3 CLI (requires it
                      # installed separately) — useful for inspecting the schema by hand; not
                      # required to run the app
```

See `CLAUDE.md` for architecture notes and `PLAN.md` for the improvement roadmap.
