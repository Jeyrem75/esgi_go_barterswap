package main

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL non définie, test ignoré")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("ouverture DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("DB inaccessible: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
