package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dale-tomson/wallet-transfer-assignment/migrations"
	_ "modernc.org/sqlite"
)

var initMigrationSQL = migrations.InitSQL

// NewSQLiteDB initializes a SQLite database connection pool optimized for concurrent execution
func NewSQLiteDB(dsn string) (*sql.DB, error) {
	if dsn == "" {
		dsn = "wallet.db"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite connection: %w", err)
	}

	// Serializes writes through one connection to eliminate database file lock contention
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Apply critical performance & integrity PRAGMAs
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA synchronous = NORMAL;",
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to execute %q: %w", pragma, err)
		}
	}

	// Run initial schema migration
	if _, err := db.ExecContext(ctx, initMigrationSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run initial migration: %w", err)
	}

	return db, nil
}
