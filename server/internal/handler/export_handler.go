package handler

import (
	"log"
	"net/http"

	"github.com/trucking-poc/server/internal/repository"
	"github.com/trucking-poc/server/internal/service"
)

// ExportHandler handles the simulated TMS and Accounting export endpoints.
type ExportHandler struct {
	tripRepo *repository.TripRepository
}

// NewExportHandler creates a new export handler.
func NewExportHandler(repo *repository.TripRepository) *ExportHandler {
	return &ExportHandler{tripRepo: repo}
}

// ExportTMS handles GET /api/v1/trips/export/tms
// Returns all validated trips transformed into TMS dispatch format.
func (h *ExportHandler) ExportTMS(w http.ResponseWriter, r *http.Request) {
	trips, err := h.tripRepo.ListValidatedTrips(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch validated trips: "+err.Error())
		return
	}

	if len(trips) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "No validated trips available for export",
		})
		return
	}

	export := service.BuildTMSExport(trips)
	log.Printf("TMS export: %d trips", len(export.Trips))
	writeJSON(w, http.StatusOK, export)
}

// ExportAccounting handles GET /api/v1/trips/export/accounting
// Returns all validated trips transformed into payroll format.
func (h *ExportHandler) ExportAccounting(w http.ResponseWriter, r *http.Request) {
	trips, err := h.tripRepo.ListValidatedTrips(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch validated trips: "+err.Error())
		return
	}

	if len(trips) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "No validated trips available for export",
		})
		return
	}

	export := service.BuildAccountingExport(trips)
	log.Printf("Accounting export: %d pay items", len(export.PayItems))
	writeJSON(w, http.StatusOK, export)
}
