package httpapi

import (
	"net/http"

	"stockflow/internal/store"
)

func createOrder(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in store.OrderInput
		if err := decodeJSON(r, &in); err != nil {
			writeStoreError(w, err)
			return
		}
		o, err := s.CreateOrder(r.Context(), in)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, o)
	}
}

func getOrder(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(r)
		if !ok {
			writeStoreError(w, store.ErrNotFound)
			return
		}
		o, err := s.GetOrder(r.Context(), id)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	}
}

func cancelOrder(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(r)
		if !ok {
			writeStoreError(w, store.ErrNotFound)
			return
		}
		o, err := s.CancelOrder(r.Context(), id)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	}
}
