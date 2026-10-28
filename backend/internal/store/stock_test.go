package store

import (
	"context"
	"errors"
	"testing"
)

func TestAdjustStockInsufficientWritesNothing(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	p := newProduct(t, s, pfx+"A", 1000)
	if _, err := s.AdjustStock(ctx, p.ID, stockIn(5)); err != nil {
		t.Fatalf("in: %v", err)
	}

	_, err := s.AdjustStock(ctx, p.ID, stockOut(8))
	var ie *InsufficientStockError
	if !errors.As(err, &ie) || ie.Available != 5 {
		t.Fatalf("err = %v, want InsufficientStockError{Available: 5}", err)
	}

	got, err := s.GetProduct(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stock != 5 {
		t.Fatalf("stock = %d, want 5 (unchanged)", got.Stock)
	}
	if n := movementCount(t, s, p.ID); n != 1 {
		t.Fatalf("movements = %d, want 1 (no movement for the rejected out)", n)
	}
}

func TestAdjustStockHappyPath(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	p := newProduct(t, s, pfx+"A", 1000)

	after, err := s.AdjustStock(ctx, p.ID, stockIn(5))
	if err != nil || after.Stock != 5 || after.LastMovementAt == nil {
		t.Fatalf("in: %+v, %v", after, err)
	}
	if n := movementCount(t, s, p.ID); n != 1 {
		t.Fatalf("movements after in = %d, want 1", n)
	}

	after, err = s.AdjustStock(ctx, p.ID, stockOut(2))
	if err != nil || after.Stock != 3 {
		t.Fatalf("out: %+v, %v", after, err)
	}
	ms, err := s.ListMovements(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("movements = %d, want 2", len(ms))
	}

	if ms[0].Type != MovementOut || ms[0].Qty != 2 || ms[1].Type != MovementIn || ms[1].Qty != 5 {
		t.Fatalf("movements order/content: %+v", ms)
	}
	if ms[0].Note == nil || *ms[0].Note != "test out" || ms[0].ProductID != p.ID {
		t.Fatalf("movement fields: %+v", ms[0])
	}

	if after, err = s.AdjustStock(ctx, p.ID, stockOut(3)); err != nil || after.Stock != 0 {
		t.Fatalf("drain: %+v, %v", after, err)
	}
}

func TestAdjustStockNotFound(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	for _, in := range []StockInput{stockIn(1), stockOut(1)} {
		if _, err := s.AdjustStock(ctx, 2147483000, in); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s: err = %v, want ErrNotFound", in.Type, err)
		}
	}
	if _, err := s.ListMovements(ctx, 2147483000); !errors.Is(err, ErrNotFound) {
		t.Fatalf("movements: err = %v, want ErrNotFound", err)
	}
}
