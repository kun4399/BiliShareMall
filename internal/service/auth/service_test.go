package auth

import (
	"database/sql"
	"path/filepath"
	"strings"
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

func TestListLoginAccountsReturnsLoggedInState(t *testing.T) {
	database := newAuthTestDatabase(t)
	if err := database.EnsureAuthAccountsTable(); err != nil {
		t.Fatalf("EnsureAuthAccountsTable error: %v", err)
	}
	if _, err := database.UpsertAuthAccount("1001", "账号A", "SESSDATA=abc; DedeUserID=1001; bili_jct=token"); err != nil {
		t.Fatalf("UpsertAuthAccount error: %v", err)
	}

	service := NewService(database)
	accounts, err := service.ListLoginAccounts()
	if err != nil {
		t.Fatalf("ListLoginAccounts error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if !accounts[0].LoggedIn {
		t.Fatal("expected account logged in")
	}
}

func TestDeleteLoginAccountSyncsDefaultSession(t *testing.T) {
	database := newAuthTestDatabase(t)
	if err := database.EnsureAuthAccountsTable(); err != nil {
		t.Fatalf("EnsureAuthAccountsTable error: %v", err)
	}
	if err := database.EnsureAuthSessionTable(); err != nil {
		t.Fatalf("EnsureAuthSessionTable error: %v", err)
	}
	id1, err := database.UpsertAuthAccount("1001", "账号A", "SESSDATA=aaa; DedeUserID=1001; bili_jct=t")
	if err != nil {
		t.Fatalf("UpsertAuthAccount #1 error: %v", err)
	}
	if _, err := database.UpsertAuthAccount("1002", "账号B", "SESSDATA=bbb; DedeUserID=1002; bili_jct=t"); err != nil {
		t.Fatalf("UpsertAuthAccount #2 error: %v", err)
	}
	if err := database.SaveAuthSession("SESSDATA=aaa; DedeUserID=1001; bili_jct=t"); err != nil {
		t.Fatalf("SaveAuthSession error: %v", err)
	}

	service := NewService(database)
	if err := service.DeleteLoginAccount(id1); err != nil {
		t.Fatalf("DeleteLoginAccount error: %v", err)
	}

	session, err := database.GetAuthSession()
	if err != nil {
		t.Fatalf("GetAuthSession error: %v", err)
	}
	if session.Cookies == "" || !strings.Contains(session.Cookies, "DedeUserID=1002") {
		t.Fatalf("expected default session switched to account 1002, got %q", session.Cookies)
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
