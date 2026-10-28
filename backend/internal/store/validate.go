package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxInt32 = 2147483647

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

const maxQty = 1_000_000

const maxNote = 500

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

const maxCustomer = 200

type CatalogEntry struct {
	Price    int64
	IsActive bool
}

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
