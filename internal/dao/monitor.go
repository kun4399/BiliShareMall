package dao

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type MonitorRule struct {
	ID       int64  `json:"id"`
	SkuID    int64  `json:"skuId"`
	MinPrice int    `json:"minPrice"`
	MaxPrice int    `json:"maxPrice"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark"`
}

type MonitorConfig struct {
	Webhook string        `json:"webhook"`
	Rules   []MonitorRule `json:"rules"`
}

func (d *Database) GetMonitorConfig() (MonitorConfig, error) {
	config := MonitorConfig{
		Webhook: "",
		Rules:   []MonitorRule{},
	}

	err := d.Db.QueryRowContext(context.Background(), `SELECT webhook FROM monitor_config WHERE id = 1`).Scan(&config.Webhook)
	if err != nil && err != sql.ErrNoRows {
		return config, err
	}

	rows, err := d.Db.QueryContext(
		context.Background(),
		`SELECT id, sku_id, min_price, max_price, enabled, remark
		FROM monitor_rules
		ORDER BY id ASC`,
	)
	if err != nil {
		return config, err
	}
	defer rows.Close()

	for rows.Next() {
		var rule MonitorRule
		var enabled int
		if err := rows.Scan(&rule.ID, &rule.SkuID, &rule.MinPrice, &rule.MaxPrice, &enabled, &rule.Remark); err != nil {
			return config, err
		}
		rule.Enabled = enabled == 1
		config.Rules = append(config.Rules, rule)
	}
	if err := rows.Err(); err != nil {
		return config, err
	}
	return config, nil
}

func (d *Database) SaveMonitorConfig(config MonitorConfig) error {
	tx, err := d.Db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(
		context.Background(),
		`INSERT INTO monitor_config(id, webhook) VALUES(1, ?)
		ON CONFLICT(id) DO UPDATE SET webhook = excluded.webhook`,
		strings.TrimSpace(config.Webhook),
	); err != nil {
		return err
	}

	keptIDs := make([]int64, 0, len(config.Rules))
	for _, rule := range config.Rules {
		if rule.SkuID <= 0 {
			return fmt.Errorf("invalid skuId: %d", rule.SkuID)
		}
		if rule.MinPrice < 0 || rule.MaxPrice < 0 {
			return fmt.Errorf("invalid rule price range")
		}
		if rule.MinPrice > rule.MaxPrice {
			return fmt.Errorf("minPrice cannot be greater than maxPrice")
		}
		enabled := 0
		if rule.Enabled {
			enabled = 1
		}
		remark := strings.TrimSpace(rule.Remark)

		if rule.ID > 0 {
			result, execErr := tx.ExecContext(
				context.Background(),
				`UPDATE monitor_rules
				SET sku_id = ?, min_price = ?, max_price = ?, enabled = ?, remark = ?, updated_at = CURRENT_TIMESTAMP
				WHERE id = ?`,
				rule.SkuID,
				rule.MinPrice,
				rule.MaxPrice,
				enabled,
				remark,
				rule.ID,
			)
			if execErr != nil {
				return execErr
			}
			affected, rowsErr := result.RowsAffected()
			if rowsErr != nil {
				return rowsErr
			}
			if affected > 0 {
				keptIDs = append(keptIDs, rule.ID)
				continue
			}
		}

		result, execErr := tx.ExecContext(
			context.Background(),
			`INSERT INTO monitor_rules(sku_id, min_price, max_price, enabled, remark) VALUES(?, ?, ?, ?, ?)`,
			rule.SkuID,
			rule.MinPrice,
			rule.MaxPrice,
			enabled,
			remark,
		)
		if execErr != nil {
			return execErr
		}
		id, idErr := result.LastInsertId()
		if idErr != nil {
			return idErr
		}
		keptIDs = append(keptIDs, id)
	}

	if len(keptIDs) == 0 {
		if _, err = tx.ExecContext(context.Background(), `DELETE FROM monitor_rules`); err != nil {
			return err
		}
		if _, err = tx.ExecContext(context.Background(), `DELETE FROM monitor_alert_history`); err != nil {
			return err
		}
	} else {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(keptIDs)), ",")
		args := make([]any, 0, len(keptIDs))
		for _, id := range keptIDs {
			args = append(args, id)
		}
		deleteSQL := fmt.Sprintf(`DELETE FROM monitor_rules WHERE id NOT IN (%s)`, placeholders)
		if _, err = tx.ExecContext(context.Background(), deleteSQL, args...); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(context.Background(), `DELETE FROM monitor_alert_history WHERE rule_id NOT IN (SELECT id FROM monitor_rules)`); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (d *Database) ReadEnabledMonitorRules() ([]MonitorRule, error) {
	rows, err := d.Db.QueryContext(
		context.Background(),
		`SELECT id, sku_id, min_price, max_price, enabled, remark
		FROM monitor_rules
		WHERE enabled = 1
		ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]MonitorRule, 0)
	for rows.Next() {
		var rule MonitorRule
		var enabled int
		if err := rows.Scan(&rule.ID, &rule.SkuID, &rule.MinPrice, &rule.MaxPrice, &enabled, &rule.Remark); err != nil {
			return nil, err
		}
		rule.Enabled = enabled == 1
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (d *Database) ReadMonitorWebhook() (string, error) {
	var webhook string
	err := d.Db.QueryRowContext(context.Background(), `SELECT webhook FROM monitor_config WHERE id = 1`).Scan(&webhook)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return webhook, err
}

func (d *Database) EnsureMonitorRuleRemarkColumn() error {
	rows, err := d.Db.QueryContext(context.Background(), `PRAGMA table_info(monitor_rules)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasRemark := false
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "remark" {
			hasRemark = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasRemark {
		return nil
	}

	_, err = d.Db.ExecContext(
		context.Background(),
		`ALTER TABLE monitor_rules ADD COLUMN remark TEXT NOT NULL DEFAULT ''`,
	)
	return err
}

func (d *Database) ReserveMonitorAlert(ruleID, c2cItemsID int64, taskID int) (bool, error) {
	result, err := d.Db.ExecContext(
		context.Background(),
		`INSERT OR IGNORE INTO monitor_alert_history(rule_id, c2c_items_id, task_id, sent, sent_at)
		VALUES(?, ?, ?, 0, NULL)`,
		ruleID,
		c2cItemsID,
		taskID,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (d *Database) MarkMonitorAlertSent(ruleID, c2cItemsID int64, sentAt time.Time) error {
	_, err := d.Db.ExecContext(
		context.Background(),
		`UPDATE monitor_alert_history
		SET sent = 1, sent_at = ?
		WHERE rule_id = ? AND c2c_items_id = ?`,
		sentAt,
		ruleID,
		c2cItemsID,
	)
	return err
}

func (d *Database) ReleaseMonitorAlertReservation(ruleID, c2cItemsID int64) error {
	_, err := d.Db.ExecContext(
		context.Background(),
		`DELETE FROM monitor_alert_history WHERE rule_id = ? AND c2c_items_id = ? AND sent = 0`,
		ruleID,
		c2cItemsID,
	)
	return err
}
