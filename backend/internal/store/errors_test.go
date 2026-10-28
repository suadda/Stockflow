package store

import (
	"errors"
	"fmt"
	"testing"
)

func TestTypedErrorsUnwrap(t *testing.T) {
	var ve *ValidationError
	err := fmt.Errorf("wrap: %w", &ValidationError{Details: []FieldError{{Field: "sku", Message: "required"}}})
	if !errors.As(err, &ve) || len(ve.Details) != 1 || ve.Details[0].Field != "sku" {
		t.Fatalf("errors.As ValidationError failed: %v", err)
	}
	if ve.Error() == "" {
		t.Fatal("ValidationError.Error() is empty")
	}

	var ie *InsufficientStockError
	err = fmt.Errorf("wrap: %w", &InsufficientStockError{SKU: "ABC", Available: 3})
	if !errors.As(err, &ie) || ie.SKU != "ABC" || ie.Available != 3 {
		t.Fatalf("errors.As InsufficientStockError failed: %v", err)
	}
	if ie.Error() == "" || (&InsufficientStockError{}).Error() == "" {
		t.Fatal("InsufficientStockError.Error() is empty")
	}
}

func TestSentinelErrors(t *testing.T) {
	for _, s := range []error{ErrNotFound, ErrSKUExists, ErrHasHistory, ErrAlreadyCancelled} {
		if !errors.Is(fmt.Errorf("wrap: %w", s), s) {
			t.Errorf("errors.Is failed for %v", s)
		}
		if s.Error() == "" {
			t.Errorf("sentinel has empty message")
		}
	}
}
