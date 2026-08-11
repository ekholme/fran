package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ekholme/fran/sqlite"
	"github.com/ekholme/fran/ui"
	_ "modernc.org/sqlite"
)

func main() {
	dbPath := "fran.db"
	if envPath := os.Getenv("FRAN_DB_PATH"); envPath != "" {
		dbPath = envPath
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	store := sqlite.NewStorage(db)
	model := ui.NewWorkoutModel(store, 50)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas! The program failed: %v", err)
		os.Exit(1)
	}
}
