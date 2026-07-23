package dao

import (
	"context"
	"database/sql"
	"fmt"
)

const catalogGroupsTableSQL = `
CREATE TABLE IF NOT EXISTS c2c_item_groups
(
	sku_id              INTEGER PRIMARY KEY,
	c2c_items_name      TEXT    NOT NULL DEFAULT '',
	detail_img          TEXT    NOT NULL DEFAULT '',
	item_count          INTEGER NOT NULL DEFAULT 0,
	min_price           INTEGER NOT NULL DEFAULT 0,
	reference_price_min INTEGER NOT NULL DEFAULT 0,
	reference_price_max INTEGER NOT NULL DEFAULT 0,
	has_reference_price INTEGER NOT NULL DEFAULT 0,
	first_seen_time     INTEGER NOT NULL DEFAULT 0,
	latest_publish_time INTEGER NOT NULL DEFAULT 0,
	updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_c2c_item_groups_first_seen
	ON c2c_item_groups(first_seen_time DESC, has_reference_price DESC, reference_price_min ASC, sku_id ASC);

CREATE INDEX IF NOT EXISTS idx_c2c_item_groups_reference_price_asc
	ON c2c_item_groups(has_reference_price DESC, reference_price_min ASC, first_seen_time DESC, sku_id ASC);

CREATE INDEX IF NOT EXISTS idx_c2c_item_groups_reference_price_desc
	ON c2c_item_groups(has_reference_price DESC, reference_price_max DESC, first_seen_time DESC, sku_id ASC);

CREATE INDEX IF NOT EXISTS idx_c2c_item_groups_latest_publish
	ON c2c_item_groups(latest_publish_time DESC, sku_id ASC);
`

const rebuildCatalogGroupsSQL = `
INSERT INTO c2c_item_groups (
	sku_id, c2c_items_name, detail_img, item_count, min_price,
	reference_price_min, reference_price_max, has_reference_price,
	first_seen_time, latest_publish_time, updated_at
)
SELECT
	grouped.sku_id,
	grouped.c2c_items_name,
	COALESCE((
		SELECT rep.detail_img
		FROM c2c_items rep
		WHERE rep.sku_id = grouped.sku_id
		  AND TRIM(COALESCE(rep.detail_img, '')) != ''
		ORDER BY rep.publish_time DESC, rep.updated_at DESC, rep.c2c_items_id DESC
		LIMIT 1
	), ''),
	grouped.item_count,
	grouped.min_price,
	COALESCE(grouped.reference_price_min, 0),
	COALESCE(grouped.reference_price_max, 0),
	CASE WHEN grouped.reference_price_min > 0 AND grouped.reference_price_max > 0 THEN 1 ELSE 0 END,
	grouped.first_seen_time,
	grouped.latest_publish_time,
	CURRENT_TIMESTAMP
FROM (
	SELECT
		sku_id,
		COALESCE(MAX(NULLIF(detail_name, '')), MAX(c2c_items_name)) AS c2c_items_name,
		COUNT(*) AS item_count,
		COALESCE(MIN(price), 0) AS min_price,
		MIN(CASE WHEN reference_price > 0 THEN reference_price END) AS reference_price_min,
		MAX(CASE WHEN reference_price > 0 THEN reference_price END) AS reference_price_max,
		MIN(COALESCE(CAST(strftime('%s', created_at) AS INTEGER) * 1000, 0)) AS first_seen_time,
		MAX(COALESCE(publish_time, 0)) AS latest_publish_time
	FROM c2c_items
	WHERE sku_id IS NOT NULL AND sku_id != 0
	GROUP BY sku_id
) grouped
`

const refreshCatalogGroupSQL = `
INSERT INTO c2c_item_groups (
	sku_id, c2c_items_name, detail_img, item_count, min_price,
	reference_price_min, reference_price_max, has_reference_price,
	first_seen_time, latest_publish_time, updated_at
)
SELECT
	grouped.sku_id,
	grouped.c2c_items_name,
	COALESCE((
		SELECT rep.detail_img
		FROM c2c_items rep
		WHERE rep.sku_id = grouped.sku_id
		  AND TRIM(COALESCE(rep.detail_img, '')) != ''
		ORDER BY rep.publish_time DESC, rep.updated_at DESC, rep.c2c_items_id DESC
		LIMIT 1
	), ''),
	grouped.item_count,
	grouped.min_price,
	COALESCE(grouped.reference_price_min, 0),
	COALESCE(grouped.reference_price_max, 0),
	CASE WHEN grouped.reference_price_min > 0 AND grouped.reference_price_max > 0 THEN 1 ELSE 0 END,
	grouped.first_seen_time,
	grouped.latest_publish_time,
	CURRENT_TIMESTAMP
FROM (
	SELECT
		sku_id,
		COALESCE(MAX(NULLIF(detail_name, '')), MAX(c2c_items_name)) AS c2c_items_name,
		COUNT(*) AS item_count,
		COALESCE(MIN(price), 0) AS min_price,
		MIN(CASE WHEN reference_price > 0 THEN reference_price END) AS reference_price_min,
		MAX(CASE WHEN reference_price > 0 THEN reference_price END) AS reference_price_max,
		MIN(COALESCE(CAST(strftime('%s', created_at) AS INTEGER) * 1000, 0)) AS first_seen_time,
		MAX(COALESCE(publish_time, 0)) AS latest_publish_time
	FROM c2c_items
	WHERE sku_id = ?
	GROUP BY sku_id
) grouped
WHERE grouped.sku_id IS NOT NULL
ON CONFLICT(sku_id) DO UPDATE SET
	c2c_items_name = excluded.c2c_items_name,
	detail_img = excluded.detail_img,
	item_count = excluded.item_count,
	min_price = excluded.min_price,
	reference_price_min = excluded.reference_price_min,
	reference_price_max = excluded.reference_price_max,
	has_reference_price = excluded.has_reference_price,
	first_seen_time = excluded.first_seen_time,
	latest_publish_time = excluded.latest_publish_time,
	updated_at = CURRENT_TIMESTAMP
`

var catalogProjectionColumns = []string{
	"sku_id",
	"c2c_items_name",
	"detail_name",
	"detail_img",
	"price",
	"reference_price",
	"publish_time",
	"created_at",
	"updated_at",
	"c2c_items_id",
}

// EnsureCatalogReadModel creates and transactionally backfills the catalog
// projection. Repeated calls are cheap after the first successful call.
func (d *Database) EnsureCatalogReadModel() error {
	d.catalogOnce.Do(func() {
		d.catalogErr = d.ensureCatalogReadModel(nil)
	})
	return d.catalogErr
}

// EnsureCatalogReadModelAndVersion atomically backfills and validates the read
// model before advancing the database version. It is used only by migrations.
func (d *Database) EnsureCatalogReadModelAndVersion(version int) error {
	d.catalogOnce.Do(func() {
		d.catalogErr = d.ensureCatalogReadModel(&version)
	})
	return d.catalogErr
}

func (d *Database) ensureCatalogReadModel(targetVersion *int) error {
	available, err := d.catalogProjectionSourceAvailable()
	if err != nil {
		return err
	}

	tx, err := d.Db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(context.Background(), catalogGroupsTableSQL); err != nil {
		return fmt.Errorf("create catalog read model: %w", err)
	}
	if !available {
		if targetVersion != nil {
			if err = updateDatabaseVersionTx(tx, *targetVersion); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(context.Background(), `PRAGMA optimize`); err != nil {
			return fmt.Errorf("optimize catalog projection: %w", err)
		}
		return tx.Commit()
	}

	var sourceCount int
	if err = tx.QueryRowContext(
		context.Background(),
		`SELECT COUNT(DISTINCT sku_id) FROM c2c_items WHERE sku_id IS NOT NULL AND sku_id != 0`,
	).Scan(&sourceCount); err != nil {
		return fmt.Errorf("count catalog source groups: %w", err)
	}

	var projectionCount int
	if err = tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM c2c_item_groups`).Scan(&projectionCount); err != nil {
		return fmt.Errorf("count catalog projection groups: %w", err)
	}

	if sourceCount != projectionCount {
		if _, err = tx.ExecContext(context.Background(), `DELETE FROM c2c_item_groups`); err != nil {
			return fmt.Errorf("clear catalog projection: %w", err)
		}
		if _, err = tx.ExecContext(context.Background(), rebuildCatalogGroupsSQL); err != nil {
			return fmt.Errorf("backfill catalog projection: %w", err)
		}
	}

	var validatedCount int
	if err = tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM c2c_item_groups`).Scan(&validatedCount); err != nil {
		return fmt.Errorf("validate catalog projection: %w", err)
	}
	if validatedCount != sourceCount {
		return fmt.Errorf("catalog projection validation failed: source=%d projection=%d", sourceCount, validatedCount)
	}

	if targetVersion != nil {
		if err = updateDatabaseVersionTx(tx, *targetVersion); err != nil {
			return err
		}
	}

	if _, err = tx.ExecContext(context.Background(), `PRAGMA optimize`); err != nil {
		return fmt.Errorf("optimize catalog projection: %w", err)
	}
	return tx.Commit()
}

func updateDatabaseVersionTx(tx *sql.Tx, version int) error {
	result, err := tx.ExecContext(
		context.Background(),
		`UPDATE version SET version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`,
		version,
	)
	if err != nil {
		return fmt.Errorf("update database version: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read database version update result: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("update database version: expected one row, updated %d", affected)
	}
	return nil
}

func (d *Database) catalogProjectionSourceAvailable() (bool, error) {
	for _, column := range catalogProjectionColumns {
		exists, err := tableColumnExists(d.Db, "c2c_items", column)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

func refreshCatalogGroupsTx(ctx context.Context, tx *sql.Tx, skuIDs map[int64]struct{}) error {
	for skuID := range skuIDs {
		if skuID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM c2c_item_groups WHERE sku_id = ?`, skuID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, refreshCatalogGroupSQL, skuID); err != nil {
			return err
		}
	}
	return nil
}

// RefreshCatalogGroups is useful for repairs and tests that intentionally
// mutate c2c_items outside the DAO write path.
func (d *Database) RefreshCatalogGroups(skuIDs ...int64) error {
	if err := d.EnsureCatalogReadModel(); err != nil {
		return err
	}
	affected := make(map[int64]struct{}, len(skuIDs))
	for _, skuID := range skuIDs {
		if skuID > 0 {
			affected[skuID] = struct{}{}
		}
	}
	tx, err := d.Db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = refreshCatalogGroupsTx(context.Background(), tx, affected); err != nil {
		return err
	}
	return tx.Commit()
}
