package dao

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mikumifa/BiliShareMall/internal/util"
	"runtime"
	"sync"
)

type Database struct {
	Db *sql.DB
}

var (
	registerSQLiteDriverOnce sync.Once
	registerSQLiteDriverErr  error
)

func NewDatabase(dbPath string) (*Database, error) {

	var extension string
	switch runtime.GOOS {
	case "darwin": // macOS
		extension = util.GetPath("dict/libsimple-osx-x64/libsimple")
	case "windows": //windows
		extension = "dict/libsimple-windows-x64/simple"
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	registerSQLiteDriverOnce.Do(func() {
		sql.Register("sqlite3_simple",
			&sqlite3.SQLiteDriver{
				Extensions: []string{
					extension,
				},
			})
	})
	if registerSQLiteDriverErr != nil {
		return nil, registerSQLiteDriverErr
	}

	db, err := sql.Open("sqlite3_simple", dbPath)
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
