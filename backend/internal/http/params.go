package httpapi

import (
	"net/url"
	"strconv"
	"strings"

	"stockflow/internal/store"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

func parseListParams(q url.Values) (store.ListFilter, []store.FieldError) {
	f := store.ListFilter{
		Search: strings.TrimSpace(q.Get("search")),
		Status: q.Get("status"),
		Page:   1,
		Limit:  defaultLimit,
	}
	var errs []store.FieldError
	for _, p := range []struct {
		name string
		dst  *int
	}{{"page", &f.Page}, {"limit", &f.Limit}} {
		v := q.Get(p.name)
		if v == "" {
			continue
		}
		n, ok := parsePositive(v)
		if !ok {
			errs = append(errs, store.FieldError{Field: p.name, Message: p.name + " must be a positive integer"})
			continue
		}
		*p.dst = n
	}
	if f.Limit > maxLimit {
		f.Limit = maxLimit
	}
	switch f.Status {
	case "", "active", "inactive", "low":
	default:
		errs = append(errs, store.FieldError{Field: "status", Message: "status must be one of active, inactive, low"})
	}
	return f, errs
}

func parsePositive(s string) (int, bool) {
	if len(s) == 0 || len(s) > 9 || s[0] == '0' {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}
