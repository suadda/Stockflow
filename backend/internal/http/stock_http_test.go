package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type movementJSON struct {
	ID        int64   `json:"id"`
	ProductID int64   `json:"product_id"`
	Type      string  `json:"type"`
	Qty       int64   `json:"qty"`
	Note      *string `json:"note"`
	CreatedAt string  `json:"created_at"`
}

func movements(t *testing.T, h http.Handler, id int64) []movementJSON {
	t.Helper()
	rec := call(h, http.MethodGet, fmt.Sprintf("/api/products/%d/movements", id), "")
	expect(t, rec, http.StatusOK)
	return decodeAs[struct {
		Data []movementJSON `json:"data"`
	}](t, rec).Data
}

func TestStockHTTP(t *testing.T) {
	h, _, pfx := testAPI(t)
	p := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sA","name":"Cable","price":25000}`, pfx))
	stock := fmt.Sprintf("/api/products/%d/stock", p.ID)

	if ms := movements(t, h, p.ID); len(ms) != 0 {
		t.Fatalf("fresh product has movements: %+v", ms)
	}

	// (b) happy path: in 5 then out 2.
	rec := call(h, http.MethodPost, stock, `{"type":"in","qty":5,"note":"receipt"}`)
	expect(t, rec, http.StatusOK)
	if got := decodeAs[productJSON](t, rec); got.Stock != 5 || got.LastMovementAt == nil {
		t.Fatalf("after in: %+v", got)
	}
	rec = call(h, http.MethodPost, stock, `{"type":"out","qty":2}`)
	expect(t, rec, http.StatusOK)
	if got := decodeAs[productJSON](t, rec); got.Stock != 3 {
		t.Fatalf("after out: %+v", got)
	}
	ms := movements(t, h, p.ID)
	if len(ms) != 2 || ms[0].Type != "out" || ms[0].Qty != 2 || ms[0].Note != nil ||
		ms[1].Type != "in" || ms[1].Qty != 5 || ms[1].Note == nil || *ms[1].Note != "receipt" {
		t.Fatalf("movements: %+v", ms)
	}

	// (a) overdraw: 409 with available, nothing written.
	e := expectError(t, call(h, http.MethodPost, stock, `{"type":"out","qty":8,"note":"too many"}`),
		http.StatusConflict, "INSUFFICIENT_STOCK")
	if e["available"] != float64(3) {
		t.Fatalf("available = %v, want 3", e["available"])
	}
	if _, has := e["sku"]; has {
		t.Fatalf("adjust error should not carry sku: %v", e)
	}
	if got := list(t, h, "search="+p.SKU); got.Data[0].Stock != 3 {
		t.Fatalf("stock changed after rejected out: %+v", got.Data[0])
	}
	if ms := movements(t, h, p.ID); len(ms) != 2 {
		t.Fatalf("movement written for rejected out: %+v", ms)
	}

	// Validation.
	cases := map[string]string{
		`{"type":"sideways","qty":1}`: "type",
		`{"type":"in","qty":0}`:       "qty",
		`{"type":"in","qty":1.5}`:     "qty",
		`{"type":"in","qty":1000001}`: "qty",
		`{"type":"in"}`:               "qty",
		`{"type":"in","qty":1,"note":"` + strings.Repeat("é", 501) + `"}`: "note",
	}
	for body, field := range cases {
		e := expectError(t, call(h, http.MethodPost, stock, body), http.StatusBadRequest, "VALIDATION_ERROR")
		if !detailFields(e)[field] {
			t.Fatalf("%s: want %s detail, got %v", body, field, e)
		}
	}
	expect(t, call(h, http.MethodPost, stock,
		`{"type":"in","qty":1,"note":"`+strings.Repeat("é", 500)+`"}`), http.StatusOK)

	// Missing product.
	expectError(t, call(h, http.MethodPost, "/api/products/2147483000/stock", `{"type":"in","qty":1}`),
		http.StatusNotFound, "NOT_FOUND")
	expectError(t, call(h, http.MethodGet, "/api/products/2147483000/movements", ""),
		http.StatusNotFound, "NOT_FOUND")
}
