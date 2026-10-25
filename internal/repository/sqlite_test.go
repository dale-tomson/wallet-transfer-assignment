package repository

import (
	"path/filepath"
	"testing"
)

func TestNewSQLiteDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_wallet.db")

	db, err := NewSQLiteDB(dbPath)
	if err != nil {
		t.Fatalf("expected NewSQLiteDB to succeed, got error: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	tables := []string{"wallets", "transfers", "ledger_entries", "idempotency_records"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s was not created: %v", table, err)
		}
	}
}
