package store

import "context"

type Summary struct {
	TotalProducts   int64 `json:"total_products"`
	ActiveProducts  int64 `json:"active_products"`
	TotalStockValue int64 `json:"total_stock_value"`
	LowStockCount   int64 `json:"low_stock_count"`
}

func (s *Store) Summary(ctx context.Context) (Summary, error) {
	return summarize(ctx, s.pool)
}

func summarize(ctx context.Context, q querier) (Summary, error) {
	var sm Summary
	err := q.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE is_active),
		       COALESCE(sum(price::bigint * stock::bigint), 0)::bigint,
		       count(*) FILTER (WHERE stock <= $1)
		FROM products`, LowStockThreshold).
		Scan(&sm.TotalProducts, &sm.ActiveProducts, &sm.TotalStockValue, &sm.LowStockCount)
	return sm, err
}
