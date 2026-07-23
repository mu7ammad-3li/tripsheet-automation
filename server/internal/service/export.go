package service

import (
	"strings"
	"time"

	"github.com/trucking-poc/server/internal/domain"
)

// Default rate per mile for payroll calculation (POC hardcoded value).
const DefaultRatePerMile = 0.55

// ---------------------------------------------------------------------------
// TMS Export (Dispatch)
// ---------------------------------------------------------------------------

// TMSExport is the top-level TMS dispatch export payload.
type TMSExport struct {
	ExportType string         `json:"export_type"`
	ExportedAt time.Time      `json:"exported_at"`
	Trips      []TMSTripEntry `json:"trips"`
}

// TMSTripEntry represents a single trip in the TMS export.
type TMSTripEntry struct {
	TripID        string            `json:"trip_id"`
	TotalMiles    *int              `json:"total_miles"`
	RouteSegments []TMSRouteSegment `json:"route_segments"`
	Odometer      TMSOdometer       `json:"odometer"`
}

// TMSRouteSegment represents a single leg of the route for TMS dispatch.
type TMSRouteSegment struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Miles       *int   `json:"miles"`
	Date        string `json:"date"`
}

// TMSOdometer holds the start and end odometer readings.
type TMSOdometer struct {
	Start *int `json:"start"`
	End   *int `json:"end"`
}

// BuildTMSExport transforms validated trip records into a TMS dispatch payload.
func BuildTMSExport(trips []domain.TripRecord) *TMSExport {
	export := &TMSExport{
		ExportType: "tms_dispatch",
		ExportedAt: time.Now(),
		Trips:      make([]TMSTripEntry, 0, len(trips)),
	}

	for _, t := range trips {
		entry := TMSTripEntry{
			TripID:     t.ID,
			TotalMiles: t.TotalMiles,
			Odometer:   TMSOdometer{Start: t.OdometerOpen, End: t.OdometerClose},
		}

		for _, li := range t.LineItems {
			seg := TMSRouteSegment{
				Miles: li.Miles,
			}

			if li.Date != nil {
				seg.Date = *li.Date
			}

			// Parse "City A to City B" or "City A -> City B" into origin/destination
			if li.Location != nil {
				origin, dest := splitRoute(*li.Location)
				seg.Origin = origin
				seg.Destination = dest
			}

			entry.RouteSegments = append(entry.RouteSegments, seg)
		}

		export.Trips = append(export.Trips, entry)
	}

	return export
}

// ---------------------------------------------------------------------------
// Accounting Export (Payroll)
// ---------------------------------------------------------------------------

// AccountingExport is the top-level accounting/payroll export payload.
type AccountingExport struct {
	ExportType string              `json:"export_type"`
	ExportedAt time.Time           `json:"exported_at"`
	PayItems   []AccountingPayItem `json:"pay_items"`
}

// AccountingPayItem represents a single trip's payroll line item.
type AccountingPayItem struct {
	TripID       string        `json:"trip_id"`
	DateRange    DateRange     `json:"date_range"`
	TotalMiles   int           `json:"total_miles"`
	BillableMiles int          `json:"billable_miles"`
	RatePerMile  float64       `json:"rate_per_mile"`
	TotalPay     float64       `json:"total_pay"`
}

// DateRange is a simple start/end date pair.
type DateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// BuildAccountingExport transforms validated trip records into a payroll payload.
func BuildAccountingExport(trips []domain.TripRecord) *AccountingExport {
	export := &AccountingExport{
		ExportType: "accounting_payroll",
		ExportedAt: time.Now(),
		PayItems:   make([]AccountingPayItem, 0, len(trips)),
	}

	for _, t := range trips {
		totalMiles := 0
		if t.TotalMiles != nil {
			totalMiles = *t.TotalMiles
		}

		// Determine date range from line items
		dr := DateRange{}
		if len(t.LineItems) > 0 {
			if t.LineItems[0].Date != nil {
				dr.Start = *t.LineItems[0].Date
			}
			if last := t.LineItems[len(t.LineItems)-1].Date; last != nil {
				dr.End = *last
			}
		}

		item := AccountingPayItem{
			TripID:        t.ID,
			DateRange:     dr,
			TotalMiles:    totalMiles,
			BillableMiles: totalMiles,
			RatePerMile:   DefaultRatePerMile,
			TotalPay:      float64(totalMiles) * DefaultRatePerMile,
		}

		export.PayItems = append(export.PayItems, item)
	}

	return export
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// splitRoute splits a location string like "City A to City B" or
// "City A -> City B" into origin and destination.
func splitRoute(location string) (string, string) {
	// Try " -> " first, then " to "
	for _, sep := range []string{" -> ", " → ", " to "} {
		if parts := strings.SplitN(location, sep, 2); len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}
	// If no separator found, return the whole string as origin
	return location, ""
}
