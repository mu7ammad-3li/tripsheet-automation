package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trucking-poc/server/internal/handler"
	"github.com/trucking-poc/server/internal/repository"
	"github.com/trucking-poc/server/internal/service"
	"github.com/trucking-poc/server/internal/storage"
)

func main() {
	ctx := context.Background()

	// ---- Database connection ----
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/trucking?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✅ Connected to Postgres")

	// ---- Initialize services ----
	extractionSvc, err := service.NewExtractionService(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize extraction service: %v", err)
	}
	defer extractionSvc.Close()

	tripRepo := repository.NewTripRepository(pool)

	auditPath := os.Getenv("AUDIT_IMAGE_PATH")
	if auditPath == "" {
		auditPath = "./audit_images"
	}
	auditStore, err := storage.NewAuditStore(auditPath)
	if err != nil {
		log.Fatalf("Failed to initialize audit store: %v", err)
	}
	log.Printf("✅ Audit images will be stored at: %s", auditPath)

	tripHandler := handler.NewTripHandler(extractionSvc, tripRepo, auditStore)
	exportHandler := handler.NewExportHandler(tripRepo)

	// ---- Set up router ----
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"trucking-trip-sheet-api"}`))
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/trips/extract", tripHandler.ExtractTrip)
		r.Get("/trips", tripHandler.ListTrips)
		r.Get("/trips/{id}", tripHandler.GetTrip)

		// Phase 4: Export endpoints
		r.Get("/trips/export/tms", exportHandler.ExportTMS)
		r.Get("/trips/export/accounting", exportHandler.ExportAccounting)
	})

	// ---- Start server ----
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Server shutdown failed: %v", err)
		}
	}()

	log.Printf("🚛 Trucking Trip Sheet API starting on :%s", port)
	log.Printf("   POST /api/v1/trips/extract")
	log.Printf("   GET  /api/v1/trips")
	log.Printf("   GET  /api/v1/trips/{id}")
	log.Printf("   GET  /api/v1/trips/export/tms")
	log.Printf("   GET  /api/v1/trips/export/accounting")
	log.Printf("   GET  /health")

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	log.Println("Server stopped gracefully.")
}
