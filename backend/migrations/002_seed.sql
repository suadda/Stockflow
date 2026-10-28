-- 002_seed.sql: StockFlow sample data.
-- One transaction. Products start at stock 0; every unit of stock comes from
-- the ledger (stock_movements), and products.stock is derived from it at the end.
-- Ids are never hardcoded: rows are looked up by SKU or via RETURNING.
-- The closing DO block fails init loudly if the seed is inconsistent.

BEGIN;

-- 1. Products (14), stock 0. Prices are integer IDR.
INSERT INTO products (sku, name, price, is_active) VALUES
    ('USB-C-1M',     'Kabel USB-C ke USB-C 1m',           45000,   TRUE),
    ('USB-C-2M',     'Kabel USB-C ke USB-C 2m',           65000,   TRUE),
    ('HDMI-2M',      'Kabel HDMI 2.0 2m',                 85000,   TRUE),
    ('MOUSE-WL',     'Mouse Wireless 2.4GHz',             125000,  TRUE),
    ('KB-MECH-87',   'Keyboard Mekanikal 87 Tombol',      650000,  TRUE),
    ('SSD-512-NVME', 'SSD NVMe 512GB',                    899000,  TRUE),
    ('FD-64GB',      'Flashdisk USB 3.0 64GB',            95000,   TRUE),
    ('PB-10000',     'Power Bank 10000mAh',               275000,  TRUE),
    ('MON-24-IPS',   'Monitor 24 inci IPS Full HD',       1850000, TRUE),
    ('HVS-A4-70',    'Kertas HVS A4 70gsm (1 rim)',       52000,   TRUE),
    ('PEN-GEL-BK',   'Pulpen Gel Hitam (1 lusin)',        36000,   TRUE),
    ('STPL-NO10',    'Stapler No. 10',                    25000,   TRUE),
    ('TONER-85A',    'Toner Printer Laser 85A',           1250000, TRUE),
    ('ADPT-VGA',     'Adapter HDMI ke VGA',               75000,   FALSE); -- discontinued

-- 2. Initial receipts: one 'in' movement per product, spread into the past
--    so "last movement" shows varied relative times.
INSERT INTO stock_movements (product_id, type, qty, note, created_at)
SELECT p.id, 'in', v.qty, 'initial receipt', now() - v.age
FROM (VALUES
    ('USB-C-1M',     120, interval '30 days'),
    ('USB-C-2M',      60, interval '27 days 4 hours'),
    ('HDMI-2M',       40, interval '25 days'),
    ('MOUSE-WL',      35, interval '21 days 6 hours'),
    ('KB-MECH-87',     8, interval '18 days'),
    ('SSD-512-NVME',   6, interval '15 days 2 hours'),
    ('FD-64GB',       80, interval '12 days'),
    ('PB-10000',      25, interval '10 days 5 hours'),
    ('MON-24-IPS',     4, interval '8 days'),
    ('HVS-A4-70',    200, interval '6 days 3 hours'),
    ('PEN-GEL-BK',   150, interval '5 days'),
    ('STPL-NO10',     45, interval '4 days 8 hours'),
    ('TONER-85A',     10, interval '3 days'),
    ('ADPT-VGA',       3, interval '45 days')
) AS v(sku, qty, age)
JOIN products p ON p.sku = v.sku;

-- 3. Orders. Inserted with total 0; unit_price copied from products.price.
WITH o AS (
    INSERT INTO sales_orders (customer, total, created_at)
    VALUES ('Toko Maju', 0, now() - interval '2 days')
    RETURNING id
)
INSERT INTO sales_order_items (order_id, sku, qty, unit_price)
SELECT o.id, v.sku, v.qty, (SELECT p.price FROM products p WHERE p.sku = v.sku)
FROM o CROSS JOIN (VALUES
    ('USB-C-1M',    5),
    ('HVS-A4-70',  10),
    ('PEN-GEL-BK',  4)
) AS v(sku, qty);

WITH o AS (
    INSERT INTO sales_orders (customer, total, created_at)
    VALUES ('CV Sinar Jaya', 0, now() - interval '5 hours')
    RETURNING id
)
INSERT INTO sales_order_items (order_id, sku, qty, unit_price)
SELECT o.id, v.sku, v.qty, (SELECT p.price FROM products p WHERE p.sku = v.sku)
FROM o CROSS JOIN (VALUES
    ('MON-24-IPS', 1),
    ('MOUSE-WL',   2),
    ('USB-C-1M',   3)
) AS v(sku, qty);

-- Totals = Σ qty × unit_price of the inserted items.
UPDATE sales_orders o
SET total = (SELECT sum(i.qty * i.unit_price) FROM sales_order_items i WHERE i.order_id = o.id);

-- 4. One 'out' movement per order item, timestamped with its order.
INSERT INTO stock_movements (product_id, type, qty, note, created_at)
SELECT p.id, 'out', i.qty, 'order #' || o.id, o.created_at
FROM sales_order_items i
JOIN sales_orders o ON o.id = i.order_id
JOIN products p ON p.sku = i.sku;

-- 5. Stock = Σin − Σout.
UPDATE products p
SET stock = COALESCE((
    SELECT sum(CASE WHEN m.type = 'in' THEN m.qty ELSE -m.qty END)
    FROM stock_movements m WHERE m.product_id = p.id
), 0);

-- 6. Integrity check: abort init on any inconsistency.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM products p
        LEFT JOIN stock_movements m ON m.product_id = p.id
        GROUP BY p.id
        HAVING p.stock <> COALESCE(sum(CASE WHEN m.type = 'in' THEN m.qty ELSE -m.qty END), 0)
    ) THEN
        RAISE EXCEPTION 'seed: product stock differs from ledger (sum in - sum out)';
    END IF;

    IF EXISTS (
        SELECT 1 FROM sales_orders o
        LEFT JOIN sales_order_items i ON i.order_id = o.id
        GROUP BY o.id
        HAVING o.total <> COALESCE(sum(i.qty * i.unit_price), 0)
    ) THEN
        RAISE EXCEPTION 'seed: order total differs from sum(qty * unit_price)';
    END IF;

    IF EXISTS (
        SELECT 1 FROM sales_order_items i
        JOIN products p ON p.sku = i.sku
        WHERE i.unit_price <> p.price
    ) THEN
        RAISE EXCEPTION 'seed: order item unit_price differs from product price';
    END IF;

    IF (SELECT count(*) FROM products) < 12 THEN
        RAISE EXCEPTION 'seed: fewer than 12 products';
    END IF;

    IF (SELECT count(*) FROM products WHERE stock <= 10) < 3 THEN
        RAISE EXCEPTION 'seed: fewer than 3 low-stock products (stock <= 10)';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM products WHERE NOT is_active) THEN
        RAISE EXCEPTION 'seed: no inactive product';
    END IF;

    IF (SELECT count(*) FROM sales_orders) < 2 THEN
        RAISE EXCEPTION 'seed: fewer than 2 orders';
    END IF;
END
$$;

COMMIT;
