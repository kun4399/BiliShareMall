package dao

import "testing"

func TestAuthSessionLifecycle(t *testing.T) {
	db := newTestDatabase(t)

	if err := db.EnsureAuthSessionTable(); err != nil {
		t.Fatalf("EnsureAuthSessionTable error: %v", err)
	}

	session, err := db.GetAuthSession()
	if err != nil {
		t.Fatalf("GetAuthSession initial error: %v", err)
	}
	if session.Cookies != "" {
		t.Fatalf("expected empty initial cookies, got %q", session.Cookies)
	}

	cookies := "SESSDATA=abc; DedeUserID=1; bili_jct=token"
	if err := db.SaveAuthSession(cookies); err != nil {
		t.Fatalf("SaveAuthSession error: %v", err)
	}

	session, err = db.GetAuthSession()
	if err != nil {
		t.Fatalf("GetAuthSession saved error: %v", err)
	}
	if session.Cookies != cookies {
		t.Fatalf("expected cookies %q, got %q", cookies, session.Cookies)
	}
	if session.UpdatedAt.IsZero() {
		t.Fatal("expected updatedAt to be populated")
	}

	if err := db.ClearAuthSession(); err != nil {
		t.Fatalf("ClearAuthSession error: %v", err)
	}

	session, err = db.GetAuthSession()
	if err != nil {
		t.Fatalf("GetAuthSession cleared error: %v", err)
	}
	if session.Cookies != "" {
		t.Fatalf("expected cleared cookies, got %q", session.Cookies)
	}
}
