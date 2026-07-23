package domain

import "time"

// TripSheet represents the full extracted data from a scanned trip sheet image.
type TripSheet struct {
	OdometerOpen   *int       `json:"odometer_open" validate:"required"`
	OdometerClose  *int       `json:"odometer_close" validate:"required"`
	TotalMiles     *int       `json:"total_miles"`
	LineItems      []LineItem `json:"line_items" validate:"required,dive"`
	ConfidenceScore float64   `json:"confidence_score" validate:"required,gte=0,lte=1"`
	FlaggedFields  []string   `json:"flagged_fields"`
}

// LineItem represents a single leg of a trip.
type LineItem struct {
	Date     *string `json:"date"`
	Location *string `json:"location"`
	Miles    *int    `json:"miles"`
}

// ValidationResult holds the outcome of deterministic cross-checks.
type ValidationResult struct {
	OdometerDeltaCheck string   `json:"odometer_delta_check"` // "pass" or "fail"
	LineItemSumCheck   string   `json:"line_item_sum_check"`  // "pass" or "fail"
	ConfidenceCheck    string   `json:"confidence_check"`     // "pass" or "fail"
	Errors             []string `json:"errors"`
}

// ExtractionResponse is the final API response returned to the caller.
type ExtractionResponse struct {
	Status     string            `json:"status"` // "validated" or "exception"
	TripSheet  *TripSheet        `json:"trip_sheet"`
	Validation *ValidationResult `json:"validation"`
	ProcessedAt time.Time        `json:"processed_at"`
}
