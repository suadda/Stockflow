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

// testStore returns a Store on the real database from DATABASE_URL and a
// unique SKU prefix whose rows are removed on cleanup. Without DATABASE_URL
// the test is skipped.
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

// newProduct creates an active product with stock 0.
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

// movementCount returns how many movements a product has.
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
