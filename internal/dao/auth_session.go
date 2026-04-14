package dao

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type AuthSession struct {
	Cookies   string    `json:"cookies"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (d *Database) EnsureAuthSessionTable() error {
	_, err := d.Db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS auth_session
		(
			id         INTEGER PRIMARY KEY CHECK (id = 1),
			cookies    TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	)
	if err != nil {
		return err
	}

	_, err = d.Db.ExecContext(
		context.Background(),
		`INSERT OR IGNORE INTO auth_session(id, cookies) VALUES(1, '')`,
	)
	return err
}

func (d *Database) GetAuthSession() (AuthSession, error) {
	var session AuthSession
	var updatedAt string
	err := d.Db.QueryRowContext(
		context.Background(),
		`SELECT cookies, COALESCE(updated_at, CURRENT_TIMESTAMP)
		FROM auth_session
		WHERE id = 1`,
	).Scan(&session.Cookies, &updatedAt)
	if err == sql.ErrNoRows {
		return AuthSession{}, nil
	}
	if err != nil {
		return session, err
	}
	if updatedAt != "" {
		parsed, parseErr := time.ParseInLocation("2006-01-02 15:04:05", updatedAt, time.Local)
		if parseErr == nil {
			session.UpdatedAt = parsed
		}
	}
	return session, nil
}

func (d *Database) SaveAuthSession(cookies string) error {
	_, err := d.Db.ExecContext(
		context.Background(),
		`INSERT INTO auth_session(id, cookies, created_at, updated_at)
		VALUES(1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET cookies = excluded.cookies, updated_at = CURRENT_TIMESTAMP`,
		strings.TrimSpace(cookies),
	)
	return err
}

func (d *Database) ClearAuthSession() error {
	_, err := d.Db.ExecContext(
		context.Background(),
		`INSERT INTO auth_session(id, cookies, created_at, updated_at)
		VALUES(1, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET cookies = '', updated_at = CURRENT_TIMESTAMP`,
	)
	return err
}
