package domain

import "time"

// TripRecord is the persistence-layer representation of a trip,
// including all fields needed to INSERT into the database.
type TripRecord struct {
	ID               string
	OdometerOpen     *int
	OdometerClose    *int
	TotalMiles       *int
	ConfidenceScore  float64
	FlaggedFields    []string
	Status           string
	ValidationErrors []string
	ImagePath        string
	LineItems        []LineItemRecord
	CreatedAt        time.Time
}

// LineItemRecord is the persistence-layer representation of a trip line item.
type LineItemRecord struct {
	ID        string
	TripID    string
	Date      *string
	Location  *string
	Miles     *int
	SortOrder int
}
