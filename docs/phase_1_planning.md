# Phase 1: AI Extraction Core (VLM → JSON) — Planning

## Context

> Trip sheets are filled out by hand in truck cabs. This leads to highly variable handwriting, unpredictable layouts, smeared ink, and human calculation errors. Standard OCR lacks the contextual understanding needed to differentiate between a fuel receipt amount and a route mileage number when the layout shifts.

This phase isolates the Vision Language Model (VLM) extraction to prove that we can consistently get structured, schema-compliant JSON from raw handwritten trip sheet images — before committing to the full Go backend.

## Objective

Benchmark VLM accuracy against human-labeled ground truth data across a range of image quality scenarios (clean scans, blurry photos, dark truck cab shots) and validate that the extraction meets the quality bar needed for downstream automation.

---

## Engineering Decisions

### 1. Benchmarking Stack: Python + Pydantic
While the production backend (Phase 2) is built in Go, **Python** is used for Phase 1 benchmarking. Python provides the best ecosystem for rapid prompt iteration, schema generation (via Pydantic), and evaluating model responses. The finalized schema and prompt port directly to the Go service.

### 2. Enforcing Structured Output
To avoid "naive" prompting issues (markdown wrappers, hallucinated keys), we enforce constrained generation:
- **`response_mime_type: "application/json"`** — forces the model to return raw JSON, no markdown.
- **Schema-in-prompt** — the expected JSON schema is embedded directly in the system prompt, ensuring compatibility across all Gemini model tiers (including Lite).
- **Pydantic for local validation** — the response is validated against a Pydantic model after extraction, acting as a second safety net.

### 3. Prompt Engineering Strategy ("Output Contract")
The prompt acts as a strict contract between the system and the VLM:
- **Role**: "You are a precise data extraction system for trucking trip sheets."
- **Constraint**: "Extract the information EXACTLY as it appears. Do NOT guess or infer missing data."
- **Null handling**: "If a field is blank or illegible, set its value to `null` and add the field name to the `flagged_fields` array."
- **Confidence**: "Provide a `confidence_score` between 0.0 and 1.0 reflecting overall legibility."

### 4. Model Selection: Gemini 3.5 Flash Lite
Selected for its availability on the free API tier and strong OCR/vision capabilities. The same prompt and schema work identically with higher-tier models (Flash, Pro) for production use.

### 5. F1 Scoring Methodology
- **Scalar fields** (`odometer_open`, `odometer_close`, `total_miles`) — exact match
- **Line items** — matched positionally (row 0 vs row 0), each sub-field (`date`, `location`, `miles`) scored independently
- **String normalization** — lowercased, commas stripped, `->` / `→` / `to` treated as equivalent separators
- **Metrics**: Precision, Recall, F1 per image + micro-averaged F1 across all images

---

## Data Schema

```json
{
  "odometer_open": "<int or null>",
  "odometer_close": "<int or null>",
  "total_miles": "<int or null>",
  "line_items": [
    {"date": "<string or null>", "location": "<string or null>", "miles": "<int or null>"}
  ],
  "confidence_score": "<float 0.0-1.0>",
  "flagged_fields": ["<field_name>", "..."]
}
```

## Test Dataset

| Image | Scenario | Challenge |
|-------|----------|-----------|
| `sample1.jpg` | Truck dashboard photo | Angled, dirty paper, mixed handwriting |
| `sample2_blurry.jpg` | Clipboard on dash, night | Motion blur, low light |
| `sample3_clean.jpg` | Office scan | Happy path — 5 line items, neat writing |
| `sample4_dark.jpg` | Dark cab, crumpled paper | Shadows, partially obscured numbers |

## Benchmark Results

All images processed with Gemini 3.5 Flash Lite. Average response time: ~3.7s per image.

| Image | F1 Score | Notes |
|-------|----------|-------|
| sample1.jpg | 100% | Perfect extraction — all fields matched |
| sample2_blurry.jpg | 100%* | All values correct after normalizing separators |
| sample3_clean.jpg | 100%* | All values correct after normalizing separators |
| sample4_dark.jpg | 100%* | All values correct after normalizing separators |
| **Micro-averaged** | **~100%** | *After separator normalization fix* |

> \* Raw F1 was 72-78% due to `->` vs `to` formatting differences in location strings. After normalizing separators in the scoring logic, all extractions matched ground truth.

## Key Findings

1. **VLM accuracy is excellent** — every numerical value (odometers, miles, dates) was extracted perfectly across all 4 test images.
2. **String formatting varies** — the model uses `to` instead of `->` for route separators. The downstream system must normalize these.
3. **Null handling works** — the model correctly returns `null` and populates `flagged_fields` when data is missing from the sheet.
4. **Confidence scoring is reasonable** — clean scans get 1.0, darker/messier images get 0.95.

## Files

- [`scripts/benchmark_vlm.py`](../scripts/benchmark_vlm.py) — Benchmarking script with F1 scoring
- [`test_data/ground_truth.json`](../test_data/ground_truth.json) — Human-labeled expected values
- [`test_data/images/`](../test_data/images/) — Sample trip sheet images
