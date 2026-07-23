# tripsheet-automation (Proof of Concept)

> **tripsheet-automation** is an AI-augmented, API-first enterprise solution designed to automate the ingestion, structured extraction, and deterministic validation of handwritten trucking trip sheets.

---

## Executive Summary & Business Case

### Context & Operational Relevance
Trip sheets are the primary operational records in transportation logistics. They are critical for tracking driver routes, stops, fuel purchases, IFTA (International Fuel Tax Agreement) jurisdictional mileage, and odometer readings. Currently, many fleets rely on dispatchers creating manual trip plans and drivers filling out paper-based sheets on the road.

### The Core Business Problem
The reliance on physical, handwritten logs creates significant operational and financial friction:
1. **Manual Data Ingestion Bottlenecks:** Critical billing and route data remains trapped on paper. Transcribing this data manually causes severe delays in driver payroll settlements, dispatch visibility, and IFTA tax reporting.
2. **Data Integrity & Quality Variance:** Documents filled out by hand in truck cabs suffer from high handwriting variability, smudged ink, and manual calculation errors (e.g., odometer discrepancies).
3. **Traditional OCR Limitations:** Standard template-based OCR systems fail on unstructured or high-variance handwritten sheets, lacking the layout understanding and contextual intelligence required to map data fields accurately.

### The Solution: Hybrid Extraction & Validation
This service isolates the AI (VLM) to handle high-variance unstructured data extraction while using **deterministic Go backend logic** to enforce strict validation rules. This hybrid approach ensures enterprise-grade accuracy, eliminates silent data corruption, and speeds up cash-flow cycles by accelerating payroll and billing hand-offs.

### Business Value & VLM Cost Efficiency
Automating extraction with the Gemini API is extremely cost-effective compared to manual transcription or legacy template OCR:
*   **Model Tier:** Serves Gemini 3.5 Flash Lite as the default extraction engine (Paid Tier).
*   **Paid Rates:** Input: **$0.075 per 1M** tokens (image is fixed at 259 tokens) | Output: **$0.30 per 1M** tokens (JSON response).
*   **Average Cost:** **`$0.25 to $0.45` per 1,000 trip sheets processed** ($0.00025 to $0.00045 per sheet).
*   **Caching Optimization:** Support for Gemini Context Caching can reduce input costs by an additional 50% for high-volume streams.


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

### Operational Ingestion Pathways

The system is designed to handle two primary ingestion workflows:

- **Depot Scan (Happy Path):** High-quality, flat scans produced by standard office scanners when drivers hand in paperwork at a terminal.

- **Truck Cab Mobile Photo (Edge Case):** Drivers submitting documents via mobile photos taken on the road. The system utilizes lightweight image preprocessing (grayscale conversion, contrast enhancement, sharpening) as a fallback to improve VLM legibility. Operational policy dictates that drivers are responsible for maintaining basic document readability; severely degraded images will be explicitly rejected by the validation handler.

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

## Web Dashboard Interface (Proof of Concept UI)

A responsive, high-fidelity single-page web dashboard is served directly by the Go backend to visualize the ingestion, extraction, and validation pipeline.

#### 1. Ingestion & Preprocessing Zone
The landing interface features a drag-and-drop zone with optional grayscale and contrast-boosting image preprocessing configuration for mobile "truck-cab" photos.

![Ingestion Interface](docs/images/screenshot_landing.png)

#### 2. Real-Time Extraction & Audit Viewer
Once uploaded, the screen splits to display the original audit document on the left and the VLM-extracted structured data (odometers, route logs) on the right.

![Extraction Audit Panel](docs/images/screenshot_extracted_data.png)

#### 3. Deterministic Validation Guardrails
The validation tab breaks down the outcome of the arithmetic validation checks (odometer delta, mileage sums, VLM confidence), indicating whether the trip is cleared or routed to the human-in-the-loop exception queue.

![Validation Guardrails](docs/images/screenshot_validation.png)

---

## POC Test Dataset & Benchmarking Results

To evaluate the VLM's extraction capabilities on high-variance handwritten sheets, a 4-image test dataset was generated to simulate clean office scans and challenging "truck-cab" mobile photos (motion blur, low light, shadows, crumpled paper).

| Sample | Ingestion Scenario | Challenge | Key Extracted Fields |
|---|---|---|---|
| `sample3_clean.jpg` | Flat Office Scan | Happy Path | Odometer: 102450 → 102780, 5 Route Legs |
| `sample1.jpg` | Truck Dashboard | Skewed angle, reflections | Odometer: 187421 → 187815, 4 Legs (Total Miles null) |
| `sample2_blurry.jpg` | Clipboard on Dash | Motion blur, night | Odometer: 245830 → 246215, 3 Route Legs |
| `sample4_dark.jpg` | Night Cab Photo | Shadows, crumpled paper | Odometer: 78200 → 78560, 2 Legs (Total Miles null) |

Our benchmarking harness achieved **100% extraction accuracy** across all numerical fields, dates, and route locations in the test dataset.

> [!NOTE]
> For the complete set of VLM-extracted JSON responses, code schemas, and detailed F1 evaluation metrics, refer to the [Phase 1 Planning Document](docs/phase_1_planning.md).

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
