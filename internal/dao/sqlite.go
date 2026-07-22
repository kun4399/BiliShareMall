package dao

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const sqliteBusyTimeoutMillis = 5000

const (
	sqliteMaxOpenConnections = 8
	sqliteMaxIdleConnections = 4
)

type Database struct {
	Db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", sqliteConnectionString(dbPath))
	if err != nil {
		return nil, err
	}
	if err := configureSQLite(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Database{Db: db}, nil
}

func configureSQLite(db *sql.DB) error {
	ctx := context.Background()

	// WAL allows readers to keep serving the web UI while crawler writes are in
	// progress. A single connection defeats that property because database/sql
	// queues every operation behind the connection currently in use.
	db.SetMaxOpenConns(sqliteMaxOpenConnections)
	db.SetMaxIdleConns(sqliteMaxIdleConnections)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", sqliteBusyTimeoutMillis)); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("read sqlite journal mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
			return fmt.Errorf("enable sqlite wal mode: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = NORMAL"); err != nil {
		return fmt.Errorf("set sqlite synchronous mode: %w", err)
	}
	return nil
}

func sqliteConnectionString(dbPath string) string {
	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	return dbPath + separator +
		"_busy_timeout=" + fmt.Sprint(sqliteBusyTimeoutMillis) +
		"&_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL"
}

func (d *Database) Init(initSql string) error {
	_, err := d.Db.ExecContext(context.Background(), initSql)
	return err
}
func (d *Database) UpdateVersion(versionId int) (err error) {
	_, err = d.Db.Exec("UPDATE version SET version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1", versionId)
	return err
}

func (d *Database) GetVersion() (version int, err error) {
	row := d.Db.QueryRow("SELECT version FROM version WHERE id = 1")
	err = row.Scan(&version)
	return version, err
}
func (d *Database) Close() error {
	return d.Db.Close()
}
