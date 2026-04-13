package dao

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadMonitorConfig(t *testing.T) {
	db := newMonitorTestDatabase(t)

	err := db.SaveMonitorConfig(MonitorConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Rules: []MonitorRule{
			{
				SkuID:    1001,
				MinPrice: 1000,
				MaxPrice: 2000,
				Enabled:  true,
			},
			{
				SkuID:    1002,
				MinPrice: 3000,
				MaxPrice: 5000,
				Enabled:  false,
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveMonitorConfig error: %v", err)
	}

	config, err := db.GetMonitorConfig()
	if err != nil {
		t.Fatalf("GetMonitorConfig error: %v", err)
	}
	if config.Webhook == "" {
		t.Fatal("expected webhook to be saved")
	}
	if len(config.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(config.Rules))
	}
	if config.Rules[0].ID <= 0 {
		t.Fatalf("expected persisted rule id, got %d", config.Rules[0].ID)
	}

	rules, err := db.ReadEnabledMonitorRules()
	if err != nil {
		t.Fatalf("ReadEnabledMonitorRules error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected only one enabled rule, got %d", len(rules))
	}
	if rules[0].SkuID != 1001 {
		t.Fatalf("expected sku 1001, got %d", rules[0].SkuID)
	}
}

func TestReserveMonitorAlertDeduplicates(t *testing.T) {
	db := newMonitorTestDatabase(t)
	if err := db.SaveMonitorConfig(MonitorConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Rules: []MonitorRule{
			{
				SkuID:    1001,
				MinPrice: 1000,
				MaxPrice: 2000,
				Enabled:  true,
			},
		},
	}); err != nil {
		t.Fatalf("SaveMonitorConfig error: %v", err)
	}

	config, err := db.GetMonitorConfig()
	if err != nil {
		t.Fatalf("GetMonitorConfig error: %v", err)
	}
	ruleID := config.Rules[0].ID

	ok, err := db.ReserveMonitorAlert(ruleID, 12345, 1)
	if err != nil {
		t.Fatalf("ReserveMonitorAlert first error: %v", err)
	}
	if !ok {
		t.Fatal("expected first reservation to succeed")
	}
	ok, err = db.ReserveMonitorAlert(ruleID, 12345, 1)
	if err != nil {
		t.Fatalf("ReserveMonitorAlert second error: %v", err)
	}
	if ok {
		t.Fatal("expected duplicated reservation to be rejected")
	}
}

func TestSaveMonitorConfigDeletesRemovedRules(t *testing.T) {
	db := newMonitorTestDatabase(t)

	if err := db.SaveMonitorConfig(MonitorConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Rules: []MonitorRule{
			{SkuID: 1, MinPrice: 100, MaxPrice: 200, Enabled: true},
			{SkuID: 2, MinPrice: 300, MaxPrice: 400, Enabled: true},
		},
	}); err != nil {
		t.Fatalf("initial SaveMonitorConfig error: %v", err)
	}

	loaded, err := db.GetMonitorConfig()
	if err != nil {
		t.Fatalf("GetMonitorConfig error: %v", err)
	}
	if len(loaded.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(loaded.Rules))
	}

	keep := loaded.Rules[0]
	keep.MaxPrice = 250
	if err := db.SaveMonitorConfig(MonitorConfig{
		Webhook: loaded.Webhook,
		Rules:   []MonitorRule{keep},
	}); err != nil {
		t.Fatalf("second SaveMonitorConfig error: %v", err)
	}

	loaded, err = db.GetMonitorConfig()
	if err != nil {
		t.Fatalf("GetMonitorConfig second error: %v", err)
	}
	if len(loaded.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(loaded.Rules))
	}
	if loaded.Rules[0].ID != keep.ID {
		t.Fatalf("expected kept rule id %d, got %d", keep.ID, loaded.Rules[0].ID)
	}
	if loaded.Rules[0].MaxPrice != 250 {
		t.Fatalf("expected updated max price 250, got %d", loaded.Rules[0].MaxPrice)
	}
}

func newMonitorTestDatabase(t *testing.T) *Database {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "monitor.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite error: %v", err)
	}
	db := &Database{Db: rawDB}
	if err := db.Init(testMonitorSchemaSQL); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

const testMonitorSchemaSQL = `
CREATE TABLE monitor_config
(
    id      INTEGER PRIMARY KEY CHECK (id = 1),
    webhook TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO monitor_config (id, webhook)
VALUES (1, '');

CREATE TABLE monitor_rules
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    sku_id     INTEGER NOT NULL,
    min_price  INTEGER NOT NULL,
    max_price  INTEGER NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE monitor_alert_history
(
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id      INTEGER NOT NULL,
    c2c_items_id INTEGER NOT NULL,
    task_id      INTEGER,
    sent         INTEGER NOT NULL DEFAULT 0,
    sent_at      DATETIME,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(rule_id, c2c_items_id)
);`
