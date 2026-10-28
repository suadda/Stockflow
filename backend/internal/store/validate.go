package store

import (
	"encoding/json"
	"errors"
	"fmt"
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

// maxCustomer bounds the customer name, in characters.
const maxCustomer = 200

// CatalogEntry is the price and status of a product referenced by an order.
type CatalogEntry struct {
	Price    int64
	IsActive bool
}

// ValidateOrder checks a whole order request against the catalog of the
// products it references and reports every failure: customer 1..200 chars,
// at least one item, known active non-duplicate SKUs, qty 1..1,000,000 and a
// total that fits the INTEGER total column.
func ValidateOrder(in OrderInput, catalog map[string]CatalogEntry) []FieldError {
	var errs []FieldError
	customer := strings.TrimSpace(in.Customer)
	switch {
	case customer == "":
		errs = addErr(errs, "customer", "customer is required")
	case utf8.RuneCountInString(customer) > maxCustomer:
		errs = addErr(errs, "customer", "customer must be at most 200 characters")
	}
	if len(in.Items) == 0 {
		return addErr(errs, "items", "items must contain at least one item")
	}

	seen := make(map[string]bool, len(in.Items))
	var total int64
	over := false
	for i, it := range in.Items {
		skuField := fmt.Sprintf("items[%d].sku", i)
		sku := strings.TrimSpace(it.SKU)
		qty, qmsg := parseQty(it.Qty)
		if qmsg != "" {
			errs = addErr(errs, fmt.Sprintf("items[%d].qty", i), qmsg)
		}
		entry, known := catalog[sku]
		switch {
		case sku == "":
			errs = addErr(errs, skuField, "sku is required")
			continue
		case seen[sku]:
			errs = addErr(errs, skuField, fmt.Sprintf("duplicate sku %q", sku))
			continue
		case !known:
			errs = addErr(errs, skuField, fmt.Sprintf("unknown sku %q", sku))
		case !entry.IsActive:
			errs = addErr(errs, skuField, fmt.Sprintf("product %q is inactive", sku))
		}
		seen[sku] = true
		// Each term is at most 2^31 * 10^6, so stopping once over the limit
		// keeps the int64 sum from overflowing.
		if known && qmsg == "" && !over {
			total += entry.Price * qty
			over = total > maxInt32
		}
	}
	if over {
		errs = addErr(errs, "items", "order total exceeds 2147483647")
	}
	return errs
}
