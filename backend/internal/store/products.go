package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// LowStockThreshold is the stock level at or below which a product is "low".
const LowStockThreshold = 10

// Product is a catalog row plus its derived last movement time.
type Product struct {
	ID             int64      `json:"id"`
	SKU            string     `json:"sku"`
	Name           string     `json:"name"`
	Price          int64      `json:"price"`
	Stock          int64      `json:"stock"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	LastMovementAt *time.Time `json:"last_movement_at"`
}

// ProductInput is the create request. Price is a json.Number so a
// non-integer becomes a field error instead of a decode error.
type ProductInput struct {
	SKU      string      `json:"sku"`
	Name     string      `json:"name"`
	Price    json.Number `json:"price"`
	IsActive *bool       `json:"is_active"`
}

// ProductUpdate is the update request. SKU is accepted only to reject changes.
type ProductUpdate struct {
	SKU      *string     `json:"sku"`
	Name     string      `json:"name"`
	Price    json.Number `json:"price"`
	IsActive *bool       `json:"is_active"`
}

// ListFilter holds parsed list parameters. Status is "", "active",
// "inactive" or "low".
type ListFilter struct {
	Search      string
	Status      string
	Page, Limit int
}

// productCols selects a product aliased as p, with last_movement_at derived
// from stock_movements (backed by idx_stock_movements_product_created).
const productCols = `p.id, p.sku, p.name, p.price, p.stock, p.is_active, p.created_at,
	(SELECT max(m.created_at) FROM stock_movements m WHERE m.product_id = p.id)`

func scanProduct(row pgx.Row) (Product, error) {
	var p Product
	if err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Price, &p.Stock, &p.IsActive, &p.CreatedAt, &p.LastMovementAt); err != nil {
		return Product{}, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	if p.LastMovementAt != nil {
		t := p.LastMovementAt.UTC()
		p.LastMovementAt = &t
	}
	return p, nil
}

func isPgCode(err error, code string) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == code
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// likeEscape escapes LIKE wildcards so user search text matches literally.
var likeEscape = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace

const listWhere = `
WHERE ($1::text = '' OR p.sku ILIKE $2 OR p.name ILIKE $2)
  AND ($3::text = ''
       OR ($3 = 'active' AND p.is_active)
       OR ($3 = 'inactive' AND NOT p.is_active)
       OR ($3 = 'low' AND p.stock <= 10))`

// ListProducts returns one page of matching products in ascending id order
// and the total number of matches.
func (s *Store) ListProducts(ctx context.Context, f ListFilter) ([]Product, int64, error) {
	pattern := "%" + likeEscape(f.Search) + "%"

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM products p`+listWhere,
		f.Search, pattern, f.Status).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `SELECT `+productCols+` FROM products p`+listWhere+
		` ORDER BY p.id LIMIT $4 OFFSET $5`,
		f.Search, pattern, f.Status, f.Limit, (f.Page-1)*f.Limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Product, 0, f.Limit)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// GetProduct returns one product or ErrNotFound.
func (s *Store) GetProduct(ctx context.Context, id int64) (Product, error) {
	p, err := scanProduct(s.pool.QueryRow(ctx, `SELECT `+productCols+` FROM products p WHERE p.id = $1`, id))
	return p, notFound(err)
}

// CreateProduct inserts a product with stock 0 (active unless is_active is
// false). Duplicate SKUs are caught by the unique constraint, so concurrent
// creates with one SKU yield exactly one row.
func (s *Store) CreateProduct(ctx context.Context, in ProductInput) (Product, error) {
	if errs := ValidateProductInput(in); len(errs) > 0 {
		return Product{}, &ValidationError{Details: errs}
	}
	price, _ := parsePrice(in.Price)
	active := in.IsActive == nil || *in.IsActive

	p, err := scanProduct(s.pool.QueryRow(ctx,
		`INSERT INTO products AS p (sku, name, price, is_active) VALUES ($1, $2, $3, $4) RETURNING `+productCols,
		strings.TrimSpace(in.SKU), strings.TrimSpace(in.Name), price, active))
	if isPgCode(err, "23505") {
		return Product{}, ErrSKUExists
	}
	return p, err
}

// UpdateProduct changes name, price and is_active. Stock and SKU never change here.
func (s *Store) UpdateProduct(ctx context.Context, id int64, in ProductUpdate) (Product, error) {
	var out Product
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var sku string
		if err := tx.QueryRow(ctx, `SELECT sku FROM products WHERE id = $1 FOR UPDATE`, id).Scan(&sku); err != nil {
			return notFound(err)
		}
		if errs := ValidateProductUpdate(in, sku); len(errs) > 0 {
			return &ValidationError{Details: errs}
		}
		price, _ := parsePrice(in.Price)
		var err error
		out, err = scanProduct(tx.QueryRow(ctx,
			`UPDATE products AS p SET name = $2, price = $3, is_active = $4 WHERE p.id = $1 RETURNING `+productCols,
			id, strings.TrimSpace(in.Name), price, *in.IsActive))
		return err
	})
	return out, err
}

// DeleteProduct removes a product that has no stock movements and no order
// items. History is never deleted: such products return ErrHasHistory.
func (s *Store) DeleteProduct(ctx context.Context, id int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var sku string
		if err := tx.QueryRow(ctx, `SELECT sku FROM products WHERE id = $1 FOR UPDATE`, id).Scan(&sku); err != nil {
			return notFound(err)
		}
		var has bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM stock_movements WHERE product_id = $1)
			     OR EXISTS (SELECT 1 FROM sales_order_items WHERE sku = $2)`, id, sku).Scan(&has); err != nil {
			return err
		}
		if has {
			return ErrHasHistory
		}
		if _, err := tx.Exec(ctx, `DELETE FROM products WHERE id = $1`, id); err != nil {
			if isPgCode(err, "23503") {
				return ErrHasHistory
			}
			return err
		}
		return nil
	})
}
