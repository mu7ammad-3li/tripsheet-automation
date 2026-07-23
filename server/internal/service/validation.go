package service

import (
	"fmt"
	"math"

	"github.com/trucking-poc/server/internal/domain"
)

const (
	// ConfidenceThreshold is the minimum confidence score to auto-validate.
	ConfidenceThreshold = 0.85
	// TolerancePercent is the acceptable deviation for arithmetic checks.
	TolerancePercent = 0.05
)

// ValidateTripSheet runs deterministic business-rule cross-checks on the
// extracted trip sheet data. It returns a ValidationResult and whether the
// overall status is "validated" or "exception".
func ValidateTripSheet(ts *domain.TripSheet) (string, *domain.ValidationResult) {
	result := &domain.ValidationResult{
		OdometerDeltaCheck: "pass",
		LineItemSumCheck:   "pass",
		ConfidenceCheck:    "pass",
		Errors:             []string{},
	}

	status := "validated"

	// ---- Check 1: Required fields (odometer values must be non-null) ----
	if ts.OdometerOpen == nil || ts.OdometerClose == nil {
		status = "exception"
		result.Errors = append(result.Errors, "Odometer open and/or close values are missing")
	}

	// ---- Check 2: Odometer sanity (close > open) ----
	if ts.OdometerOpen != nil && ts.OdometerClose != nil {
		if *ts.OdometerClose <= *ts.OdometerOpen {
			status = "exception"
			result.OdometerDeltaCheck = "fail"
			result.Errors = append(result.Errors,
				fmt.Sprintf("Odometer close (%d) is not greater than odometer open (%d)",
					*ts.OdometerClose, *ts.OdometerOpen))
		}
	}

	// ---- Check 3: Odometer delta ≈ total_miles (±5%) ----
	if ts.OdometerOpen != nil && ts.OdometerClose != nil && ts.TotalMiles != nil {
		delta := *ts.OdometerClose - *ts.OdometerOpen
		totalMiles := *ts.TotalMiles

		if totalMiles > 0 {
			deviation := math.Abs(float64(delta-totalMiles)) / float64(totalMiles)
			if deviation > TolerancePercent {
				status = "exception"
				result.OdometerDeltaCheck = "fail"
				result.Errors = append(result.Errors,
					fmt.Sprintf("Odometer delta (%d) does not match total_miles (%d) — deviation %.1f%% exceeds %.0f%% tolerance",
						delta, totalMiles, deviation*100, TolerancePercent*100))
			}
		}
	}

	// ---- Check 4: Sum of line-item miles ≈ total_miles (±5%) ----
	if ts.TotalMiles != nil && len(ts.LineItems) > 0 {
		sumMiles := 0
		for _, item := range ts.LineItems {
			if item.Miles != nil {
				sumMiles += *item.Miles
			}
		}

		totalMiles := *ts.TotalMiles
		if totalMiles > 0 {
			deviation := math.Abs(float64(sumMiles-totalMiles)) / float64(totalMiles)
			if deviation > TolerancePercent {
				status = "exception"
				result.LineItemSumCheck = "fail"
				result.Errors = append(result.Errors,
					fmt.Sprintf("Sum of line-item miles (%d) does not match total_miles (%d) — deviation %.1f%% exceeds %.0f%% tolerance",
						sumMiles, totalMiles, deviation*100, TolerancePercent*100))
			}
		}
	}

	// ---- Check 5: Confidence threshold ----
	if ts.ConfidenceScore < ConfidenceThreshold {
		status = "exception"
		result.ConfidenceCheck = "fail"
		result.Errors = append(result.Errors,
			fmt.Sprintf("Confidence score (%.2f) is below threshold (%.2f)",
				ts.ConfidenceScore, ConfidenceThreshold))
	}

	return status, result
}
