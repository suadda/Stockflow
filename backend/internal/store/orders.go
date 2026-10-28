package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	StatusCreated   = "CREATED"
	StatusCancelled = "CANCELLED"
)

type OrderItemInput struct {
	SKU string      `json:"sku"`
	Qty json.Number `json:"qty"`
}

type OrderInput struct {
	Customer string           `json:"customer"`
	Items    []OrderItemInput `json:"items"`
}

type OrderItem struct {
	SKU       string `json:"sku"`
	Qty       int64  `json:"qty"`
	UnitPrice int64  `json:"unit_price"`
	Subtotal  int64  `json:"subtotal"`
}

type Order struct {
	ID          int64       `json:"id"`
	Customer    string      `json:"customer"`
	Status      string      `json:"status"`
	Total       int64       `json:"total"`
	CreatedAt   time.Time   `json:"created_at"`
	CancelledAt *time.Time  `json:"cancelled_at"`
	Items       []OrderItem `json:"items"`
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type line struct {
	idx int
	sku string
	qty int64
}

func sortLines(ls []line) {
	sort.Slice(ls, func(a, b int) bool { return ls[a].sku < ls[b].sku })
}

func (s *Store) CreateOrder(ctx context.Context, in OrderInput) (Order, error) {
	skus := make([]string, len(in.Items))
	for i, it := range in.Items {
		skus[i] = strings.TrimSpace(it.SKU)
	}

	var out Order
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		catalog, err := loadCatalog(ctx, tx, skus)
		if err != nil {
			return err
		}
		if errs := ValidateOrder(in, catalog); len(errs) > 0 {
			return &ValidationError{Details: errs}
		}

		lines := make([]line, len(in.Items))
		for i, it := range in.Items {
			qty, _ := parseQty(it.Qty)
			lines[i] = line{idx: i, sku: skus[i], qty: qty}
		}
		// Ascending SKU order: every transaction takes product row locks in the
		// same order, so concurrent multi-item orders cannot deadlock.
		sortLines(lines)

		ids := make([]int64, len(lines))
		prices := make([]int64, len(lines))
		var total int64
		for k, l := range lines {
			// Guarded decrement: the stock check and the decrement are one
			// statement, so concurrent orders can never oversell.
			err := tx.QueryRow(ctx,
				`UPDATE products SET stock = stock - $2 WHERE sku = $1 AND stock >= $2 RETURNING id, price`,
				l.sku, l.qty).Scan(&ids[k], &prices[k])
			if errors.Is(err, pgx.ErrNoRows) {
				return shortLine(ctx, tx, l)
			}
			if err != nil {
				return err
			}
			total += prices[k] * l.qty
		}
		if total > maxInt32 {
			return &ValidationError{Details: []FieldError{{Field: "items", Message: "order total exceeds 2147483647"}}}
		}

		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO sales_orders (customer, total) VALUES ($1, $2) RETURNING id`,
			strings.TrimSpace(in.Customer), total).Scan(&id); err != nil {
			return err
		}
		note := fmt.Sprintf("order #%d", id)
		for k, l := range lines {
			if _, err := tx.Exec(ctx,
				`INSERT INTO sales_order_items (order_id, sku, qty, unit_price) VALUES ($1, $2, $3, $4)`,
				id, l.sku, l.qty, prices[k]); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO stock_movements (product_id, type, qty, note) VALUES ($1, 'out', $2, $3)`,
				ids[k], l.qty, note); err != nil {
				return err
			}
		}
		out, err = loadOrder(ctx, tx, id)
		return err
	})
	return out, err
}

func shortLine(ctx context.Context, tx pgx.Tx, l line) error {
	var available int64
	err := tx.QueryRow(ctx, `SELECT stock FROM products WHERE sku = $1`, l.sku).Scan(&available)
	if errors.Is(err, pgx.ErrNoRows) {
		return &ValidationError{Details: []FieldError{{
			Field: fmt.Sprintf("items[%d].sku", l.idx), Message: fmt.Sprintf("unknown sku %q", l.sku),
		}}}
	}
	if err != nil {
		return err
	}
	return &InsufficientStockError{SKU: l.sku, Available: available}
}

func loadCatalog(ctx context.Context, q querier, skus []string) (map[string]CatalogEntry, error) {
	catalog := map[string]CatalogEntry{}
	if len(skus) == 0 {
		return catalog, nil
	}
	rows, err := q.Query(ctx, `SELECT sku, price, is_active FROM products WHERE sku = ANY($1)`, skus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sku string
		var e CatalogEntry
		if err := rows.Scan(&sku, &e.Price, &e.IsActive); err != nil {
			return nil, err
		}
		catalog[sku] = e
	}
	return catalog, rows.Err()
}

func (s *Store) GetOrder(ctx context.Context, id int64) (Order, error) {
	return loadOrder(ctx, s.pool, id)
}

func (s *Store) CancelOrder(ctx context.Context, id int64) (Order, error) {
	var out Order
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// Row lock: a concurrent cancel waits here, then sees CANCELLED, so
		// stock is restored exactly once.
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM sales_orders WHERE id = $1 FOR UPDATE`, id).Scan(&status); err != nil {
			return notFound(err)
		}
		if status == StatusCancelled {
			return ErrAlreadyCancelled
		}

		rows, err := tx.Query(ctx, `SELECT sku, qty FROM sales_order_items WHERE order_id = $1`, id)
		if err != nil {
			return err
		}
		lines, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (line, error) {
			var l line
			return l, r.Scan(&l.sku, &l.qty)
		})
		if err != nil {
			return err
		}
		sortLines(lines)

		note := fmt.Sprintf("order #%d cancelled", id)
		for _, l := range lines {
			var pid int64
			if err := tx.QueryRow(ctx,
				`UPDATE products SET stock = stock + $2 WHERE sku = $1 RETURNING id`, l.sku, l.qty).Scan(&pid); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO stock_movements (product_id, type, qty, note) VALUES ($1, 'in', $2, $3)`,
				pid, l.qty, note); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE sales_orders SET status = 'CANCELLED', cancelled_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
		out, err = loadOrder(ctx, tx, id)
		return err
	})
	return out, err
}

func loadOrder(ctx context.Context, q querier, id int64) (Order, error) {
	var o Order
	err := q.QueryRow(ctx,
		`SELECT id, customer, status, total, created_at, cancelled_at FROM sales_orders WHERE id = $1`, id).
		Scan(&o.ID, &o.Customer, &o.Status, &o.Total, &o.CreatedAt, &o.CancelledAt)
	if err != nil {
		return Order{}, notFound(err)
	}
	o.CreatedAt = o.CreatedAt.UTC()
	if o.CancelledAt != nil {
		t := o.CancelledAt.UTC()
		o.CancelledAt = &t
	}

	rows, err := q.Query(ctx,
		`SELECT sku, qty, unit_price FROM sales_order_items WHERE order_id = $1 ORDER BY id`, id)
	if err != nil {
		return Order{}, err
	}
	o.Items, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (OrderItem, error) {
		var it OrderItem
		err := r.Scan(&it.SKU, &it.Qty, &it.UnitPrice)
		it.Subtotal = it.Qty * it.UnitPrice
		return it, err
	})
	return o, err
}
