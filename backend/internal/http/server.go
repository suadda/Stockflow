package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"stockflow/internal/store"
)

func NewRouter(s *store.Store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Ping(r.Context()); err != nil {
			slog.Error("healthz ping failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database unavailable", nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/products", listProducts(s))
	mux.HandleFunc("POST /api/products", createProduct(s))
	mux.HandleFunc("PUT /api/products/{id}", updateProduct(s))
	mux.HandleFunc("DELETE /api/products/{id}", deleteProduct(s))
	mux.HandleFunc("POST /api/products/{id}/stock", adjustStock(s))
	mux.HandleFunc("GET /api/products/{id}/movements", listMovements(s))
	mux.HandleFunc("POST /api/orders", createOrder(s))
	mux.HandleFunc("GET /api/orders/{id}", getOrder(s))
	mux.HandleFunc("POST /api/orders/{id}/cancel", cancelOrder(s))
	mux.HandleFunc("GET /api/summary", getSummary(s))

	mux.HandleFunc("/", fallback(mux))

	return logger(recoverer(mux))
}

var probeMethods = []string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
}

func fallback(mux *http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var allow []string
		for _, m := range probeMethods {
			probe := r.WithContext(r.Context())
			probe.Method = m
			if _, pattern := mux.Handler(probe); pattern != "/" {
				allow = append(allow, m)
			}
		}
		if len(allow) > 0 {
			w.Header().Set("Allow", strings.Join(allow, ", "))
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
			return
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found", nil)
	}
}
