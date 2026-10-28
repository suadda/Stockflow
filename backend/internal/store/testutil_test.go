package store

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DB test skipped: run `docker compose up -d`, then set " +
			"DATABASE_URL=postgres://stockflow:stockflow@localhost:5432/stockflow?sslmode=disable")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping: %v", err)
	}
	prefix := "S" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-"
	t.Cleanup(func() {
		like := prefix + "%"
		for _, q := range []string{
			`WITH d AS (DELETE FROM sales_order_items WHERE order_id IN
			   (SELECT order_id FROM sales_order_items WHERE sku LIKE $1) RETURNING order_id)
			 DELETE FROM sales_orders WHERE id IN (SELECT order_id FROM d)`,
			`DELETE FROM stock_movements WHERE product_id IN (SELECT id FROM products WHERE sku LIKE $1)`,
			`DELETE FROM products WHERE sku LIKE $1`,
		} {
			if _, err := pool.Exec(ctx, q, like); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
		pool.Close()
	})
	return New(pool), prefix
}

func newProduct(t *testing.T, s *Store, sku string, price int64) Product {
	t.Helper()
	p, err := s.CreateProduct(context.Background(), ProductInput{
		SKU: sku, Name: "Test " + sku, Price: json.Number(strconv.FormatInt(price, 10)),
	})
	if err != nil {
		t.Fatalf("create %s: %v", sku, err)
	}
	return p
}

func movementCount(t *testing.T, s *Store, id int64) int {
	t.Helper()
	ms, err := s.ListMovements(context.Background(), id)
	if err != nil {
		t.Fatalf("movements: %v", err)
	}
	return len(ms)
}

func stockIn(qty int) StockInput {
	return StockInput{Type: MovementIn, Qty: json.Number(strconv.Itoa(qty)), Note: "test in"}
}

func stockOut(qty int) StockInput {
	return StockInput{Type: MovementOut, Qty: json.Number(strconv.Itoa(qty)), Note: "test out"}
}

func stocked(t *testing.T, s *Store, sku string, price int64, qty int) Product {
	t.Helper()
	p := newProduct(t, s, sku, price)
	if qty > 0 {
		var err error
		if p, err = s.AdjustStock(context.Background(), p.ID, stockIn(qty)); err != nil {
			t.Fatalf("stock %s: %v", sku, err)
		}
	}
	return p
}

func order(customer string, items ...OrderItemInput) OrderInput {
	return OrderInput{Customer: customer, Items: items}
}

func item(sku string, qty int) OrderItemInput {
	return OrderItemInput{SKU: sku, Qty: json.Number(strconv.Itoa(qty))}
}

func stockOf(t *testing.T, s *Store, id int64) int64 {
	t.Helper()
	p, err := s.GetProduct(context.Background(), id)
	if err != nil {
		t.Fatalf("get %d: %v", id, err)
	}
	return p.Stock
}

func countWhere(t *testing.T, s *Store, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func movementsOf(t *testing.T, s *Store, id int64, typ string) int {
	return countWhere(t, s, `SELECT count(*) FROM stock_movements WHERE product_id = $1 AND type = $2`, id, typ)
}

func ordersFor(t *testing.T, s *Store, prefix string) int {
	return countWhere(t, s,
		`SELECT count(DISTINCT order_id) FROM sales_order_items WHERE sku LIKE $1`, prefix+"%")
}
