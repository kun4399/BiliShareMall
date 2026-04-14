package app

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"

	"github.com/kun4399/BiliShareMall/internal/domain"
)

func TestInitializeAllowsSecondAppAgainstSharedCurrentDatabase(t *testing.T) {
	dbPath := prepareTestAppEnv(t)

	first := NewApp()
	if err := first.Initialize(); err != nil {
		t.Fatalf("first Initialize error: %v", err)
	}
	t.Cleanup(func() {
		if first.d != nil {
			_ = first.d.Close()
		}
	})

	second := NewApp()
	if err := second.Initialize(); err != nil {
		t.Fatalf("expected second Initialize against %s to succeed, got %v", dbPath, err)
	}
	t.Cleanup(func() {
		if second.d != nil {
			_ = second.d.Close()
		}
	})
}

func TestInitializeSkipsSetupWritesWhenSchemaIsCurrent(t *testing.T) {
	prepareTestAppEnv(t)

	first := NewApp()
	if err := first.Initialize(); err != nil {
		t.Fatalf("first Initialize error: %v", err)
	}
	t.Cleanup(func() {
		if first.d != nil {
			_ = first.d.Close()
		}
	})

	updatedAtBefore := queryVersionUpdatedAt(t, first.d.Db)

	time.Sleep(1100 * time.Millisecond)

	second := NewApp()
	if err := second.Initialize(); err != nil {
		t.Fatalf("second Initialize error: %v", err)
	}
	t.Cleanup(func() {
		if second.d != nil {
			_ = second.d.Close()
		}
	})

	updatedAtAfter := queryVersionUpdatedAt(t, first.d.Db)
	if updatedAtAfter != updatedAtBefore {
		t.Fatalf("expected current-schema startup to avoid version writes, updated_at changed from %q to %q", updatedAtBefore, updatedAtAfter)
	}
}

func TestInitializeRecreatesLegacyDatabase(t *testing.T) {
	dbPath := prepareTestAppEnv(t)

	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open legacy sqlite error: %v", err)
	}

	if _, err := rawDB.Exec(`
CREATE TABLE version
(
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	version    INTEGER NOT NULL,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO version(id, version, updated_at)
VALUES (1, 1, CURRENT_TIMESTAMP);
`); err != nil {
		_ = rawDB.Close()
		t.Fatalf("create legacy version table error: %v", err)
	}

	if err := rawDB.Close(); err != nil {
		t.Fatalf("close legacy sqlite error: %v", err)
	}

	instance := NewApp()
	if err := instance.Initialize(); err != nil {
		t.Fatalf("Initialize legacy database error: %v", err)
	}
	t.Cleanup(func() {
		if instance.d != nil {
			_ = instance.d.Close()
		}
	})

	version, err := instance.d.GetVersion()
	if err != nil {
		t.Fatalf("GetVersion after recreate error: %v", err)
	}
	if version != DatabaseVersion {
		t.Fatalf("expected recreated database version %d, got %d", DatabaseVersion, version)
	}

	var cookies string
	if err := instance.d.Db.QueryRow(`SELECT cookies FROM auth_session WHERE id = 1`).Scan(&cookies); err != nil {
		t.Fatalf("expected auth_session table after recreate, got %v", err)
	}
}

func prepareTestAppEnv(t *testing.T) string {
	t.Helper()

	_, _, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path failed")
	}

	baseDir := t.TempDir()
	dataDir := t.TempDir()
	dictDir := filepath.Join(baseDir, "dict")
	if err := os.MkdirAll(dictDir, 0o755); err != nil {
		t.Fatalf("create dict dir error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dictDir, "init.sql"), []byte(testInitSchemaSQL), 0o644); err != nil {
		t.Fatalf("write init.sql error: %v", err)
	}

	originalBasePath := domain.Env.BasePath
	originalDataPath := domain.Env.DataPath
	originalAppName := domain.Env.AppName

	domain.Env.BasePath = baseDir
	domain.Env.DataPath = dataDir
	domain.Env.AppName = "BiliShareMall"

	t.Cleanup(func() {
		domain.Env.BasePath = originalBasePath
		domain.Env.DataPath = originalDataPath
		domain.Env.AppName = originalAppName
	})

	return filepath.Join(dataDir, "bsm.db")
}

func queryVersionUpdatedAt(t *testing.T, db *sql.DB) string {
	t.Helper()

	var updatedAt string
	if err := db.QueryRow(`SELECT updated_at FROM version WHERE id = 1`).Scan(&updatedAt); err != nil {
		t.Fatalf("query version updated_at error: %v", err)
	}
	if updatedAt == "" {
		t.Fatal("expected non-empty version updated_at")
	}
	return updatedAt
}

func TestIsSQLiteLockedError(t *testing.T) {
	if !isSQLiteLockedError(fmt.Errorf("database is locked")) {
		t.Fatal("expected database is locked to be detected")
	}
	if !isSQLiteLockedError(fmt.Errorf("database table is locked: version")) {
		t.Fatal("expected database table is locked to be detected")
	}
	if isSQLiteLockedError(fmt.Errorf("some other startup failure")) {
		t.Fatal("expected unrelated error not to be detected as sqlite lock")
	}
}

const testInitSchemaSQL = `
CREATE TABLE IF NOT EXISTS version
(
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	version    INTEGER NOT NULL,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT OR REPLACE INTO version (id, version, updated_at)
VALUES (1, 1, CURRENT_TIMESTAMP);

CREATE TABLE IF NOT EXISTS c2c_items
(
	c2c_items_id      INTEGER PRIMARY KEY,
	c2c_items_name    TEXT NOT NULL,
	reference_price   INTEGER NOT NULL DEFAULT 0,
	show_market_price TEXT,
	normalized_status TEXT NOT NULL DEFAULT '在售'
);

CREATE TABLE IF NOT EXISTS monitor_rules
(
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	sku_id    INTEGER NOT NULL,
	min_price INTEGER NOT NULL,
	max_price INTEGER NOT NULL,
	enabled   INTEGER NOT NULL DEFAULT 1
);
`
