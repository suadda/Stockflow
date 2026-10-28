package httpapi

import (
	"fmt"
	"net/http"
	"testing"
)

type orderJSON struct {
	ID          int64   `json:"id"`
	Customer    string  `json:"customer"`
	Status      string  `json:"status"`
	Total       int64   `json:"total"`
	CreatedAt   string  `json:"created_at"`
	CancelledAt *string `json:"cancelled_at"`
	Items       []struct {
		SKU       string `json:"sku"`
		Qty       int64  `json:"qty"`
		UnitPrice int64  `json:"unit_price"`
		Subtotal  int64  `json:"subtotal"`
	} `json:"items"`
}

func TestOrdersHTTP(t *testing.T) {
	h, pool, pfx := testAPI(t)
	a := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sA","name":"Cable","price":45000}`, pfx))
	b := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sB","name":"Toner","price":1250000}`, pfx))
	addIn(t, pool, a.ID, 10)
	addIn(t, pool, b.ID, 2)
	stockOfSKU := func(sku string) int64 {
		return list(t, h, "search="+sku).Data[0].Stock
	}

	// Happy path: server prices and total.
	rec := call(h, http.MethodPost, "/api/orders",
		fmt.Sprintf(`{"customer":"Toko Maju","items":[{"sku":"%s","qty":3},{"sku":"%s","qty":1}]}`, a.SKU, b.SKU))
	expect(t, rec, http.StatusCreated)
	o := decodeAs[orderJSON](t, rec)
	if o.Status != "CREATED" || o.Total != 3*45000+1250000 || o.CancelledAt != nil || len(o.Items) != 2 {
		t.Fatalf("order: %+v", o)
	}
	for _, it := range o.Items {
		if it.Subtotal != it.Qty*it.UnitPrice || (it.SKU == a.SKU && it.UnitPrice != 45000) || (it.SKU == b.SKU && it.UnitPrice != 1250000) {
			t.Fatalf("item: %+v", it)
		}
	}
	if stockOfSKU(a.SKU) != 7 || stockOfSKU(b.SKU) != 1 {
		t.Fatal("stock not decremented")
	}

	// A client-supplied price is rejected, never used.
	expectError(t, call(h, http.MethodPost, "/api/orders",
		fmt.Sprintf(`{"customer":"X","items":[{"sku":"%s","qty":1,"unit_price":1}]}`, a.SKU)),
		http.StatusBadRequest, "INVALID_JSON")

	// (c) over HTTP: 409 names the short SKU; nothing changes.
	e := expectError(t, call(h, http.MethodPost, "/api/orders",
		fmt.Sprintf(`{"customer":"X","items":[{"sku":"%s","qty":2},{"sku":"%s","qty":5}]}`, a.SKU, b.SKU)),
		http.StatusConflict, "INSUFFICIENT_STOCK")
	if e["sku"] != b.SKU || e["available"] != float64(1) {
		t.Fatalf("409 body: %v", e)
	}
	if stockOfSKU(a.SKU) != 7 || stockOfSKU(b.SKU) != 1 {
		t.Fatal("stock changed after 409")
	}

	// 400s with field paths.
	cases := map[string]string{
		`{"customer":"X","items":[]}`: "items",
		`{"customer":"X"}`:            "items",
		fmt.Sprintf(`{"customer":"","items":[{"sku":"%s","qty":1}]}`, a.SKU):                              "customer",
		fmt.Sprintf(`{"customer":"X","items":[{"sku":"%sNOPE","qty":1}]}`, pfx):                           "items[0].sku",
		fmt.Sprintf(`{"customer":"X","items":[{"sku":"%s","qty":1},{"sku":"%s","qty":1}]}`, a.SKU, a.SKU): "items[1].sku",
		fmt.Sprintf(`{"customer":"X","items":[{"sku":"%s","qty":0}]}`, a.SKU):                             "items[0].qty",
		fmt.Sprintf(`{"customer":"X","items":[{"sku":"%s","qty":1.5}]}`, a.SKU):                           "items[0].qty",
	}
	for body, field := range cases {
		e := expectError(t, call(h, http.MethodPost, "/api/orders", body), http.StatusBadRequest, "VALIDATION_ERROR")
		if !detailFields(e)[field] {
			t.Fatalf("%s: want %s detail, got %v", body, field, e)
		}
	}

	// Get, cancel twice, 404s.
	path := fmt.Sprintf("/api/orders/%d", o.ID)
	rec = call(h, http.MethodGet, path, "")
	expect(t, rec, http.StatusOK)
	if got := decodeAs[orderJSON](t, rec); got.ID != o.ID || got.Total != o.Total {
		t.Fatalf("get: %+v", got)
	}
	rec = call(h, http.MethodPost, path+"/cancel", "")
	expect(t, rec, http.StatusOK)
	if got := decodeAs[orderJSON](t, rec); got.Status != "CANCELLED" || got.CancelledAt == nil {
		t.Fatalf("cancel: %+v", got)
	}
	if stockOfSKU(a.SKU) != 10 || stockOfSKU(b.SKU) != 2 {
		t.Fatal("stock not restored")
	}
	expectError(t, call(h, http.MethodPost, path+"/cancel", ""), http.StatusConflict, "ALREADY_CANCELLED")
	if stockOfSKU(a.SKU) != 10 {
		t.Fatal("second cancel changed stock")
	}
	expectError(t, call(h, http.MethodGet, "/api/orders/2147483000", ""), http.StatusNotFound, "NOT_FOUND")
	expectError(t, call(h, http.MethodPost, "/api/orders/2147483000/cancel", ""), http.StatusNotFound, "NOT_FOUND")
}
