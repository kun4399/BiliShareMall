package auth

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/kun4399/BiliShareMall/internal/dao"
	_ "github.com/mattn/go-sqlite3"
)

func TestGetSharedLoginSessionReturnsLoggedInWhenPersistedCookiesExist(t *testing.T) {
	database := newAuthTestDatabase(t)
	if err := database.EnsureAuthSessionTable(); err != nil {
		t.Fatalf("EnsureAuthSessionTable error: %v", err)
	}
	if err := database.SaveAuthSession("SESSDATA=abc; DedeUserID=1; bili_jct=token"); err != nil {
		t.Fatalf("SaveAuthSession error: %v", err)
	}

	service := NewService(database)
	session, err := service.GetSharedLoginSession()
	if err != nil {
		t.Fatalf("GetSharedLoginSession error: %v", err)
	}

	if !session.LoggedIn {
		t.Fatal("expected persisted shared session to be logged in")
	}
	if session.UpdatedAt == 0 {
		t.Fatal("expected updatedAt to be populated")
	}
}

func newAuthTestDatabase(t *testing.T) *dao.Database {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "auth.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite error: %v", err)
	}

	database := &dao.Database{Db: rawDB}
	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}
