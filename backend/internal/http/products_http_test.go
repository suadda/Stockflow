package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

type productJSON struct {
	ID             int64   `json:"id"`
	SKU            string  `json:"sku"`
	Name           string  `json:"name"`
	Price          int64   `json:"price"`
	Stock          int64   `json:"stock"`
	IsActive       bool    `json:"is_active"`
	CreatedAt      string  `json:"created_at"`
	LastMovementAt *string `json:"last_movement_at"`
}

type pageJSON struct {
	Data  []productJSON `json:"data"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
	Total int64         `json:"total"`
}

func mustCreate(t *testing.T, h http.Handler, body string) productJSON {
	t.Helper()
	rec := call(h, http.MethodPost, "/api/products", body)
	expect(t, rec, http.StatusCreated)
	return decodeAs[productJSON](t, rec)
}

// addIn records an 'in' movement and the matching stock increase, keeping
// the ledger consistent for other tests sharing the database.
func addIn(t *testing.T, pool *pgxpool.Pool, id int64, qty int) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		WITH u AS (UPDATE products SET stock = stock + $2 WHERE id = $1 RETURNING id)
		INSERT INTO stock_movements (product_id, type, qty, note) SELECT id, 'in', $2, 'test' FROM u`, id, qty)
	if err != nil {
		t.Fatalf("addIn: %v", err)
	}
}

func list(t *testing.T, h http.Handler, query string) pageJSON {
	t.Helper()
	rec := call(h, http.MethodGet, "/api/products?"+query, "")
	expect(t, rec, http.StatusOK)
	return decodeAs[pageJSON](t, rec)
}

func TestProductsCreate(t *testing.T) {
	h, _, pfx := testAPI(t)

	p := mustCreate(t, h, fmt.Sprintf(`{"sku":" %sA ","name":" Cable ","price":25000}`, pfx))
	if p.SKU != pfx+"A" || p.Name != "Cable" || p.Price != 25000 || p.Stock != 0 || !p.IsActive {
		t.Fatalf("created %+v", p)
	}
	if p.LastMovementAt != nil || !strings.HasSuffix(p.CreatedAt, "Z") {
		t.Fatalf("timestamps %+v", p)
	}

	inactive := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sB","name":"Old","price":0,"is_active":false}`, pfx))
	if inactive.IsActive {
		t.Fatal("is_active false ignored")
	}

	expectError(t, call(h, http.MethodPost, "/api/products",
		fmt.Sprintf(`{"sku":"%sA","name":"Again","price":1}`, pfx)), http.StatusConflict, "SKU_EXISTS")

	cases := []struct {
		body   string
		fields []string
	}{
		{`{"sku":"  ","name":"","price":-1}`, []string{"sku", "name", "price"}},
		{fmt.Sprintf(`{"sku":"%sC","name":"x","price":1.5}`, pfx), []string{"price"}},
		{fmt.Sprintf(`{"sku":"%sC","name":"x"}`, pfx), []string{"price"}},
		{fmt.Sprintf(`{"sku":"%sC","name":"x","price":2147483648}`, pfx), []string{"price"}},
	}
	for _, tc := range cases {
		e := expectError(t, call(h, http.MethodPost, "/api/products", tc.body), http.StatusBadRequest, "VALIDATION_ERROR")
		got := detailFields(e)
		if len(got) != len(tc.fields) {
			t.Fatalf("%s: fields %v, want %v", tc.body, got, tc.fields)
		}
		for _, f := range tc.fields {
			if !got[f] {
				t.Fatalf("%s: missing field %s in %v", tc.body, f, got)
			}
		}
	}

	expectError(t, call(h, http.MethodPost, "/api/products",
		fmt.Sprintf(`{"sku":"%sC","name":"x","price":1,"stock":5}`, pfx)), http.StatusBadRequest, "INVALID_JSON")
}

func TestProductsUpdate(t *testing.T) {
	h, _, pfx := testAPI(t)
	p := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sA","name":"Cable","price":25000}`, pfx))
	path := fmt.Sprintf("/api/products/%d", p.ID)

	rec := call(h, http.MethodPut, path, `{"name":"Cable 2m","price":27500,"is_active":false}`)
	expect(t, rec, http.StatusOK)
	u := decodeAs[productJSON](t, rec)
	if u.Name != "Cable 2m" || u.Price != 27500 || u.IsActive || u.SKU != p.SKU || u.Stock != 0 {
		t.Fatalf("updated %+v", u)
	}

	// Sending the unchanged SKU is fine; changing it is not.
	expect(t, call(h, http.MethodPut, path,
		fmt.Sprintf(`{"sku":"%sA","name":"Cable","price":1,"is_active":true}`, pfx)), http.StatusOK)
	e := expectError(t, call(h, http.MethodPut, path,
		`{"sku":"OTHER","name":"Cable","price":1,"is_active":true}`), http.StatusBadRequest, "VALIDATION_ERROR")
	if !detailFields(e)["sku"] {
		t.Fatalf("want sku detail: %v", e)
	}

	e = expectError(t, call(h, http.MethodPut, path, `{"name":" ","price":1}`), http.StatusBadRequest, "VALIDATION_ERROR")
	if f := detailFields(e); !f["name"] || !f["is_active"] {
		t.Fatalf("want name and is_active details: %v", e)
	}

	body := `{"name":"x","price":1,"is_active":true}`
	for _, id := range []string{"2147483000", "abc", "0", "99999999999"} {
		expectError(t, call(h, http.MethodPut, "/api/products/"+id, body), http.StatusNotFound, "NOT_FOUND")
	}
}

func TestProductsDelete(t *testing.T) {
	h, pool, pfx := testAPI(t)

	fresh := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sA","name":"Temp","price":0}`, pfx))
	path := fmt.Sprintf("/api/products/%d", fresh.ID)
	rec := call(h, http.MethodDelete, path, "")
	expect(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Fatalf("204 with body %q", rec.Body.String())
	}
	expectError(t, call(h, http.MethodDelete, path, ""), http.StatusNotFound, "NOT_FOUND")

	used := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sB","name":"Used","price":100}`, pfx))
	addIn(t, pool, used.ID, 3)
	expectError(t, call(h, http.MethodDelete, fmt.Sprintf("/api/products/%d", used.ID), ""),
		http.StatusConflict, "PRODUCT_HAS_HISTORY")
	if got := list(t, h, "search="+url.QueryEscape(used.SKU)); got.Total != 1 || got.Data[0].Stock != 3 {
		t.Fatalf("product or history changed: %+v", got)
	}
}

func TestProductsList(t *testing.T) {
	h, pool, pfx := testAPI(t)
	a := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sA","name":"Kabel Merah","price":100}`, pfx))
	b := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sB","name":"Kabel Biru","price":200}`, pfx))
	c := mustCreate(t, h, fmt.Sprintf(`{"sku":"%sC","name":"Lama","price":300,"is_active":false}`, pfx))

	q := "search=" + url.QueryEscape(strings.ToLower(pfx)) // case-insensitive
	all := list(t, h, q)
	if all.Total != 3 || len(all.Data) != 3 || all.Page != 1 || all.Limit != 20 {
		t.Fatalf("all: %+v", all)
	}
	for i, want := range []int64{a.ID, b.ID, c.ID} {
		if all.Data[i].ID != want {
			t.Fatalf("order: %+v", all.Data)
		}
	}

	if got := list(t, h, q+"&status=active"); got.Total != 2 {
		t.Fatalf("active: %+v", got)
	}
	if got := list(t, h, q+"&status=inactive"); got.Total != 1 || got.Data[0].ID != c.ID {
		t.Fatalf("inactive: %+v", got)
	}
	if got := list(t, h, q+"&status=low"); got.Total != 3 {
		t.Fatalf("low before: %+v", got)
	}

	addIn(t, pool, a.ID, 20)
	if got := list(t, h, q+"&status=low"); got.Total != 2 {
		t.Fatalf("low after stock 20: %+v", got)
	}
	got := list(t, h, "search="+url.QueryEscape(a.SKU))
	if got.Total != 1 || got.Data[0].LastMovementAt == nil || !strings.HasSuffix(*got.Data[0].LastMovementAt, "Z") {
		t.Fatalf("last_movement_at not set: %+v", got)
	}

	// Name search, paging, clamping.
	if got := list(t, h, q+"&search=biru"); got.Total < 1 {
		t.Fatalf("name search: %+v", got)
	}
	if got := list(t, h, q+"&page=2&limit=2"); got.Total != 3 || len(got.Data) != 1 || got.Data[0].ID != c.ID || got.Page != 2 || got.Limit != 2 {
		t.Fatalf("paging: %+v", got)
	}
	if got := list(t, h, q+"&limit=500"); got.Limit != 100 {
		t.Fatalf("clamp: %+v", got)
	}

	// Wildcards match literally; empty results are [] not null.
	rec := call(h, http.MethodGet, "/api/products?search="+url.QueryEscape(pfx+"%"), "")
	expect(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Fatalf("want empty data array: %s", rec.Body.String())
	}

	for _, bad := range []string{"page=0", "limit=abc", "page=-1", "status=bogus"} {
		expectError(t, call(h, http.MethodGet, "/api/products?"+bad, ""), http.StatusBadRequest, "VALIDATION_ERROR")
	}
}

func TestProductsConcurrentDuplicateCreate(t *testing.T) {
	h, _, pfx := testAPI(t)
	body := fmt.Sprintf(`{"sku":"%sDUP","name":"Race","price":1}`, pfx)

	const n = 10
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = call(h, http.MethodPost, "/api/products", body).Code
		}()
	}
	wg.Wait()

	created, conflict := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflict++
		}
	}
	if created != 1 || conflict != n-1 {
		t.Fatalf("codes %v: want exactly one 201 and %d 409s", codes, n-1)
	}
}
