CREATE TABLE IF NOT EXISTS scrapy_items
(
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    price_filter    TEXT NOT NULL,
    price_filter_label TEXT NOT NULL,
    discount_filter TEXT NOT NULL,
    discount_filter_label TEXT NOT NULL,
    product         TEXT NOT NULL,
    product_name    TEXT NOT NULL,
    nums            INTEGER,
    increase_number INTEGER,
    next_token      TEXT,
    create_time     DATETIME,
    `order`         TEXT
);

CREATE TABLE IF NOT EXISTS version
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    version    INTEGER NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT OR
REPLACE
INTO version (id, version, updated_at)
VALUES (1, 1, CURRENT_TIMESTAMP);

CREATE TABLE IF NOT EXISTS c2c_items
(
    c2c_items_id      INTEGER PRIMARY KEY, -- 主键，确保唯一性
    type              INTEGER,
    c2c_items_name    TEXT    NOT NULL,
    detail_name       TEXT,
    detail_img        TEXT,
    sku_id            INTEGER,
    items_id          INTEGER,
    total_items_count INTEGER,
    price             INTEGER,
    show_price        TEXT,
    show_market_price TEXT,
    seller_uid        TEXT,
    seller_name       TEXT,
    payment_time      INTEGER,
    publish_time      INTEGER,
    is_my_publish     BOOLEAN,
    uface             TEXT,
    raw_status        INTEGER,
    raw_sale_status   INTEGER,
    normalized_status TEXT    NOT NULL DEFAULT '在售',
    status_checked_at DATETIME,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE IF NOT EXISTS c2c_fts USING fts5
(
    c2c_items_name,
    content=c2c_items,
    content_rowid=c2c_items_id,
    tokenize = 'simple'
);

-- Trigger for insert
CREATE TRIGGER IF NOT EXISTS c2c_items_insert
    AFTER INSERT
    ON c2c_items
BEGIN
    INSERT INTO c2c_fts(c2c_items_name, rowid)
    VALUES (NEW.c2c_items_name, NEW.c2c_items_id);
END;

-- Trigger for update
CREATE TRIGGER IF NOT EXISTS c2c_items_update
    AFTER UPDATE
    ON c2c_items
BEGIN
    -- Delete the old record from c2c_fts
    DELETE FROM c2c_fts WHERE rowid = OLD.c2c_items_id;

    -- Insert the updated record into c2c_fts
    INSERT INTO c2c_fts(c2c_items_name, rowid)
    VALUES (NEW.c2c_items_name, NEW.c2c_items_id);
END;

-- Trigger for delete
CREATE TRIGGER IF NOT EXISTS c2c_items_delete
    AFTER DELETE
    ON c2c_items
BEGIN
    DELETE FROM c2c_fts WHERE rowid = OLD.c2c_items_id;
END;
