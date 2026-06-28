// Package sqlite handles interactions with an sqlite3 database
package sqlite

import "database/sql"

type Storage struct {
	db *sql.DB
}

// NewStorage creates a new storage object
func NewStorage(db *sql.DB) *Storage {
	return &Storage{
		db: db,
	}
}
