package store

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxInt32 is the upper bound of the INTEGER money and quantity columns.
const maxInt32 = 2147483647

// parsePrice converts a JSON number to an integer price in 0..maxInt32.
// It returns a client-facing message when the value is missing or invalid.
func parsePrice(n json.Number) (int64, string) {
	if n == "" {
		return 0, "price is required"
	}
	v, err := strconv.ParseInt(string(n), 10, 64)
	switch {
	case errors.Is(err, strconv.ErrRange):
		return 0, "price must be between 0 and 2147483647"
	case err != nil:
		return 0, "price must be an integer"
	case v < 0 || v > maxInt32:
		return 0, "price must be between 0 and 2147483647"
	}
	return v, ""
}

func addErr(errs []FieldError, field, msg string) []FieldError {
	return append(errs, FieldError{Field: field, Message: msg})
}

// ValidateProductInput checks a create request: sku and name non-blank after
// trimming, price an integer in 0..2147483647. Every failure is reported.
func ValidateProductInput(in ProductInput) []FieldError {
	var errs []FieldError
	if strings.TrimSpace(in.SKU) == "" {
		errs = addErr(errs, "sku", "sku is required")
	}
	if strings.TrimSpace(in.Name) == "" {
		errs = addErr(errs, "name", "name is required")
	}
	if _, msg := parsePrice(in.Price); msg != "" {
		errs = addErr(errs, "price", msg)
	}
	return errs
}

// ValidateProductUpdate checks an update request against the stored SKU:
// the SKU is immutable, name non-blank, price valid, is_active present.
func ValidateProductUpdate(in ProductUpdate, storedSKU string) []FieldError {
	var errs []FieldError
	if in.SKU != nil && strings.TrimSpace(*in.SKU) != storedSKU {
		errs = addErr(errs, "sku", "sku is immutable and cannot be changed")
	}
	if strings.TrimSpace(in.Name) == "" {
		errs = addErr(errs, "name", "name is required")
	}
	if _, msg := parsePrice(in.Price); msg != "" {
		errs = addErr(errs, "price", msg)
	}
	if in.IsActive == nil {
		errs = addErr(errs, "is_active", "is_active is required")
	}
	return errs
}

// maxQty bounds a single stock adjustment or order line.
const maxQty = 1_000_000

// maxNote bounds a stock movement note, in characters.
const maxNote = 500

// parseQty converts a JSON number to a quantity in 1..maxQty.
func parseQty(n json.Number) (int64, string) {
	if n == "" {
		return 0, "qty is required"
	}
	v, err := strconv.ParseInt(string(n), 10, 64)
	if errors.Is(err, strconv.ErrRange) || (err == nil && (v < 1 || v > maxQty)) {
		return 0, "qty must be between 1 and 1000000"
	}
	if err != nil {
		return 0, "qty must be an integer"
	}
	return v, ""
}

// ValidateStockInput checks type is in/out, qty is an integer in
// 1..1,000,000 and note is at most 500 characters.
func ValidateStockInput(in StockInput) []FieldError {
	var errs []FieldError
	if in.Type != MovementIn && in.Type != MovementOut {
		errs = addErr(errs, "type", `type must be "in" or "out"`)
	}
	if _, msg := parseQty(in.Qty); msg != "" {
		errs = addErr(errs, "qty", msg)
	}
	if utf8.RuneCountInString(in.Note) > maxNote {
		errs = addErr(errs, "note", "note must be at most 500 characters")
	}
	return errs
}
