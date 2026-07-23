# Phase 3: Persistence (Postgres) — Planning

## Context

> The audit trail is non-negotiable. A human must always be able to view the original scanned image alongside the extracted data, especially for trips routed to the exception queue.

With the data extracted and validated, it moves to the persistence layer. Every trip — whether validated or exception — is stored alongside the raw image for auditability.

## Objective

Persist extracted trip sheet data to Postgres using atomic transactions, and store the raw scanned image to a local filesystem (swappable to S3/GCS in production) for audit purposes.

---

## Database Schema

### `trips` table
| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | Primary key, auto-generated |
| `odometer_open` | INTEGER | Nullable |
| `odometer_close` | INTEGER | Nullable |
| `total_miles` | INTEGER | Nullable |
| `confidence_score` | FLOAT | VLM confidence |
| `flagged_fields` | TEXT[] | Postgres array of flagged field names |
| `status` | VARCHAR(20) | `validated` or `exception` |
| `validation_errors` | TEXT[] | Array of error messages from guardrails |
| `image_path` | TEXT | Path to the stored audit image |
| `created_at` | TIMESTAMPTZ | Auto-set on insert |

### `trip_line_items` table
| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | Primary key |
| `trip_id` | UUID | FK → `trips.id` (CASCADE delete) |
| `date` | VARCHAR(20) | Date as written on sheet |
| `location` | TEXT | Route segment |
| `miles` | INTEGER | Nullable |
| `sort_order` | INTEGER | Preserves row order from the original sheet |

### Indexes
- `idx_trip_line_items_trip_id` — FK lookup performance
- `idx_trips_status` — filter by validated/exception
- `idx_trips_created_at` — chronological queries

## Engineering Decisions

### 1. Driver: `pgx/v5`
- Pure Go, no CGO dependency
- Native support for Postgres arrays (`TEXT[]`), UUIDs, connection pooling
- `pgxpool.Pool` for connection pool management

### 2. Atomic Transactions
Single atomic transaction per trip:
1. `INSERT INTO trips` → get back the generated UUID
2. `CopyFrom` bulk insert all `trip_line_items` using the Postgres COPY protocol
3. `COMMIT` — or full `ROLLBACK` on any failure

No partial data is ever persisted.

### 3. Bulk Insert via COPY Protocol
Line items are inserted using `pgx.CopyFrom`, which uses the Postgres binary COPY protocol. This is significantly faster than individual `INSERT` statements and scales well for trips with many legs.

### 4. Audit Image Storage
- **POC**: Local filesystem at `./audit_images/`, keyed by trip UUID (e.g., `audit_images/abc-123.jpg`)
- **Production**: Swap to S3/GCS by implementing the same interface — the DB always stores just the path string
- The raw image is preserved exactly as uploaded, before any preprocessing

### 5. Migrations
SQL-based migration files in `server/migrations/`. Both `up` and `down` scripts provided for reproducibility.

```bash
# Apply migration
psql -d trucking -f server/migrations/001_create_trips.up.sql

# Rollback
psql -d trucking -f server/migrations/001_create_trips.down.sql
```

## Files

- [`server/migrations/001_create_trips.up.sql`](../server/migrations/001_create_trips.up.sql) — Schema creation
- [`server/migrations/001_create_trips.down.sql`](../server/migrations/001_create_trips.down.sql) — Schema rollback
- [`server/internal/domain/trip_record.go`](../server/internal/domain/trip_record.go) — Persistence structs
- [`server/internal/repository/trip_repository.go`](../server/internal/repository/trip_repository.go) — Repository layer
- [`server/internal/storage/audit.go`](../server/internal/storage/audit.go) — Audit image store
