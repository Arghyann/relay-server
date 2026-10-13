package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

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

func listAllKeys(db *sql.DB) error {
	rows, err := db.Query("SELECT id, name, key, is_active, created_at FROM api_keys ORDER BY id ASC")
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("%-4s | %-15s | %-52s | %-8s\n", "ID", "NAME", "KEY", "STATUS")
	fmt.Println(strings.Repeat("-", 85))

	for rows.Next() {
		var id, isActive int
		var name, key, createdAt string
		if err := rows.Scan(&id, &name, &key, &isActive, &createdAt); err != nil {
			continue
		}
		status := "active"
		if isActive == 0 {
			status = "revoked"
		}
		fmt.Printf("%-4d | %-15s | %-52s | %-8s\n", id, name, key, status)
	}
	return nil
}
