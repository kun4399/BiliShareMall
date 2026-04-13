package dao

import (
	"context"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	Db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	return &Database{Db: db}, nil
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
