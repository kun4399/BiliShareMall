package dao

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestWALReadersDoNotQueueBehindWriterConnection(t *testing.T) {
	database, err := NewDatabase(filepath.Join(t.TempDir(), "concurrency.db"))
	if err != nil {
		t.Fatalf("NewDatabase error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if got := database.Db.Stats().MaxOpenConnections; got != sqliteMaxOpenConnections {
		t.Fatalf("expected %d database connections, got %d", sqliteMaxOpenConnections, got)
	}
	if _, err := database.Db.Exec(`CREATE TABLE records(id INTEGER PRIMARY KEY, value TEXT); INSERT INTO records VALUES(1, 'old')`); err != nil {
		t.Fatalf("prepare database error: %v", err)
	}

	ctx := context.Background()
	writer, err := database.Db.Conn(ctx)
	if err != nil {
		t.Fatalf("get writer connection error: %v", err)
	}
	defer writer.Close()
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin writer transaction error: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE records SET value = 'new' WHERE id = 1`); err != nil {
		t.Fatalf("update in writer transaction error: %v", err)
	}

	readResult := make(chan error, 1)
	go func() {
		var value string
		err := database.Db.QueryRowContext(ctx, `SELECT value FROM records WHERE id = 1`).Scan(&value)
		if err == nil && value != "old" {
			t.Errorf("expected WAL reader snapshot %q, got %q", "old", value)
		}
		readResult <- err
	}()

	select {
	case err := <-readResult:
		if err != nil {
			t.Fatalf("concurrent read error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reader queued behind writer instead of using a separate WAL connection")
	}
}
