package store

import (
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

var ErrSKUExists = errors.New("sku already exists")

var ErrHasHistory = errors.New("product has history")

var ErrAlreadyCancelled = errors.New("order already cancelled")

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct{ Details []FieldError }

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %d field error(s)", len(e.Details))
}

type InsufficientStockError struct {
	SKU       string
	Available int64
}

func (e *InsufficientStockError) Error() string {
	if e.SKU == "" {
		return fmt.Sprintf("insufficient stock: only %d available", e.Available)
	}
	return fmt.Sprintf("insufficient stock for %s: only %d available", e.SKU, e.Available)
}
