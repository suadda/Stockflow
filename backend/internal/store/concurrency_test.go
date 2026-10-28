package store

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func run(n int, fn func(i int) error) []error {
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = fn(i)
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func tally(t *testing.T, errs []error) (ok, short int) {
	t.Helper()
	for _, err := range errs {
		var ie *InsufficientStockError
		switch {
		case err == nil:
			ok++
		case errors.As(err, &ie):
			short++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	return ok, short
}

func TestConcurrentOrdersNeverOversell(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	p := stocked(t, s, pfx+"A", 1000, 10)

	errs := run(50, func(int) error {
		_, err := s.CreateOrder(ctx, order("Racer", item(p.SKU, 1)))
		return err
	})
	ok, short := tally(t, errs)
	if ok != 10 || short != 40 {
		t.Fatalf("accepted %d, rejected %d; want 10 and 40", ok, short)
	}
	if got := stockOf(t, s, p.ID); got != 0 {
		t.Fatalf("final stock %d, want 0", got)
	}
	if n := movementsOf(t, s, p.ID, MovementOut); n != 10 {
		t.Fatalf("%d out movements, want 10", n)
	}
	if n := ordersFor(t, s, pfx); n != 10 {
		t.Fatalf("%d orders, want 10", n)
	}
}

func TestConcurrentReversedOrdersDoNotDeadlock(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	a := stocked(t, s, pfx+"A", 1000, 100)
	b := stocked(t, s, pfx+"B", 2000, 100)

	errs := run(40, func(i int) error {
		in := order("Racer", item(a.SKU, 1), item(b.SKU, 1))
		if i%2 == 1 {
			in = order("Racer", item(b.SKU, 1), item(a.SKU, 1))
		}
		_, err := s.CreateOrder(ctx, in)
		return err
	})
	if ok, _ := tally(t, errs); ok != 40 {
		t.Fatalf("%d of 40 orders succeeded", ok)
	}
	for _, p := range []Product{a, b} {
		if got := stockOf(t, s, p.ID); got != 60 {
			t.Fatalf("%s stock %d, want 60", p.SKU, got)
		}
		if n := movementsOf(t, s, p.ID, MovementOut); n != 40 {
			t.Fatalf("%s: %d out movements, want 40", p.SKU, n)
		}
	}
}

func TestConcurrentOutAdjustmentsNeverGoNegative(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	p := stocked(t, s, pfx+"A", 1000, 10)

	errs := run(30, func(int) error {
		_, err := s.AdjustStock(ctx, p.ID, stockOut(1))
		return err
	})
	ok, short := tally(t, errs)
	if ok != 10 || short != 20 {
		t.Fatalf("accepted %d, rejected %d; want 10 and 20", ok, short)
	}
	if got := stockOf(t, s, p.ID); got != 0 {
		t.Fatalf("final stock %d, want 0", got)
	}
}

func TestConcurrentCancelsRestoreOnce(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	p := stocked(t, s, pfx+"A", 1000, 10)
	o, err := s.CreateOrder(ctx, order("Toko Maju", item(p.SKU, 3)))
	if err != nil {
		t.Fatal(err)
	}

	errs := run(10, func(int) error {
		_, err := s.CancelOrder(ctx, o.ID)
		return err
	})
	ok, already := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrAlreadyCancelled):
			already++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || already != 9 {
		t.Fatalf("cancelled %d, already %d; want 1 and 9", ok, already)
	}
	if got := stockOf(t, s, p.ID); got != 10 {
		t.Fatalf("stock %d, want 10 (restored exactly once)", got)
	}
}
