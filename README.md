# Trucking Trip Sheet Automation

> **AI-augmented, API-first service** that automates the ingestion, extraction, and validation of trucking trip sheets — turning handwritten paper into structured, validated data ready for payroll and dispatch.

---

## Background & Context

Trip sheets are the lifeblood of trucking operations. They are the primary operational records used to track driver routes, stops, fuel purchases, IFTA (International Fuel Tax Agreement) jurisdictional mileage, and odometer readings during a haul.

Currently, many operations rely on dispatchers creating manual trip plans and drivers filling out paper-based sheets to log their actual trip details.

## The Core Problem

The reliance on physical, handwritten trip sheets creates significant operational friction:

- **Manual Data Entry Bottlenecks:** Critical operational data is trapped on paper. Manually transcribing this data into the system creates significant delays for downstream operations like driver payroll settlement, dispatch visibility, and fuel analytics.

- **Data Quality and Variability:** Physical trip sheets are filled out by hand in truck cabs. This leads to highly variable handwriting, unpredictable layouts, smeared ink, and human calculation errors (e.g., incorrect odometer math).

- **Limitations of Traditional Tech:** Standard OCR (Optical Character Recognition) struggles to accurately extract data from messy, unstructured handwritten forms. It lacks the contextual understanding needed to differentiate between a fuel receipt amount and a route mileage number when the layout shifts.

## The Solution

This project is an **AI-augmented, API-first service built in Go** that automates the ingestion, extraction, and validation of trucking trip sheets.

By isolating the AI to handle **only** unstructured data and using **deterministic code** for validation, the system provides high accuracy without silent data corruption.

```
📄 Paper Trip Sheet → 📸 Photo/Scan → POST /api/v1/trips/extract
  → Gemini VLM extracts structured JSON
  → Go validates business rules deterministically
  → Postgres persists + audit image saved
  → GET /export/tms        → dispatch data out
  → GET /export/accounting  → payroll data out
```

### Key Objectives

1. **Dual-Channel Ingestion:** Support both a digital web form (via QR code) for real-time entry and a scanning pipeline for physical paper sheets.

2. **Intelligent Extraction:** Utilize Vision Large Language Models (VLMs) strictly for unstructured, high-variance inputs (handwritten fields, checkboxes, border-crossing logs) to understand context and layout better than standard OCR.

3. **Automated Validation & Reconciliation:** Implement deterministic Go logic to enforce schema validation, perform arithmetic cross-checks on odometer/mileage data, and reconcile actual routes against dispatch plans.

4. **Downstream Integration:** Push the validated, structured JSON payload directly into the company's Transportation Management System (TMS) and accounting software to trigger automated workflows.

5. **Human-in-the-Loop Safeguards:** Require the VLM to assign confidence scores. Any low-confidence reads, missing required fields, or validation failures are automatically routed to an exception queue for human review.

### Environmental Realities

The system is designed to handle two primary ingestion pathways:

- **Clean Scans:** High-quality, flat scans produced by standard office scanners when drivers hand in paperwork at a depot.

- **The "Truck Cab" Edge Case:** Drivers submitting documents via mobile photos taken on the road. The system utilizes lightweight image preprocessing (deskewing, contrast enhancement) as a fallback. However, operational policy dictates that drivers are responsible for maintaining document legibility; severely degraded images will be explicitly rejected by the API.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Go API Server                        │
│                                                             │
│  ┌──────────┐   ┌──────────────┐   ┌────────────────────┐  │
│  │  Image    │   │  Gemini VLM  │   │  Validation Engine │  │
│  │  Preproc  │──▶│  Extraction  │──▶│  (Deterministic)   │  │
│  └──────────┘   └──────────────┘   └────────┬───────────┘  │
│                                              │              │
│                          ┌───────────────────┼──────────┐   │
│                          ▼                   ▼          │   │
│                   ┌─────────────┐    ┌──────────────┐   │   │
│                   │  Validated  │    │  Exception   │   │   │
│                   │  Queue      │    │  Queue       │   │   │
│                   └──────┬──────┘    └──────────────┘   │   │
│                          │                              │   │
│                ┌─────────┼──────────┐                   │   │
│                ▼                    ▼                    │   │
│         ┌────────────┐    ┌──────────────────┐          │   │
│         │ TMS Export │    │ Accounting Export │          │   │
│         └────────────┘    └──────────────────┘          │   │
│                                                         │   │
│  ┌──────────────────┐  ┌─────────────────────────────┐  │   │
│  │  Postgres        │  │  Audit Image Storage        │  │   │
│  │  (trips, items)  │  │  (local fs / S3)            │  │   │
│  └──────────────────┘  └─────────────────────────────┘  │   │
└─────────────────────────────────────────────────────────────┘
```

### Validation Guardrails

The Go backend enforces these deterministic checks **after** VLM extraction:

| Check | Rule | On Failure |
|-------|------|------------|
| Odometer Delta | `close - open ≈ total_miles` (±5%) | → Exception Queue |
| Line Item Sum | `sum(line_items[].miles) ≈ total_miles` (±5%) | → Exception Queue |
| Confidence Threshold | `confidence_score > 0.85` | → Exception Queue |
| Required Fields | Odometer values must be non-null | → Exception Queue |
| Odometer Sanity | `close > open` | → Exception Queue |

---

## Project Structure

```
├── scripts/
│   └── benchmark_vlm.py         # Phase 1: Python VLM benchmarking & F1 scoring
├── test_data/
│   ├── images/                   # Sample trip sheet images (clean + edge cases)
│   └── ground_truth.json         # Human-labeled ground truth for F1 scoring
├── docs/
│   ├── phase_1_planning.md       # VLM extraction engineering decisions
│   ├── phase_2_planning.md       # Go API architecture & validation logic
│   ├── phase_3_planning.md       # Postgres schema & persistence design
│   └── phase_4_planning.md       # TMS/Accounting export design
├── server/
│   ├── cmd/api/main.go           # Entry point, dependency wiring, server start
│   ├── internal/
│   │   ├── domain/               # Core structs (TripSheet, LineItem, TripRecord)
│   │   ├── handler/              # HTTP handlers (extraction, export)
│   │   ├── service/              # Business logic (VLM, validation, export transforms)
│   │   ├── repository/           # Postgres persistence (pgx/v5, atomic transactions)
│   │   ├── preprocessing/        # Image enhancement (grayscale, contrast, sharpen)
│   │   └── storage/              # Audit image filesystem store
│   ├── migrations/               # SQL schema migrations
│   ├── go.mod
│   └── go.sum
├── requirements.txt              # Python dependencies (Phase 1 benchmarking)
├── .gitignore
└── MVP Implementation Plan.md    # Original 4-phase implementation plan
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/trips/extract` | Upload image → VLM extract → validate → persist |
| `GET` | `/api/v1/trips` | List all persisted trips |
| `GET` | `/api/v1/trips/{id}` | Get single trip with line items |
| `GET` | `/api/v1/trips/export/tms` | Export validated trips as TMS dispatch payload |
| `GET` | `/api/v1/trips/export/accounting` | Export validated trips as payroll payload |
| `GET` | `/health` | Health check |

---

## Quick Start

### 1. Phase 1 — VLM Benchmarking (Python)

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
export GEMINI_API_KEY="your_key_here"
python scripts/benchmark_vlm.py
```

### 2. Phases 2–4 — Go API Server

```bash
# Set up Postgres
createdb trucking
psql -d trucking -f server/migrations/001_create_trips.up.sql

# Run the server
cd server
export GEMINI_API_KEY="your_key_here"
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/trucking?sslmode=disable"
go run ./cmd/api/
```

### 3. Test the full pipeline

```bash
# Extract a trip sheet
curl -X POST http://localhost:8080/api/v1/trips/extract \
  -F "image=@test_data/images/sample3_clean.jpg"

# List all trips
curl http://localhost:8080/api/v1/trips

# Export for TMS
curl http://localhost:8080/api/v1/trips/export/tms

# Export for Accounting
curl http://localhost:8080/api/v1/trips/export/accounting
```

---

## MVP Implementation Phases

| Phase | Description | Status |
|-------|-------------|--------|
| **Phase 1** | AI Extraction Core (VLM → JSON) — Python benchmarking with F1 scoring | ✅ Complete |
| **Phase 2** | Go Ingestion & Validation API — chi router, deterministic guardrails | ✅ Complete |
| **Phase 3** | Persistence (Postgres) — pgx/v5, atomic transactions, audit storage | ✅ Complete |
| **Phase 4** | TMS/Accounting Hand-off — simulated export endpoints | ✅ Complete |

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **VLM** | Google Gemini 3.5 Flash Lite |
| **Backend** | Go 1.22+, chi router |
| **Validation** | go-playground/validator, custom arithmetic checks |
| **Database** | PostgreSQL, pgx/v5 |
| **Image Processing** | disintegration/imaging |
| **Benchmarking** | Python 3.12, Pydantic, google-genai SDK |
