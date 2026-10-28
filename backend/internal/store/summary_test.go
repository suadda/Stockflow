package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestSummaryMatchesRecount compares Summary with a Go recount over the same
// REPEATABLE READ snapshot, so tests running in parallel cannot skew it.
func TestSummaryMatchesRecount(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()

	// A product whose price*stock overflows int32, to prove bigint summing.
	big := stocked(t, s, pfx+"BIG", 2147483647, 1000000)
	if big.Stock != 1000000 {
		t.Fatalf("stock %d", big.Stock)
	}
	stocked(t, s, pfx+"LOW", 5000, 7)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	got, err := summarize(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := tx.Query(ctx, `SELECT price, stock, is_active FROM products`)
	if err != nil {
		t.Fatal(err)
	}
	var want Summary
	for rows.Next() {
		var price, stock int64
		var active bool
		if err := rows.Scan(&price, &stock, &active); err != nil {
			t.Fatal(err)
		}
		want.TotalProducts++
		if active {
			want.ActiveProducts++
		}
		want.TotalStockValue += price * stock
		if stock <= LowStockThreshold {
			want.LowStockCount++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatalf("summary %+v, recount %+v", got, want)
	}
	if got.TotalStockValue < 2147483647*1000000 {
		t.Fatalf("stock value %d did not include the big product", got.TotalStockValue)
	}
}
