package server

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func OpenDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return err
	}

	entries, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		var version int
		if _, err := fmt.Sscanf(entry, "migrations/%d_", &version); err != nil {
			return fmt.Errorf("parse migration %s: %w", entry, err)
		}

		var exists int
		if err := db.QueryRow(`SELECT 1 FROM schema_migrations WHERE version = ?`, version).Scan(&exists); err == nil {
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		query, err := migrationsFS.ReadFile(entry)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(query)); err != nil {
			return fmt.Errorf("run migration %s: %w", entry, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, version); err != nil {
			return err
		}
	}
	return nil
}

func InitAdmin(db *sql.DB, username, password string) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errors.New("users table is not empty")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO users(username, password_hash, is_admin) VALUES (?, ?, 1)`,
		username, string(passwordHash),
	)
	return err
}
