package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	MovementIn  = "in"
	MovementOut = "out"
)

type StockInput struct {
	Type string      `json:"type"`
	Qty  json.Number `json:"qty"`
	Note string      `json:"note"`
}

type Movement struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"product_id"`
	Type      string    `json:"type"`
	Qty       int64     `json:"qty"`
	Note      *string   `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) AdjustStock(ctx context.Context, id int64, in StockInput) (Product, error) {
	if errs := ValidateStockInput(in); len(errs) > 0 {
		return Product{}, &ValidationError{Details: errs}
	}
	qty, _ := parseQty(in.Qty)
	var note *string
	if n := strings.TrimSpace(in.Note); n != "" {
		note = &n
	}

	var out Product
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		sql := `UPDATE products SET stock = stock + $2 WHERE id = $1`
		if in.Type == MovementOut {
			sql = `UPDATE products SET stock = stock - $2 WHERE id = $1 AND stock >= $2`
		}
		tag, err := tx.Exec(ctx, sql, id, qty)
		if isPgCode(err, "22003") {
			return &ValidationError{Details: []FieldError{{Field: "qty", Message: "resulting stock would exceed 2147483647"}}}
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {

			var available int64
			if err := tx.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1`, id).Scan(&available); err != nil {
				return notFound(err)
			}
			return &InsufficientStockError{Available: available}
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO stock_movements (product_id, type, qty, note) VALUES ($1, $2, $3, $4)`,
			id, in.Type, qty, note); err != nil {
			return err
		}
		out, err = scanProduct(tx.QueryRow(ctx, `SELECT `+productCols+` FROM products p WHERE p.id = $1`, id))
		return err
	})
	return out, err
}

func (s *Store) ListMovements(ctx context.Context, productID int64) ([]Movement, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1)`, productID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, product_id, type, qty, note, created_at FROM stock_movements
		 WHERE product_id = $1 ORDER BY created_at DESC, id DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Movement{}
	for rows.Next() {
		var m Movement
		if err := rows.Scan(&m.ID, &m.ProductID, &m.Type, &m.Qty, &m.Note, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.CreatedAt = m.CreatedAt.UTC()
		out = append(out, m)
	}
	return out, rows.Err()
}
