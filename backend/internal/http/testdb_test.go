package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"stockflow/internal/store"
)

func testAPI(t *testing.T) (http.Handler, *pgxpool.Pool, string) {
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
	prefix := "T" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-"
	t.Cleanup(func() {
		cleanupPrefix(t, pool, prefix)
		pool.Close()
	})
	return NewRouter(store.New(pool)), pool, prefix
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeAs[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func expect(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body.String())
	}
}

func expectError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	expect(t, rec, status)
	e := envelope(t, rec)
	if e["code"] != code {
		t.Fatalf("code %v, want %s: %s", e["code"], code, rec.Body.String())
	}
	return e
}

func detailFields(e map[string]any) map[string]bool {
	out := map[string]bool{}
	d, _ := e["details"].([]any)
	for _, x := range d {
		if m, ok := x.(map[string]any); ok {
			out[fmt.Sprint(m["field"])] = true
		}
	}
	return out
}

func cleanupPrefix(t *testing.T, pool *pgxpool.Pool, prefix string) {
	ctx := context.Background()
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
}
