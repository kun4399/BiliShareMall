package dao

import "testing"

func TestAuthAccountCRUD(t *testing.T) {
	db := newTestDatabase(t)
	if err := db.EnsureAuthAccountsTable(); err != nil {
		t.Fatalf("EnsureAuthAccountsTable error: %v", err)
	}

	id1, err := db.UpsertAuthAccount("1001", "账号A", "SESSDATA=a;DedeUserID=1001")
	if err != nil {
		t.Fatalf("UpsertAuthAccount first error: %v", err)
	}
	id2, err := db.UpsertAuthAccount("1002", "账号B", "SESSDATA=b;DedeUserID=1002")
	if err != nil {
		t.Fatalf("UpsertAuthAccount second error: %v", err)
	}
	if id1 == id2 {
		t.Fatalf("expected different account ids, got %d", id1)
	}

	if _, err := db.UpsertAuthAccount("1001", "账号A2", "SESSDATA=a2;DedeUserID=1001"); err != nil {
		t.Fatalf("UpsertAuthAccount update error: %v", err)
	}

	accounts, err := db.ListAuthAccounts()
	if err != nil {
		t.Fatalf("ListAuthAccounts error: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}

	account, err := db.GetAuthAccountByID(id1)
	if err != nil {
		t.Fatalf("GetAuthAccountByID error: %v", err)
	}
	if account.AccountName != "账号A2" {
		t.Fatalf("expected updated account name, got %q", account.AccountName)
	}

	if err := db.DeleteAuthAccount(id2); err != nil {
		t.Fatalf("DeleteAuthAccount error: %v", err)
	}
	accounts, err = db.ListAuthAccounts()
	if err != nil {
		t.Fatalf("ListAuthAccounts after delete error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account after delete, got %d", len(accounts))
	}

	if err := db.ClearAuthAccounts(); err != nil {
		t.Fatalf("ClearAuthAccounts error: %v", err)
	}
	accounts, err = db.ListAuthAccounts()
	if err != nil {
		t.Fatalf("ListAuthAccounts after clear error: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected empty accounts after clear, got %d", len(accounts))
	}
}

func TestSyncLegacyAuthSessionToAccount(t *testing.T) {
	db := newTestDatabase(t)
	if err := db.EnsureAuthSessionTable(); err != nil {
		t.Fatalf("EnsureAuthSessionTable error: %v", err)
	}
	if err := db.EnsureAuthAccountsTable(); err != nil {
		t.Fatalf("EnsureAuthAccountsTable error: %v", err)
	}
	if err := db.SaveAuthSession("SESSDATA=abc; DedeUserID=9527; bili_jct=token"); err != nil {
		t.Fatalf("SaveAuthSession error: %v", err)
	}

	if err := db.SyncLegacyAuthSessionToAccount(); err != nil {
		t.Fatalf("SyncLegacyAuthSessionToAccount error: %v", err)
	}

	accounts, err := db.ListAuthAccounts()
	if err != nil {
		t.Fatalf("ListAuthAccounts error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 synced account, got %d", len(accounts))
	}
	if accounts[0].UID != "9527" {
		t.Fatalf("expected synced uid 9527, got %q", accounts[0].UID)
	}
}
