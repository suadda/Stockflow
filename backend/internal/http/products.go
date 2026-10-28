package httpapi

import (
	"net/http"

	"stockflow/internal/store"
)

func listProducts(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, errs := parseListParams(r.URL.Query())
		if len(errs) > 0 {
			writeStoreError(w, &store.ValidationError{Details: errs})
			return
		}
		items, total, err := s.ListProducts(r.Context(), f)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": items, "page": f.Page, "limit": f.Limit, "total": total,
		})
	}
}

func createProduct(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in store.ProductInput
		if err := decodeJSON(r, &in); err != nil {
			writeStoreError(w, err)
			return
		}
		p, err := s.CreateProduct(r.Context(), in)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, p)
	}
}

func updateProduct(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(r)
		if !ok {
			writeStoreError(w, store.ErrNotFound)
			return
		}
		var in store.ProductUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeStoreError(w, err)
			return
		}
		p, err := s.UpdateProduct(r.Context(), id, in)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func deleteProduct(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(r)
		if !ok {
			writeStoreError(w, store.ErrNotFound)
			return
		}
		if err := s.DeleteProduct(r.Context(), id); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func adjustStock(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(r)
		if !ok {
			writeStoreError(w, store.ErrNotFound)
			return
		}
		var in store.StockInput
		if err := decodeJSON(r, &in); err != nil {
			writeStoreError(w, err)
			return
		}
		p, err := s.AdjustStock(r.Context(), id, in)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func listMovements(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(r)
		if !ok {
			writeStoreError(w, store.ErrNotFound)
			return
		}
		ms, err := s.ListMovements(r.Context(), id)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": ms})
	}
}
