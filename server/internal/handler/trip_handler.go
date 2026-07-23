package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/trucking-poc/server/internal/domain"
	"github.com/trucking-poc/server/internal/preprocessing"
	"github.com/trucking-poc/server/internal/repository"
	"github.com/trucking-poc/server/internal/service"
	"github.com/trucking-poc/server/internal/storage"
)

const maxUploadSize = 10 << 20 // 10 MB

// TripHandler handles HTTP requests for trip sheet extraction.
type TripHandler struct {
	extractionService *service.ExtractionService
	tripRepo          *repository.TripRepository
	auditStore        *storage.AuditStore
}

// NewTripHandler creates a new handler with the given dependencies.
func NewTripHandler(es *service.ExtractionService, repo *repository.TripRepository, audit *storage.AuditStore) *TripHandler {
	return &TripHandler{
		extractionService: es,
		tripRepo:          repo,
		auditStore:        audit,
	}
}

// ExtractTrip handles POST /api/v1/trips/extract
// Accepts a multipart form with a single image file field named "image".
func (h *TripHandler) ExtractTrip(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// ---- 1. Parse multipart form ----
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "Failed to parse multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing 'image' field in form data")
		return
	}
	defer file.Close()

	log.Printf("Received file: %s (%d bytes)", header.Filename, header.Size)

	// ---- 2. Read image bytes ----
	imageBytes, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read uploaded file")
		return
	}

	// ---- 3. Validate MIME type using content sniffing ----
	detectedType := http.DetectContentType(imageBytes)
	if detectedType != "image/jpeg" && detectedType != "image/png" {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("Unsupported image type: %s. Only JPEG and PNG are accepted.", detectedType))
		return
	}
	log.Printf("Detected MIME type: %s", detectedType)

	// ---- 4. Optional preprocessing (contrast/grayscale boost) ----
	preprocess := r.URL.Query().Get("preprocess")
	if preprocess == "true" || preprocess == "1" {
		log.Println("Applying image preprocessing (grayscale + contrast + sharpen)...")
		processed, newMime, err := preprocessing.EnhanceImage(imageBytes, detectedType)
		if err != nil {
			log.Printf("Warning: preprocessing failed, using original image: %v", err)
		} else {
			imageBytes = processed
			detectedType = newMime
			log.Printf("Preprocessing complete. New size: %d bytes", len(imageBytes))
		}
	}

	// ---- 5. Call VLM extraction ----
	log.Println("Sending image to Gemini VLM for extraction...")
	tripSheet, err := h.extractionService.ExtractFromImage(r.Context(), imageBytes, detectedType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VLM extraction failed: "+err.Error())
		return
	}
	log.Printf("Extraction complete. Confidence: %.2f, Line items: %d",
		tripSheet.ConfidenceScore, len(tripSheet.LineItems))

	// ---- 6. Run deterministic validation ----
	status, validation := service.ValidateTripSheet(tripSheet)
	log.Printf("Validation result: %s (errors: %d)", status, len(validation.Errors))

	// ---- 7. Persist to database ----
	record := buildTripRecord(tripSheet, status, validation)

	// Save audit image first (we need the trip ID from the DB insert)
	if err := h.tripRepo.SaveTrip(r.Context(), record); err != nil {
		log.Printf("Warning: failed to persist trip to database: %v", err)
		// Don't fail the request — still return the extraction result
	} else {
		log.Printf("Trip persisted to DB with ID: %s", record.ID)

		// Save audit image keyed by trip ID
		if h.auditStore != nil {
			imgPath, err := h.auditStore.SaveImage(record.ID, imageBytes, detectedType)
			if err != nil {
				log.Printf("Warning: failed to save audit image: %v", err)
			} else {
				record.ImagePath = imgPath
				log.Printf("Audit image saved: %s", imgPath)
				// Persist the stored image path back to the database
				if err := h.tripRepo.UpdateImagePath(r.Context(), record.ID, imgPath); err != nil {
					log.Printf("Warning: failed to update image path in DB: %v", err)
				}
			}
		}
	}

	// ---- 8. Build and return response ----
	response := domain.ExtractionResponse{
		Status:      status,
		TripSheet:   tripSheet,
		Validation:  validation,
		ProcessedAt: time.Now(),
	}

	log.Printf("Request completed in %s", time.Since(startTime))
	writeJSON(w, http.StatusOK, response)
}

// GetTrip handles GET /api/v1/trips/{id}
func (h *TripHandler) GetTrip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing trip ID")
		return
	}

	record, err := h.tripRepo.GetTripByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Trip not found: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, record)
}

// ListTrips handles GET /api/v1/trips
func (h *TripHandler) ListTrips(w http.ResponseWriter, r *http.Request) {
	trips, err := h.tripRepo.ListTrips(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list trips: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, trips)
}

// buildTripRecord converts a TripSheet + validation result into a persistence record.
func buildTripRecord(ts *domain.TripSheet, status string, val *domain.ValidationResult) *domain.TripRecord {
	record := &domain.TripRecord{
		OdometerOpen:     ts.OdometerOpen,
		OdometerClose:    ts.OdometerClose,
		TotalMiles:       ts.TotalMiles,
		ConfidenceScore:  ts.ConfidenceScore,
		FlaggedFields:    ts.FlaggedFields,
		Status:           status,
		ValidationErrors: val.Errors,
	}

	for i, item := range ts.LineItems {
		record.LineItems = append(record.LineItems, domain.LineItemRecord{
			Date:      item.Date,
			Location:  item.Location,
			Miles:     item.Miles,
			SortOrder: i,
		})
	}

	return record
}

// writeJSON sends a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// writeError sends a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	log.Printf("Error [%d]: %s", status, message)
	writeJSON(w, status, map[string]string{"error": message})
}
