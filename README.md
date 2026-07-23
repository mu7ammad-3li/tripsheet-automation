# tripsheet-automation (Proof of Concept)

> **AI-augmented, API-first Proof of Concept (POC) service** that automates the ingestion, extraction, and validation of trucking trip sheets — turning handwritten paper into structured, validated data ready for payroll and dispatch.

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

```mermaid
graph TD
    subgraph Go API Server
        direction TB
        A[Scanned Image / Mobile Photo] --> B[Image Preprocessing]
        B --> C[Gemini VLM Extraction]
        C --> D[Validation Engine]
        
        D -->|Pass| E[Validated Queue]
        D -->|Fail| F[Exception Queue]
        
        E --> G[TMS Export]
        E --> H[Accounting Export]
        
        E & F --> I[(Postgres DB)]
        E & F --> J[Audit Image Storage]
    end
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

## POC Test Dataset & GenAI Extraction Results

This Proof of Concept benchmark uses a set of 4 test images generated to cover both clean scanner inputs and the challenging environmental realities of truck cab photos (skewed angles, motion blur, bad lighting, crumpled paper).

Below are the test images and the actual structured JSON objects returned by the Gemini 3.5 Flash Lite VLM:

### 1. Happy Path: Clean Office Scan (`sample3_clean.jpg`)
* **Description:** A flat, clean scan on a white background with neat handwriting.
* **Image:**
  ![Clean Scan](test_data/images/sample3_clean.jpg)
* **VLM Extraction Response:**
  <details>
  <summary>Show JSON Output</summary>

  ```json
  {
    "odometer_open": 102450,
    "odometer_close": 102780,
    "total_miles": 436,
    "line_items": [
      {"date": "7/01/24", "location": "Chicago, IL to Milwaukee, WI", "miles": 92},
      {"date": "7/01/24", "location": "Milwaukee, WI to Madison, WI", "miles": 79},
      {"date": "7/02/24", "location": "Madison, WI to Rockford, IL", "miles": 67},
      {"date": "7/02/24", "location": "Rockford, IL to Springfield, IL", "miles": 102},
      {"date": "7/02/24", "location": "Springfield, IL to St Louis, MO", "miles": 96}
    ],
    "confidence_score": 1.0,
    "flagged_fields": []
  }
  ```
  </details>

### 2. Dashboard Photo (`sample1.jpg`)
* **Description:** A mobile photo of a trip sheet lying on a truck dashboard, angled with reflections.
* **Image:**
  ![Dashboard Photo](test_data/images/sample1.jpg)
* **VLM Extraction Response:**
  <details>
  <summary>Show JSON Output</summary>

  ```json
  {
    "odometer_open": 187421,
    "odometer_close": 187815,
    "total_miles": null,
    "line_items": [
      {"date": "5/20/24", "location": "Springfield, MO -> St. Louis, MO", "miles": 224},
      {"date": "5/20/24", "location": "St. Louis, MO -> Troy, IL (Fuel)", "miles": 28},
      {"date": "5/21/24", "location": "Troy, IL -> Indianapolis, IN", "miles": 247},
      {"date": "5/21/24", "location": "Indianapolis, IN -> Columbus, OH", "miles": 176}
    ],
    "confidence_score": 0.95,
    "flagged_fields": ["total_miles"]
  }
  ```
  </details>

### 3. Motion Blurred & Messy (`sample2_blurry.jpg`)
* **Description:** A blurry, out-of-focus photo of a trip sheet on a clipboard inside a truck.
* **Image:**
  ![Blurry Photo](test_data/images/sample2_blurry.jpg)
* **VLM Extraction Response:**
  <details>
  <summary>Show JSON Output</summary>

  ```json
  {
    "odometer_open": 245830,
    "odometer_close": 246215,
    "total_miles": 472,
    "line_items": [
      {"date": "6/10/24", "location": "Dallas TX to Austin TX", "miles": 195},
      {"date": "6/10/24", "location": "Austin TX to San Antonio TX", "miles": 80},
      {"date": "6/11/24", "location": "San Antonio TX to Houston TX", "miles": 197}
    ],
    "confidence_score": 1.0,
    "flagged_fields": []
  }
  ```
  </details>

### 4. Poor Night Lighting & Crumpled (`sample4_dark.jpg`)
* **Description:** A dark, poorly lit photo taken at night inside a truck cab with heavy shadows and crumpled paper.
* **Image:**
  ![Dark/Shadow Photo](test_data/images/sample4_dark.jpg)
* **VLM Extraction Response:**
  <details>
  <summary>Show JSON Output</summary>

  ```json
  {
    "odometer_open": 78200,
    "odometer_close": 78560,
    "total_miles": null,
    "line_items": [
      {"date": "3/15/24", "location": "Atlanta GA to Birmingham AL", "miles": 148},
      {"date": "3/16/24", "location": "Birmingham AL to Memphis TN", "miles": 243}
    ],
    "confidence_score": 0.95,
    "flagged_fields": ["total_miles"]
  }
  ```
  </details>

### F1 Scoring Summary
Our Python benchmarking harness evaluates the GenAI extractions against human-labeled ground truth values. After normalizing string separators (such as mapping `->` and `to` to the same format), the VLM achieved **100% extraction accuracy** for all numerical fields, dates, and locations across the test dataset.

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
