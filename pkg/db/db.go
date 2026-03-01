// Solo - Personal AI Agent
// License: MIT

// Package db provides a process-wide singleton SQLite connection pool.
// All packages that need SQLite call db.Get(path) — they share one pool
// and never open their own connection.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; registered as "sqlite"
)

var (
	instance *sql.DB
	once     sync.Once
	initErr  error
)

// Get returns the process-wide shared SQLite connection pool, creating it on
// the first call. Subsequent calls with any path return the same pool — the
// path argument is only used during initialisation.
//
// The caller must never call db.Close() on the returned *sql.DB.
func Get(path string) (*sql.DB, error) {
	once.Do(func() {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			initErr = fmt.Errorf("db: cannot create data dir: %w", err)
			return
		}

		db, err := sql.Open("sqlite", path)
		if err != nil {
			initErr = fmt.Errorf("db: cannot open database %q: %w", path, err)
			return
		}

		db.SetMaxOpenConns(5)
		db.SetMaxIdleConns(2)
		db.SetConnMaxLifetime(30 * time.Minute)

		if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
			_ = err
		}

		instance = db
	})

	if initErr != nil {
		return nil, initErr
	}
	return instance, nil
}
