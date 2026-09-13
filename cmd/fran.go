package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ekholme/fran/sqlite"
	"github.com/ekholme/fran/ui"
	_ "modernc.org/sqlite"
)

// defaultDBPath returns fran.db's location per the XDG Base Directory
// spec: $XDG_DATA_HOME/fran/fran.db, or ~/.local/share/fran/fran.db if
// XDG_DATA_HOME isn't set.
func defaultDBPath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "fran.db"
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "fran", "fran.db")
}

func main() {
	dbPath := defaultDBPath()
	if envPath := os.Getenv("FRAN_DB_PATH"); envPath != "" {
		dbPath = envPath
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("failed to create database directory: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := sqlite.EnsureSchema(db); err != nil {
		log.Fatalf("failed to ensure database schema: %v", err)
	}

	store := sqlite.NewStorage(db)
	model := ui.NewWorkoutModel(store, 50)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas! The program failed: %v", err)
		os.Exit(1)
	}
}
