package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		name TEXT,
		created TEXT DEFAULT (datetime('now')),
		updated TEXT DEFAULT (datetime('now'))
	) STRICT;

	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		created TEXT DEFAULT (datetime('now')),
		updated TEXT DEFAULT (datetime('now')),
		FOREIGN KEY (user_id) REFERENCES users(id)
	) STRICT;

	CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects(user_id);
	`

	_, err := db.Exec(schema)
	return err
}
