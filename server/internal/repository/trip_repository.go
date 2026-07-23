package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trucking-poc/server/internal/domain"
)

// TripRepository handles persistence of trip sheet data to Postgres.
type TripRepository struct {
	pool *pgxpool.Pool
}

// NewTripRepository creates a new repository backed by the given connection pool.
func NewTripRepository(pool *pgxpool.Pool) *TripRepository {
	return &TripRepository{pool: pool}
}

// SaveTrip persists a TripRecord and its LineItems in a single atomic transaction.
func (r *TripRepository) SaveTrip(ctx context.Context, record *domain.TripRecord) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op if committed

	// ---- Insert parent trip record ----
	err = tx.QueryRow(ctx, `
		INSERT INTO trips (odometer_open, odometer_close, total_miles, confidence_score,
		                   flagged_fields, status, validation_errors, image_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`,
		record.OdometerOpen,
		record.OdometerClose,
		record.TotalMiles,
		record.ConfidenceScore,
		record.FlaggedFields,
		record.Status,
		record.ValidationErrors,
		record.ImagePath,
	).Scan(&record.ID, &record.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert trip: %w", err)
	}

	// ---- Bulk insert line items using CopyFrom ----
	if len(record.LineItems) > 0 {
		rows := make([][]interface{}, len(record.LineItems))
		for i, item := range record.LineItems {
			rows[i] = []interface{}{
				record.ID,
				item.Date,
				item.Location,
				item.Miles,
				item.SortOrder,
			}
		}

		copyCount, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{"trip_line_items"},
			[]string{"trip_id", "date", "location", "miles", "sort_order"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			return fmt.Errorf("failed to bulk insert line items: %w", err)
		}

		if int(copyCount) != len(record.LineItems) {
			return fmt.Errorf("expected to insert %d line items, but inserted %d",
				len(record.LineItems), copyCount)
		}
	}

	// ---- Commit transaction ----
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// UpdateImagePath updates the image_path column for a trip.
// This is used because the image is saved to disk using the generated trip UUID
// after the main trip record is inserted.
func (r *TripRepository) UpdateImagePath(ctx context.Context, id string, path string) error {
	_, err := r.pool.Exec(ctx, "UPDATE trips SET image_path = $1 WHERE id = $2", path, id)
	if err != nil {
		return fmt.Errorf("failed to update image path: %w", err)
	}
	return nil
}

// GetTripByID retrieves a trip and its line items by ID.
func (r *TripRepository) GetTripByID(ctx context.Context, id string) (*domain.TripRecord, error) {
	record := &domain.TripRecord{}

	err := r.pool.QueryRow(ctx, `
		SELECT id, odometer_open, odometer_close, total_miles, confidence_score,
		       flagged_fields, status, validation_errors, image_path, created_at
		FROM trips WHERE id = $1
	`, id).Scan(
		&record.ID,
		&record.OdometerOpen,
		&record.OdometerClose,
		&record.TotalMiles,
		&record.ConfidenceScore,
		&record.FlaggedFields,
		&record.Status,
		&record.ValidationErrors,
		&record.ImagePath,
		&record.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get trip: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, trip_id, date, location, miles, sort_order
		FROM trip_line_items WHERE trip_id = $1 ORDER BY sort_order
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get line items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.LineItemRecord
		if err := rows.Scan(&item.ID, &item.TripID, &item.Date, &item.Location, &item.Miles, &item.SortOrder); err != nil {
			return nil, fmt.Errorf("failed to scan line item: %w", err)
		}
		record.LineItems = append(record.LineItems, item)
	}

	return record, nil
}

// ListTrips returns all trips ordered by creation date (newest first).
func (r *TripRepository) ListTrips(ctx context.Context) ([]domain.TripRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, odometer_open, odometer_close, total_miles, confidence_score,
		       flagged_fields, status, validation_errors, image_path, created_at
		FROM trips ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list trips: %w", err)
	}
	defer rows.Close()

	var trips []domain.TripRecord
	for rows.Next() {
		var t domain.TripRecord
		if err := rows.Scan(
			&t.ID, &t.OdometerOpen, &t.OdometerClose, &t.TotalMiles,
			&t.ConfidenceScore, &t.FlaggedFields, &t.Status,
			&t.ValidationErrors, &t.ImagePath, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan trip: %w", err)
		}
		trips = append(trips, t)
	}

	return trips, nil
}

// ListValidatedTrips returns only trips with status "validated", including their line items.
// Used by the export endpoints to ensure only human-approved or auto-validated data is exported.
func (r *TripRepository) ListValidatedTrips(ctx context.Context) ([]domain.TripRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, odometer_open, odometer_close, total_miles, confidence_score,
		       flagged_fields, status, validation_errors, image_path, created_at
		FROM trips WHERE status = 'validated' ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list validated trips: %w", err)
	}
	defer rows.Close()

	var trips []domain.TripRecord
	for rows.Next() {
		var t domain.TripRecord
		if err := rows.Scan(
			&t.ID, &t.OdometerOpen, &t.OdometerClose, &t.TotalMiles,
			&t.ConfidenceScore, &t.FlaggedFields, &t.Status,
			&t.ValidationErrors, &t.ImagePath, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan trip: %w", err)
		}

		// Fetch line items for each trip
		liRows, err := r.pool.Query(ctx, `
			SELECT id, trip_id, date, location, miles, sort_order
			FROM trip_line_items WHERE trip_id = $1 ORDER BY sort_order
		`, t.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get line items for trip %s: %w", t.ID, err)
		}

		for liRows.Next() {
			var li domain.LineItemRecord
			if err := liRows.Scan(&li.ID, &li.TripID, &li.Date, &li.Location, &li.Miles, &li.SortOrder); err != nil {
				liRows.Close()
				return nil, fmt.Errorf("failed to scan line item: %w", err)
			}
			t.LineItems = append(t.LineItems, li)
		}
		liRows.Close()

		trips = append(trips, t)
	}

	return trips, nil
}
