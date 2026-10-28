package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"stockflow/internal/store"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

// envelope decodes {"error":{...}} into a generic map.
func envelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode envelope %q: %v", rec.Body.String(), err)
	}
	if body.Error == nil {
		t.Fatalf("no error object in %q", rec.Body.String())
	}
	if c, _ := body.Error["code"].(string); c == "" {
		t.Fatalf("empty code in %q", rec.Body.String())
	}
	if m, _ := body.Error["message"].(string); m == "" {
		t.Fatalf("empty message in %q", rec.Body.String())
	}
	return body.Error
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusConflict, "INSUFFICIENT_STOCK", "only 2 left",
		map[string]any{"available": 2, "code": "HIJACK", "message": "HIJACK"})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type %q", ct)
	}
	e := envelope(t, rec)
	if e["code"] != "INSUFFICIENT_STOCK" || e["message"] != "only 2 left" {
		t.Fatalf("extras overrode code/message: %v", e)
	}
	if e["available"] != float64(2) {
		t.Fatalf("available = %v", e["available"])
	}
}

func TestDecodeJSONInvalid(t *testing.T) {
	type dst struct {
		A int `json:"a"`
	}
	cases := map[string]string{
		"malformed":     `{"a":`,
		"unknown field": `{"a":1,"price":5}`,
		"trailing data": `{"a":1}{"b":2}`,
		"empty body":    ``,
		"over 1 MiB":    `{"a":1,"pad":"` + strings.Repeat("x", maxBodyBytes) + `"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			var d dst
			err := decodeJSON(r, &d)
			if !errors.Is(err, errInvalidJSON) {
				t.Fatalf("err = %v, want errInvalidJSON", err)
			}
			rec := httptest.NewRecorder()
			writeStoreError(rec, err)
			if rec.Code != http.StatusBadRequest || envelope(t, rec)["code"] != "INVALID_JSON" {
				t.Fatalf("got %d %s", rec.Code, rec.Body.String())
			}
		})
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":7}`))
	var d dst
	if err := decodeJSON(r, &d); err != nil || d.A != 7 {
		t.Fatalf("valid body: err=%v a=%d", err, d.A)
	}
}

func TestRecoverer(t *testing.T) {
	t.Run("panic becomes 500 INTERNAL", func(t *testing.T) {
		h := logger(recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("db password is hunter2")
		})))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		e := envelope(t, rec)
		if rec.Code != http.StatusInternalServerError || e["code"] != "INTERNAL" {
			t.Fatalf("got %d %v", rec.Code, e)
		}
		if strings.Contains(rec.Body.String(), "hunter2") {
			t.Fatal("panic text leaked to client")
		}
	})

	t.Run("panic after headers keeps status", func(t *testing.T) {
		h := logger(recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			panic("late")
		})))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
			t.Fatalf("got %d body %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("ErrAbortHandler re-panics", func(t *testing.T) {
		h := recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		}))
		defer func() {
			if v := recover(); v != http.ErrAbortHandler {
				t.Fatalf("recovered %v, want http.ErrAbortHandler", v)
			}
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	})
}

func TestWriteStoreError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
		check  func(t *testing.T, e map[string]any)
	}{
		{"validation nil details", &store.ValidationError{}, 400, "VALIDATION_ERROR",
			func(t *testing.T, e map[string]any) {
				if d, ok := e["details"].([]any); !ok || len(d) != 0 {
					t.Fatalf("details = %v, want []", e["details"])
				}
			}},
		{"validation details", &store.ValidationError{Details: []store.FieldError{{Field: "sku", Message: "required"}}}, 400, "VALIDATION_ERROR",
			func(t *testing.T, e map[string]any) {
				d, _ := e["details"].([]any)
				f, _ := d[0].(map[string]any)
				if len(d) != 1 || f["field"] != "sku" || f["message"] != "required" {
					t.Fatalf("details = %v", e["details"])
				}
			}},
		{"not found", store.ErrNotFound, 404, "NOT_FOUND", nil},
		{"sku exists", store.ErrSKUExists, 409, "SKU_EXISTS", nil},
		{"has history", store.ErrHasHistory, 409, "PRODUCT_HAS_HISTORY", nil},
		{"insufficient no sku", &store.InsufficientStockError{Available: 3}, 409, "INSUFFICIENT_STOCK",
			func(t *testing.T, e map[string]any) {
				if _, has := e["sku"]; has || e["available"] != float64(3) {
					t.Fatalf("got %v", e)
				}
			}},
		{"insufficient with sku", &store.InsufficientStockError{SKU: "A", Available: 0}, 409, "INSUFFICIENT_STOCK",
			func(t *testing.T, e map[string]any) {
				if e["sku"] != "A" || e["available"] != float64(0) {
					t.Fatalf("got %v", e)
				}
			}},
		{"already cancelled", store.ErrAlreadyCancelled, 409, "ALREADY_CANCELLED", nil},
		{"pg unique violation", &pgconn.PgError{Code: "23505"}, 409, "SKU_EXISTS", nil},
		{"pg fk violation", &pgconn.PgError{Code: "23503"}, 409, "PRODUCT_HAS_HISTORY", nil},
		{"unknown error", errors.New("boom secret"), 500, "INTERNAL",
			func(t *testing.T, e map[string]any) {
				if strings.Contains(fmt.Sprint(e["message"]), "boom") {
					t.Fatal("internal error leaked")
				}
			}},
	}
	for _, tc := range cases {
		for _, wrapped := range []bool{false, true} {
			err := tc.err
			if wrapped {
				err = fmt.Errorf("ctx: %w", err)
			}
			t.Run(fmt.Sprintf("%s wrapped=%v", tc.name, wrapped), func(t *testing.T) {
				rec := httptest.NewRecorder()
				writeStoreError(rec, err)
				e := envelope(t, rec)
				if rec.Code != tc.status || e["code"] != tc.code {
					t.Fatalf("got %d %v, want %d %s", rec.Code, e["code"], tc.status, tc.code)
				}
				if tc.check != nil {
					tc.check(t, e)
				}
			})
		}
	}
}

func TestParseID(t *testing.T) {
	cases := map[string]bool{
		"1": true, "42": true,
		"0": false, "-1": false, "+1": false, "1a": false, "": false, " 1": false,
		"99999999999999999999": false,
	}
	for in, ok := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.SetPathValue("id", in)
		if _, got := parseID(r); got != ok {
			t.Errorf("parseID(%q) ok=%v, want %v", in, got, ok)
		}
	}
}
