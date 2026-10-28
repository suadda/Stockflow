# StockFlow

Mini ERP module for a warehouse supervisor: **manage products, adjust stock, and record sales orders that consume stock.**

```
PostgreSQL  →  Go API (backend/)  →  React + TypeScript UI (frontend/)
```

The UI calls the real API; there is no mock data.

## Contents

- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Project structure](#project-structure)
- [Seed data](#seed-data)
- [API](#api)
- [Tests](#tests)
- [Design decisions](#design-decisions)
- [Known limitations and next steps](#known-limitations-and-next-steps)
- [Hours spent](#hours-spent)

## Quick start

**Prerequisites:** Docker, Go 1.25+, Node 20+.

**1. Clone and start the database** (Postgres 16, migrations and seed applied automatically)

```bash
git clone https://github.com/suadda/Stockflow.git
cd Stockflow
docker compose up -d --wait
```

**2. Start the API** (terminal 1)

```bash
cd backend
go run ./cmd/api
```

API runs on http://localhost:8080. Check it with `curl http://localhost:8080/healthz` → `{"status":"ok"}`.

**3. Start the UI** (terminal 2)

```bash
cd frontend
npm install
npm run dev
```

Open **http://localhost:5173**.

> Vite forwards `/api/*` to `localhost:8080`, so the browser stays same-origin and the API needs no CORS code.

### Reset the database

Migrations only run on an empty volume. To go back to the seed data:

```bash
docker compose down -v && docker compose up -d --wait
```

## Configuration

The defaults work out of the box with `docker compose`. Override them with environment variables (see [`backend/.env.example`](backend/.env.example)); the API does not load `.env` files by itself.

| Variable | Default | Purpose |
|---|---|---|
| `ADDR` | `:8080` | API listen address |
| `DATABASE_URL` | `postgres://stockflow:stockflow@localhost:5432/stockflow?sslmode=disable` | Postgres connection |

Postgres runs on `localhost:5432` with database, user and password all set to `stockflow` (local dev only).

## Project structure

```
stockflow/
├── docker-compose.yml          Postgres 16, mounts backend/migrations as init scripts
├── backend/
│   ├── cmd/api/                Entrypoint
│   ├── internal/config/        Env-based config
│   ├── internal/http/          Handlers, routing, middleware, error envelope
│   ├── internal/store/         All SQL and transactions (products, stock, orders, summary)
│   └── migrations/             001_init.sql (schema), 002_seed.sql (demo data + integrity check)
└── frontend/
    └── src/
        ├── api/                Typed API client and types
        ├── components/         Modals, summary cards, badges
        ├── pages/              Products, order form, order detail
        └── lib/                Hash router, validators, formatters, hooks
```

## Seed data

- **14 products** (electronics and office supplies, integer IDR prices)
- **5 low-stock** products (stock ≤ 10) and **1 inactive** product (`ADPT-VGA`)
- **2 orders**

Every unit of stock comes from a stock movement. A check block at the end of `002_seed.sql` fails the database init if the ledger or an order total is inconsistent.

## API

### Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check (pings the database) |
| `GET` | `/api/products` | List products. Query: `search` (SKU or name), `status` (`active`, `inactive`, `low`), `page`, `limit` |
| `POST` | `/api/products` | Create a product (starts with stock 0) |
| `PUT` | `/api/products/{id}` | Update name, price, active flag (SKU is immutable) |
| `DELETE` | `/api/products/{id}` | Delete a product that has no history |
| `POST` | `/api/products/{id}/stock` | Adjust stock with an `in` or `out` movement |
| `GET` | `/api/products/{id}/movements` | Movement history, newest first |
| `POST` | `/api/orders` | Create an order (server-side pricing, all-or-nothing) |
| `GET` | `/api/orders/{id}` | Order detail |
| `POST` | `/api/orders/{id}/cancel` | Cancel an order and restore stock |
| `GET` | `/api/summary` | Totals for the dashboard cards |

The product list is paginated: `{"data": [...], "page": 1, "limit": 5, "total": 14}`.

### Error format

Every error uses one envelope:

```json
{"error":{"code":"INSUFFICIENT_STOCK","message":"insufficient stock for USB-C-1M: only 112 available","sku":"USB-C-1M","available":112}}
```

| Code | Status | When |
|---|---|---|
| `VALIDATION_ERROR` | 400 | Field rules fail; `details: [{field, message}]` lists every failure |
| `INVALID_JSON` | 400 | Malformed JSON, unknown field (including a client `unit_price`), trailing data, body over 1 MiB |
| `NOT_FOUND` | 404 | Missing product/order, bad id, unknown route |
| `METHOD_NOT_ALLOWED` | 405 | Known path, wrong method (with `Allow` header) |
| `SKU_EXISTS` | 409 | Duplicate SKU on create |
| `PRODUCT_HAS_HISTORY` | 409 | Delete of a product with movements or order items |
| `INSUFFICIENT_STOCK` | 409 | Overdraw; includes `available`, plus `sku` for orders |
| `ALREADY_CANCELLED` | 409 | Cancelling a cancelled order |
| `INTERNAL` | 500 | Unexpected error; details only in the server log |

### Try it with curl

<details>
<summary>Bash examples (click to expand)</summary>

```bash
API=http://localhost:8080

# --- Products ---------------------------------------------------------------
curl -s "$API/api/products?search=usb&status=active&page=1&limit=5"

curl -s -X POST $API/api/products -H 'Content-Type: application/json' \
  -d '{"sku":"DEMO-001","name":"Demo Cable","price":25000}'     # 201, stock 0
curl -s -X POST $API/api/products -H 'Content-Type: application/json' \
  -d '{"sku":"DEMO-001","name":"Again","price":1}'              # 409 SKU_EXISTS
curl -s -X POST $API/api/products -H 'Content-Type: application/json' \
  -d '{"sku":" ","name":"","price":1.5}'                        # 400 VALIDATION_ERROR, 3 details

curl -s -X PUT $API/api/products/1 -H 'Content-Type: application/json' \
  -d '{"name":"Kabel USB-C ke USB-C 1m","price":45000,"is_active":true}'   # 200
curl -s -X PUT $API/api/products/999999 -H 'Content-Type: application/json' \
  -d '{"name":"x","price":1,"is_active":true}'                  # 404 NOT_FOUND

curl -s -X DELETE $API/api/products/1                           # 409 PRODUCT_HAS_HISTORY

# --- Stock ------------------------------------------------------------------
curl -s -X POST $API/api/products/1/stock -H 'Content-Type: application/json' \
  -d '{"type":"in","qty":5,"note":"receipt"}'                   # 200
curl -s -X POST $API/api/products/1/stock -H 'Content-Type: application/json' \
  -d '{"type":"out","qty":999999}'                              # 409 INSUFFICIENT_STOCK + available
curl -s $API/api/products/1/movements                           # {"data":[...]} newest first

# --- Orders (use the id from the 201 response in place of 3) ----------------
curl -s -X POST $API/api/orders -H 'Content-Type: application/json' \
  -d '{"customer":"Toko Maju","items":[{"sku":"USB-C-1M","qty":2},{"sku":"HDMI-2M","qty":1}]}'    # 201
curl -s -X POST $API/api/orders -H 'Content-Type: application/json' \
  -d '{"customer":"Toko Maju","items":[{"sku":"USB-C-1M","qty":1},{"sku":"MON-24-IPS","qty":50}]}' # 409, sku MON-24-IPS
curl -s -X POST $API/api/orders -H 'Content-Type: application/json' \
  -d '{"customer":"","items":[{"sku":"NOPE","qty":0}]}'         # 400, customer + items[0].sku + items[0].qty
curl -s $API/api/orders/3
curl -s -X POST $API/api/orders/3/cancel                        # 200, stock restored
curl -s -X POST $API/api/orders/3/cancel                        # 409 ALREADY_CANCELLED

# --- Summary ----------------------------------------------------------------
curl -s $API/api/summary   # {"total_products":..,"active_products":..,"total_stock_value":..,"low_stock_count":..}
```

</details>

## Tests

### Run them

```bash
# Backend (DB tests use the compose database and are SKIPPED when DATABASE_URL is unset)
cd backend
export DATABASE_URL="postgres://stockflow:stockflow@localhost:5432/stockflow?sslmode=disable"
go vet ./...
go test ./...
go test -race ./...

# Frontend
cd ../frontend
npm run test -- --run
npm run build        # strict type check + production build
```

> **Check that nothing was skipped:** run `go test -v ./... | grep SKIP`. No output means every DB test ran.

<details>
<summary>Windows / PowerShell notes</summary>

- Set the variable with `$env:DATABASE_URL="postgres://stockflow:stockflow@localhost:5432/stockflow?sslmode=disable"`.
- `npm run dev -- --port 5173` loses the `--` in PowerShell; plain `npm run dev` already uses 5173.
- `-race` needs cgo and gcc. Without gcc, run it in a container (Docker Desktop):

```powershell
cd backend
docker run --rm -v "${PWD}:/src" -w /src `
  -e DATABASE_URL="postgres://stockflow:stockflow@host.docker.internal:5432/stockflow?sslmode=disable" `
  golang:1.25 go test -race ./...
```

- For curl, use `curl.exe` and pipe the body:
  `'{"type":"in","qty":5}' | curl.exe -X POST ... -H "Content-Type: application/json" --data-binary "@-"`

</details>

### What is covered

| Test | Where |
|---|---|
| (a) An `out` larger than stock returns 409 and writes nothing | `internal/store/stock_test.go`, `internal/http/stock_http_test.go` |
| (b) A valid adjustment updates stock and writes exactly one movement | same files |
| (c) An order with one short item changes no stock, order, item or movement | `internal/store/orders_test.go`, `internal/http/orders_http_test.go` |
| (d) Concurrency (`-race`): 50 orders vs stock 10 accept exactly 10; reversed multi-item orders never deadlock; concurrent `out`s never go negative; concurrent cancels restore stock once | `internal/store/concurrency_test.go` |
| Error envelope, invalid JSON, panic → 500, error mapping, id parsing | `internal/http/respond_test.go`, `server_test.go` |
| Products CRUD, filters, paging, duplicate-SKU race | `internal/http/products_http_test.go` |
| Summary vs recount in one snapshot; seed integrity | `internal/store/summary_test.go`, `seed_test.go` |
| ApiError parsing, formatters, validators (Vitest) | `frontend/src/**/*.test.ts` |

## Design decisions

**Stock safety**

- **Stock is decremented only by a guarded update:** `UPDATE products SET stock = stock - $q WHERE … AND stock >= $q`. The row lock plus the re-check under READ COMMITTED means concurrent orders queue on the row and each sees the committed stock. Stock cannot go negative, and no request ever does SELECT-then-UPDATE. `CHECK (stock >= 0)` is the second line of defence. When the guard matches no row, a follow-up read only *explains* why (404 vs 409 with `available`); it never decides.
- **The ledger always balances.** Create writes `out` movements, cancel writes `in` movements, so `stock = Σin − Σout` holds.

**Orders**

- **One transaction per order**, validated up front (all errors reported together). Items are processed in ascending SKU order, so every transaction takes product locks in the same order and two multi-item orders cannot deadlock.
- **Server-side pricing.** `unit_price` is read from the row the guarded update just locked; the client schema has no price field at all.
- **Cancel locks the order row** with `SELECT … FOR UPDATE`, so concurrent cancels serialize and stock is restored exactly once.

**Data rules**

- **SKU is immutable** after creation (order items reference it). New products start at stock 0. Inactive products can be adjusted but not ordered.
- **History is never deleted.** A product with movements or order items returns 409 `PRODUCT_HAS_HISTORY`; mark it inactive instead.
- **`last_movement_at` is derived** with a `max(created_at)` subquery, backed by an index on `(product_id, created_at)`. One source of truth, nothing to keep in sync.

**API and UI**

- **One error envelope**, decoded once in `frontend/src/api/client.ts` into a typed `ApiError`. Screens switch on `code`: the adjust modal shows "Only N in stock", the order form highlights the failing SKU row, and validation messages land next to their fields.
- **Additive endpoints** beyond the brief: the `status` filter on the product list and `GET /api/summary` for the cards.
- **Stdlib over frameworks:** `net/http` with Go 1.22 method patterns, `pgx/v5`, and plain SQL migrations mounted into Postgres' init directory. The frontend uses a tiny hash router and plain CSS.

## Known limitations and next steps

**Known limitations**

- No authentication: the API is meant for local use only.
- No order list endpoint or page; orders are reachable by id only.
- The order form loads the first 100 active products instead of a typeahead search.
- Migrations only run on an empty volume (init scripts, no versioning).

**Next steps**

- `Idempotency-Key` on `POST /api/orders` so a retried submit never double-orders.
- Run the API and UI in compose too (`docker compose up --build`).
- Order list page and a typeahead SKU picker.
- Versioned migrations with a tool (goose/atlas).
- Daily sales report endpoint and card.
- Browser-level tests (Playwright) for the 409 flows, and component tests for the modals.

## Commit history

This history is not a clean build log. The backend foundation files (module, config, store foundation, middleware, entrypoint and the seed migration) were committed after the code that depends on them, and the comment cleanup was committed as separate commits. I did not rewrite the history to make it look tidier. Early commits may not build on their own. The latest state was checked with `go vet`, `go test` and `npm run build`.

## Hours spent
- 10 Hours