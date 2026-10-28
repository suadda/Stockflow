package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestCreateOrderHappyPath(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	a := stocked(t, s, pfx+"A", 45000, 10)
	b := stocked(t, s, pfx+"B", 1250000, 3)

	// Request order B, A; stock is taken in sorted order but the result is the same.
	o, err := s.CreateOrder(ctx, order(" Toko Maju ", item(b.SKU, 2), item(a.SKU, 4)))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if o.Status != StatusCreated || o.Customer != "Toko Maju" || o.CancelledAt != nil {
		t.Fatalf("order: %+v", o)
	}
	if want := int64(2*1250000 + 4*45000); o.Total != want {
		t.Fatalf("total = %d, want %d", o.Total, want)
	}
	prices := map[string]int64{a.SKU: a.Price, b.SKU: b.Price}
	for _, it := range o.Items {
		if it.UnitPrice != prices[it.SKU] || it.Subtotal != it.Qty*it.UnitPrice {
			t.Fatalf("item: %+v", it)
		}
	}
	if len(o.Items) != 2 {
		t.Fatalf("items: %+v", o.Items)
	}
	if stockOf(t, s, a.ID) != 6 || stockOf(t, s, b.ID) != 1 {
		t.Fatal("stock not decremented")
	}
	note := fmt.Sprintf("order #%d", o.ID)
	for _, id := range []int64{a.ID, b.ID} {
		if n := countWhere(t, s, `SELECT count(*) FROM stock_movements WHERE product_id = $1 AND type = 'out' AND note = $2`, id, note); n != 1 {
			t.Fatalf("product %d: %d out movements noted %q, want 1", id, n, note)
		}
	}

	got, err := s.GetOrder(ctx, o.ID)
	if err != nil || got.Total != o.Total || len(got.Items) != 2 {
		t.Fatalf("get: %+v, %v", got, err)
	}
}

// Required test (c): one insufficient item rolls back the whole order.
func TestCreateOrderInsufficientRollsBack(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	a := stocked(t, s, pfx+"A", 1000, 5)
	b := stocked(t, s, pfx+"B", 2000, 1) // second in sorted order, so A was already decremented

	_, err := s.CreateOrder(ctx, order("Toko Maju", item(a.SKU, 2), item(b.SKU, 3)))
	var ie *InsufficientStockError
	if !errors.As(err, &ie) || ie.SKU != b.SKU || ie.Available != 1 {
		t.Fatalf("err = %v, want InsufficientStockError{%s, 1}", err, b.SKU)
	}
	if stockOf(t, s, a.ID) != 5 || stockOf(t, s, b.ID) != 1 {
		t.Fatal("stock changed after rollback")
	}
	if movementsOf(t, s, a.ID, MovementOut)+movementsOf(t, s, b.ID, MovementOut) != 0 {
		t.Fatal("out movement written after rollback")
	}
	if n := ordersFor(t, s, pfx); n != 0 {
		t.Fatalf("%d orders/items written after rollback", n)
	}
	if n := countWhere(t, s, `SELECT count(*) FROM sales_order_items WHERE sku LIKE $1`, pfx+"%"); n != 0 {
		t.Fatalf("%d order items written after rollback", n)
	}
}

func TestCreateOrderValidation(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	a := stocked(t, s, pfx+"A", 1000, 5)
	off := stocked(t, s, pfx+"OFF", 1000, 5)
	if _, err := s.UpdateProduct(ctx, off.ID, ProductUpdate{Name: "Off", Price: "1000", IsActive: new(bool)}); err != nil {
		t.Fatal(err)
	}
	huge := newProduct(t, s, pfx+"HUGE", 2147483647)

	cases := []struct {
		name   string
		in     OrderInput
		fields []string
	}{
		{"empty items", order("X"), []string{"items"}},
		{"blank customer", order(" ", item(a.SKU, 1)), []string{"customer"}},
		{"unknown sku", order("X", item(pfx+"NOPE", 1)), []string{"items[0].sku"}},
		{"duplicate sku", order("X", item(a.SKU, 1), item(a.SKU, 2)), []string{"items[1].sku"}},
		{"qty zero", order("X", item(a.SKU, 0)), []string{"items[0].qty"}},
		{"inactive", order("X", item(off.SKU, 1)), []string{"items[0].sku"}},
		{"total too big", order("X", item(huge.SKU, 2)), []string{"items"}},
		{"all together", order("", item(pfx+"NOPE", 1), item(a.SKU, -1)), []string{"customer", "items[0].sku", "items[1].qty"}},
	}
	for _, tc := range cases {
		_, err := s.CreateOrder(ctx, tc.in)
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("%s: err = %v, want ValidationError", tc.name, err)
		}
		got := map[string]bool{}
		for _, d := range ve.Details {
			got[d.Field] = true
		}
		if len(got) != len(tc.fields) {
			t.Fatalf("%s: fields %v, want %v", tc.name, ve.Details, tc.fields)
		}
		for _, f := range tc.fields {
			if !got[f] {
				t.Fatalf("%s: missing %s in %v", tc.name, f, ve.Details)
			}
		}
	}
	if stockOf(t, s, a.ID) != 5 || ordersFor(t, s, pfx) != 0 {
		t.Fatal("validation failure wrote data")
	}
}

func TestCancelOrder(t *testing.T) {
	s, pfx := testStore(t)
	ctx := context.Background()
	a := stocked(t, s, pfx+"A", 1000, 5)
	b := stocked(t, s, pfx+"B", 2000, 5)
	o, err := s.CreateOrder(ctx, order("Toko Maju", item(a.SKU, 2), item(b.SKU, 5)))
	if err != nil {
		t.Fatal(err)
	}

	c, err := s.CancelOrder(ctx, o.ID)
	if err != nil || c.Status != StatusCancelled || c.CancelledAt == nil || c.Total != o.Total {
		t.Fatalf("cancel: %+v, %v", c, err)
	}
	if stockOf(t, s, a.ID) != 5 || stockOf(t, s, b.ID) != 5 {
		t.Fatal("stock not restored")
	}
	if movementsOf(t, s, a.ID, MovementIn) != 2 || movementsOf(t, s, b.ID, MovementIn) != 2 {
		t.Fatal("want initial receipt + one cancel 'in' movement per product")
	}

	if _, err := s.CancelOrder(ctx, o.ID); !errors.Is(err, ErrAlreadyCancelled) {
		t.Fatalf("second cancel: %v", err)
	}
	if stockOf(t, s, a.ID) != 5 || stockOf(t, s, b.ID) != 5 {
		t.Fatal("second cancel changed stock")
	}

	// A cancelled order still blocks deleting its products: history is kept.
	if err := s.DeleteProduct(ctx, a.ID); !errors.Is(err, ErrHasHistory) {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.CancelOrder(ctx, 2147483000); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing cancel: %v", err)
	}
	if _, err := s.GetOrder(ctx, 2147483000); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing get: %v", err)
	}
}
