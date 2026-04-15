package dao

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AuthAccount struct {
	ID          int64     `json:"id"`
	UID         string    `json:"uid"`
	AccountName string    `json:"accountName"`
	Cookies     string    `json:"cookies"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (d *Database) EnsureAuthAccountsTable() error {
	_, err := d.Db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS auth_accounts
		(
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			uid          TEXT NOT NULL UNIQUE,
			account_name TEXT NOT NULL DEFAULT '',
			cookies      TEXT NOT NULL DEFAULT '',
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	)
	return err
}

func (d *Database) UpsertAuthAccount(uid, accountName, cookies string) (int64, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return 0, fmt.Errorf("uid is required")
	}
	cookies = strings.TrimSpace(cookies)
	accountName = strings.TrimSpace(accountName)

	if _, err := d.Db.ExecContext(
		context.Background(),
		`INSERT INTO auth_accounts(uid, account_name, cookies, created_at, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(uid) DO UPDATE SET
			account_name = excluded.account_name,
			cookies = excluded.cookies,
			updated_at = CURRENT_TIMESTAMP`,
		uid,
		accountName,
		cookies,
	); err != nil {
		return 0, err
	}

	var id int64
	if err := d.Db.QueryRowContext(
		context.Background(),
		`SELECT id FROM auth_accounts WHERE uid = ?`,
		uid,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (d *Database) ListAuthAccounts() ([]AuthAccount, error) {
	rows, err := d.Db.QueryContext(
		context.Background(),
		`SELECT id, uid, account_name, cookies, COALESCE(updated_at, CURRENT_TIMESTAMP)
		FROM auth_accounts
		ORDER BY updated_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ret := make([]AuthAccount, 0)
	for rows.Next() {
		var item AuthAccount
		var updatedAt string
		if err := rows.Scan(
			&item.ID,
			&item.UID,
			&item.AccountName,
			&item.Cookies,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if updatedAt != "" {
			parsed, parseErr := time.ParseInLocation("2006-01-02 15:04:05", updatedAt, time.Local)
			if parseErr == nil {
				item.UpdatedAt = parsed
			}
		}
		ret = append(ret, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (d *Database) GetAuthAccountByID(id int64) (AuthAccount, error) {
	var item AuthAccount
	var updatedAt string
	err := d.Db.QueryRowContext(
		context.Background(),
		`SELECT id, uid, account_name, cookies, COALESCE(updated_at, CURRENT_TIMESTAMP)
		FROM auth_accounts
		WHERE id = ?`,
		id,
	).Scan(
		&item.ID,
		&item.UID,
		&item.AccountName,
		&item.Cookies,
		&updatedAt,
	)
	if err != nil {
		return AuthAccount{}, err
	}
	if updatedAt != "" {
		parsed, parseErr := time.ParseInLocation("2006-01-02 15:04:05", updatedAt, time.Local)
		if parseErr == nil {
			item.UpdatedAt = parsed
		}
	}
	return item, nil
}

func (d *Database) DeleteAuthAccount(id int64) error {
	_, err := d.Db.ExecContext(context.Background(), `DELETE FROM auth_accounts WHERE id = ?`, id)
	return err
}

func (d *Database) ClearAuthAccounts() error {
	_, err := d.Db.ExecContext(context.Background(), `DELETE FROM auth_accounts`)
	return err
}

func (d *Database) SyncLegacyAuthSessionToAccount() error {
	session, err := d.GetAuthSession()
	if err != nil {
		return err
	}
	cookies := strings.TrimSpace(session.Cookies)
	if cookies == "" {
		return nil
	}

	uid := cookieValue(cookies, "DedeUserID")
	if uid == "" {
		return nil
	}
	_, err = d.UpsertAuthAccount(uid, uid, cookies)
	return err
}

func cookieValue(cookieHeader, key string) string {
	for _, segment := range strings.Split(cookieHeader, ";") {
		pair := strings.SplitN(strings.TrimSpace(segment), "=", 2)
		if len(pair) != 2 {
			continue
		}
		if strings.TrimSpace(pair[0]) == key {
			return strings.TrimSpace(pair[1])
		}
	}
	return ""
}
