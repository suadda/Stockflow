package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"stockflow/internal/store"
)

// maxBodyBytes caps request bodies at 1 MiB.
const maxBodyBytes = 1 << 20

// errInvalidJSON marks a request body that could not be decoded. decodeJSON
// wraps it with a human-readable reason; writeStoreError maps it to 400.
var errInvalidJSON = errors.New("invalid JSON body")

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

// writeError writes {"error":{"code":..,"message":..,...extra}}. Extras never
// overwrite code or message.
func writeError(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	body := make(map[string]any, len(extra)+2)
	for k, v := range extra {
		body[k] = v
	}
	body["code"] = code
	body["message"] = message
	writeJSON(w, status, map[string]any{"error": body})
}

// decodeJSON decodes exactly one JSON value from the body into dst, rejecting
// unknown fields, trailing data and bodies over 1 MiB. Failures wrap
// errInvalidJSON.
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %s", errInvalidJSON, decodeReason(err))
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		reason := "unexpected data after JSON value"
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			reason = decodeReason(err)
		}
		return fmt.Errorf("%w: %s", errInvalidJSON, reason)
	}
	return nil
}

// decodeReason turns a json/io error into a short client-facing reason.
func decodeReason(err error) string {
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return "request body exceeds 1 MiB"
	case errors.Is(err, io.EOF):
		return "request body is empty"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "unexpected end of JSON input"
	}
	return strings.TrimPrefix(err.Error(), "json: ")
}

// writeStoreError maps store, decode and database errors to the error envelope.
// Unknown errors are logged and reported as a generic 500.
func writeStoreError(w http.ResponseWriter, err error) {
	var verr *store.ValidationError
	var serr *store.InsufficientStockError
	var pgErr *pgconn.PgError

	switch {
	case errors.Is(err, errInvalidJSON):
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), nil)
	case errors.As(err, &verr):
		details := verr.Details
		if details == nil {
			details = []store.FieldError{}
		}
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "validation failed",
			map[string]any{"details": details})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found", nil)
	case errors.Is(err, store.ErrSKUExists):
		writeSKUExists(w)
	case errors.Is(err, store.ErrHasHistory):
		writeHasHistory(w)
	case errors.As(err, &serr):
		extra := map[string]any{"available": serr.Available}
		if serr.SKU != "" {
			extra["sku"] = serr.SKU
		}
		writeError(w, http.StatusConflict, "INSUFFICIENT_STOCK", serr.Error(), extra)
	case errors.Is(err, store.ErrAlreadyCancelled):
		writeError(w, http.StatusConflict, "ALREADY_CANCELLED", "order is already cancelled", nil)
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		writeSKUExists(w)
	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		writeHasHistory(w)
	default:
		slog.Error("internal error", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "internal server error", nil)
	}
}

func writeSKUExists(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, "SKU_EXISTS", "a product with this SKU already exists", nil)
}

func writeHasHistory(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, "PRODUCT_HAS_HISTORY",
		"product has stock movements or order items and cannot be deleted", nil)
}

// parseID returns the {id} path value when it is a positive int32 written as
// plain digits.
func parseID(r *http.Request) (int64, bool) {
	s := r.PathValue("id")
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	// ids are SERIAL (int4); anything larger cannot exist.
	id, err := strconv.ParseInt(s, 10, 32)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
