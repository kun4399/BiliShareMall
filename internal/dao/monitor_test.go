package dao

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
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
				Remark:   "自用规则",
			},
			{
				SkuID:    1002,
				MinPrice: 3000,
				MaxPrice: 5000,
				Enabled:  false,
				Remark:   "备用规则",
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
	if config.Rules[0].Remark != "自用规则" {
		t.Fatalf("expected remark to be persisted, got %q", config.Rules[0].Remark)
	}
	if config.Rules[0].SkuName != "测试商品一号" {
		t.Fatalf("expected sku name to be backfilled, got %q", config.Rules[0].SkuName)
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
				Remark:   "dup-test",
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
			{SkuID: 1, MinPrice: 100, MaxPrice: 200, Enabled: true, Remark: "keep-me"},
			{SkuID: 2, MinPrice: 300, MaxPrice: 400, Enabled: true, Remark: "remove-me"},
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
	keep.Remark = "updated-remark"
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
	if loaded.Rules[0].Remark != "updated-remark" {
		t.Fatalf("expected updated remark, got %q", loaded.Rules[0].Remark)
	}
}

func TestSaveMonitorConfigResetsHistoryForChangedRules(t *testing.T) {
	db := newMonitorTestDatabase(t)

	if err := db.SaveMonitorConfig(MonitorConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Rules: []MonitorRule{
			{SkuID: 101, MinPrice: 100, MaxPrice: 200, Enabled: true, Remark: "r1"},
			{SkuID: 202, MinPrice: 300, MaxPrice: 400, Enabled: true, Remark: "r2"},
		},
	}); err != nil {
		t.Fatalf("SaveMonitorConfig init error: %v", err)
	}

	config, err := db.GetMonitorConfig()
	if err != nil {
		t.Fatalf("GetMonitorConfig error: %v", err)
	}
	r1 := config.Rules[0]
	r2 := config.Rules[1]

	if _, err := db.Db.Exec(
		`INSERT INTO monitor_alert_history(rule_id, c2c_items_id, task_id, sent) VALUES (?, ?, ?, 1), (?, ?, ?, 1)`,
		r1.ID, 5001, 1,
		r2.ID, 6001, 1,
	); err != nil {
		t.Fatalf("insert monitor_alert_history error: %v", err)
	}
	if err := db.CreateMonitorAlertEvent(MonitorAlertEvent{
		RuleID:     r1.ID,
		C2CItemsID: 5001,
		TaskID:     1,
		SkuID:      r1.SkuID,
		ItemName:   "r1-item",
		Price:      120,
		ShowPrice:  "1.20",
		ItemLink:   "https://example.com/5001",
		Status:     "sent",
	}); err != nil {
		t.Fatalf("CreateMonitorAlertEvent r1 error: %v", err)
	}
	if err := db.CreateMonitorAlertEvent(MonitorAlertEvent{
		RuleID:     r2.ID,
		C2CItemsID: 6001,
		TaskID:     1,
		SkuID:      r2.SkuID,
		ItemName:   "r2-item",
		Price:      320,
		ShowPrice:  "3.20",
		ItemLink:   "https://example.com/6001",
		Status:     "sent",
	}); err != nil {
		t.Fatalf("CreateMonitorAlertEvent r2 error: %v", err)
	}

	r1.MinPrice = 150
	if err := db.SaveMonitorConfig(MonitorConfig{
		Webhook: config.Webhook,
		Rules:   []MonitorRule{r1, r2},
	}); err != nil {
		t.Fatalf("SaveMonitorConfig update error: %v", err)
	}

	var r1HistoryCount int
	if err := db.Db.QueryRow(`SELECT COUNT(*) FROM monitor_alert_history WHERE rule_id = ?`, r1.ID).Scan(&r1HistoryCount); err != nil {
		t.Fatalf("count r1 history error: %v", err)
	}
	if r1HistoryCount != 0 {
		t.Fatalf("expected r1 history to be reset, got %d", r1HistoryCount)
	}
	var r2HistoryCount int
	if err := db.Db.QueryRow(`SELECT COUNT(*) FROM monitor_alert_history WHERE rule_id = ?`, r2.ID).Scan(&r2HistoryCount); err != nil {
		t.Fatalf("count r2 history error: %v", err)
	}
	if r2HistoryCount != 1 {
		t.Fatalf("expected r2 history to stay intact, got %d", r2HistoryCount)
	}

	r1Events, err := db.ReadMonitorAlertEventsByRule(r1.ID, 10)
	if err != nil {
		t.Fatalf("ReadMonitorAlertEventsByRule r1 error: %v", err)
	}
	if len(r1Events) != 0 {
		t.Fatalf("expected r1 events to be reset, got %d", len(r1Events))
	}
	r2Events, err := db.ReadMonitorAlertEventsByRule(r2.ID, 10)
	if err != nil {
		t.Fatalf("ReadMonitorAlertEventsByRule r2 error: %v", err)
	}
	if len(r2Events) != 1 {
		t.Fatalf("expected r2 events to remain, got %d", len(r2Events))
	}
}

func TestReadMonitorAlertEventsByRuleOrdersByNewestAndLimit(t *testing.T) {
	db := newMonitorTestDatabase(t)
	if err := db.SaveMonitorConfig(MonitorConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Rules: []MonitorRule{
			{SkuID: 101, MinPrice: 100, MaxPrice: 200, Enabled: true, Remark: "r1"},
		},
	}); err != nil {
		t.Fatalf("SaveMonitorConfig error: %v", err)
	}
	config, err := db.GetMonitorConfig()
	if err != nil {
		t.Fatalf("GetMonitorConfig error: %v", err)
	}
	ruleID := config.Rules[0].ID

	if err := db.CreateMonitorAlertEvent(MonitorAlertEvent{
		RuleID:     ruleID,
		C2CItemsID: 7001,
		TaskID:     1,
		SkuID:      101,
		ItemName:   "old",
		Price:      100,
		ShowPrice:  "1.00",
		ItemLink:   "https://example.com/7001",
		Status:     "sent",
	}); err != nil {
		t.Fatalf("CreateMonitorAlertEvent old error: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if err := db.CreateMonitorAlertEvent(MonitorAlertEvent{
		RuleID:       ruleID,
		C2CItemsID:   7002,
		TaskID:       1,
		SkuID:        101,
		ItemName:     "new",
		Price:        200,
		ShowPrice:    "2.00",
		ItemLink:     "https://example.com/7002",
		Status:       "failed",
		ErrorMessage: "send failed",
	}); err != nil {
		t.Fatalf("CreateMonitorAlertEvent new error: %v", err)
	}

	events, err := db.ReadMonitorAlertEventsByRule(ruleID, 1)
	if err != nil {
		t.Fatalf("ReadMonitorAlertEventsByRule error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event with limit 1, got %d", len(events))
	}
	if events[0].C2CItemsID != 7002 {
		t.Fatalf("expected newest event first, got %d", events[0].C2CItemsID)
	}
	if events[0].Status != "failed" {
		t.Fatalf("expected failed status, got %s", events[0].Status)
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
CREATE TABLE c2c_items
(
    c2c_items_id      INTEGER PRIMARY KEY,
    c2c_items_name    TEXT    NOT NULL,
    detail_name       TEXT,
    detail_img        TEXT,
    sku_id            INTEGER,
    publish_time      INTEGER,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO c2c_items(c2c_items_id, c2c_items_name, detail_name, detail_img, sku_id, publish_time)
VALUES
    (1, '测试商品一号', '测试商品一号', '//img-1.png', 1001, 1710000000000),
    (2, '测试商品二号', '测试商品二号', '//img-2.png', 1002, 1710003600000);

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
    remark     TEXT NOT NULL DEFAULT '',
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
);

CREATE TABLE monitor_alert_events
(
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id       INTEGER NOT NULL,
    c2c_items_id  INTEGER NOT NULL,
    task_id       INTEGER,
    sku_id        INTEGER NOT NULL DEFAULT 0,
    item_name     TEXT    NOT NULL DEFAULT '',
    price         INTEGER NOT NULL DEFAULT 0,
    show_price    TEXT    NOT NULL DEFAULT '',
    item_link     TEXT    NOT NULL DEFAULT '',
    status        TEXT    NOT NULL,
    error_message TEXT    NOT NULL DEFAULT '',
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);`
