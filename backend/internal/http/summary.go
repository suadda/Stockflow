package httpapi

import (
	"net/http"

	"stockflow/internal/store"
)

func getSummary(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sm, err := s.Summary(r.Context())
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sm)
	}
}
