package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterFallback(t *testing.T) {
	h := NewRouter(nil)
	cases := []struct {
		method, path string
		status       int
		code, allow  string
	}{
		{http.MethodGet, "/nope", http.StatusNotFound, "NOT_FOUND", ""},
		{http.MethodPost, "/healthz", http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.status {
			t.Fatalf("%s %s: status %d, want %d", tc.method, tc.path, rec.Code, tc.status)
		}
		var body struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s: decode: %v", tc.method, tc.path, err)
		}
		if body.Error.Code != tc.code || body.Error.Message == "" {
			t.Fatalf("%s %s: error %+v, want code %s", tc.method, tc.path, body.Error, tc.code)
		}
		if got := rec.Header().Get("Allow"); got != tc.allow {
			t.Fatalf("%s %s: Allow %q, want %q", tc.method, tc.path, got, tc.allow)
		}
	}
}
