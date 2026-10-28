package store

import "testing"

func TestSeedIntegrity(t *testing.T) {
	s, _ := testStore(t)

	const freshHint = " (expects a freshly seeded DB: docker compose down -v; docker compose up -d)"
	if n := countWhere(t, s, `SELECT count(*) FROM products`); n < 12 {
		t.Fatalf("%d products, want >= 12"+freshHint, n)
	}
	if n := countWhere(t, s, `SELECT count(*) FROM products WHERE stock <= $1`, LowStockThreshold); n < 2 {
		t.Fatalf("%d low-stock products, want >= 2"+freshHint, n)
	}
	if n := countWhere(t, s, `SELECT count(*) FROM products WHERE NOT is_active`); n < 1 {
		t.Fatalf("no inactive product" + freshHint)
	}
	if n := countWhere(t, s, `SELECT count(*) FROM sales_orders`); n < 2 {
		t.Fatalf("%d orders, want >= 2"+freshHint, n)
	}

	if n := countWhere(t, s, `
		SELECT count(*) FROM (
			SELECT p.id FROM products p
			LEFT JOIN stock_movements m ON m.product_id = p.id
			GROUP BY p.id
			HAVING p.stock <> COALESCE(sum(CASE WHEN m.type = 'in' THEN m.qty ELSE -m.qty END), 0)
		) bad`); n != 0 {
		t.Fatalf("%d products where stock differs from the movement ledger", n)
	}

	if n := countWhere(t, s, `
		SELECT count(*) FROM (
			SELECT o.id FROM sales_orders o
			LEFT JOIN sales_order_items i ON i.order_id = o.id
			GROUP BY o.id
			HAVING o.total <> COALESCE(sum(i.qty::bigint * i.unit_price), 0)
		) bad`); n != 0 {
		t.Fatalf("%d orders whose total differs from their items", n)
	}
}
