package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"

	_ "modernc.org/sqlite"
)

func createDb() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "relay.db")
	if err != nil {
		return nil, err
	}
	query := `
		CREATE TABLE IF NOT EXISTS api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		key TEXT UNIQUE NOT NULL,
		is_active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func validateKey(db *sql.DB, key string) (string, bool) {
	if key == "" {
		return "", false
	}

	var name string
	query := "SELECT name FROM api_keys WHERE key = ? AND is_active = 1"
	err := db.QueryRow(query, key).Scan(&name)
	if err != nil {
		return "", false
	}

	return name, true
}

func addKey(db *sql.DB, name string) (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	key := "rk_" + hex.EncodeToString(bytes)
	query := "INSERT INTO api_keys (name, key) VALUES (?, ?)"
	_, err := db.Exec(query, name, key)
	if err != nil {
		return "", err
	}

	return key, nil
}
