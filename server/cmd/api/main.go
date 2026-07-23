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

	"github.com/trucking-poc/server/internal/handler"
	"github.com/trucking-poc/server/internal/service"
)

func main() {
	ctx := context.Background()

	// ---- Initialize services ----
	extractionSvc, err := service.NewExtractionService(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize extraction service: %v", err)
	}
	defer extractionSvc.Close()

	tripHandler := handler.NewTripHandler(extractionSvc)

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
	log.Printf("   GET  /health")

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	log.Println("Server stopped gracefully.")
}
