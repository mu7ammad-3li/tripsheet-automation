import os
import sys
import json
import time
import logging
from pathlib import Path
from pydantic import BaseModel, Field
from google import genai
from google.genai import types
from dotenv import load_dotenv

load_dotenv()

# Show HTTP-level activity so we can see retries / hangs
logging.basicConfig(
    level=logging.DEBUG,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    stream=sys.stderr,
)

# ---------------------------------------------------------------------------
# Pydantic models (used for LOCAL validation of the response)
# ---------------------------------------------------------------------------
class LineItem(BaseModel):
    date: str | None = Field(description="Date of the trip leg")
    location: str | None = Field(description="Location of the trip leg")
    miles: int | None = Field(description="Miles driven for this leg")

class TripSheet(BaseModel):
    odometer_open: int | None = Field(description="Starting odometer reading")
    odometer_close: int | None = Field(description="Ending odometer reading")
    total_miles: int | None = Field(description="Total miles calculated or written")
    line_items: list[LineItem] = Field(description="List of individual trip legs")
    confidence_score: float = Field(description="Confidence score between 0.0 and 1.0 based on legibility")
    flagged_fields: list[str] = Field(description="List of field names that were illegible or missing")

# ---------------------------------------------------------------------------
# The JSON schema we want back – embedded directly in the prompt so it works
# with every model tier, including Lite.
# ---------------------------------------------------------------------------
EXPECTED_SCHEMA = """{
  "odometer_open": <int or null>,
  "odometer_close": <int or null>,
  "total_miles": <int or null>,
  "line_items": [
    {"date": "<string or null>", "location": "<string or null>", "miles": <int or null>}
  ],
  "confidence_score": <float 0.0-1.0>,
  "flagged_fields": ["<field_name>", ...]
}"""

SYSTEM_PROMPT = f"""You are a precise data extraction system for trucking trip sheets.

RULES:
1. Extract the information EXACTLY as it appears on the document.
2. Return ONLY a single valid JSON object — no markdown fences, no explanation.
3. Do NOT guess or infer missing data. If a field is blank or illegible, set its value to null and add the field name to the "flagged_fields" array.
4. Provide a "confidence_score" between 0.0 and 1.0 reflecting overall legibility.

REQUIRED JSON SCHEMA:
{EXPECTED_SCHEMA}
"""

# ---------------------------------------------------------------------------
# F1 Scoring Logic
# ---------------------------------------------------------------------------
def normalize_str(val: str | None) -> str | None:
    """Lowercase, strip whitespace/punctuation, normalize location separators."""
    if val is None:
        return None
    s = val.lower().strip()
    s = s.replace(",", "").replace(".", "")
    # Normalize all arrow/separator variants to a single form
    s = s.replace("→", " to ").replace("->", " to ").replace("—>", " to ")
    # Collapse multiple spaces
    s = " ".join(s.split())
    return s


def score_scalar(predicted, expected, field_name: str) -> dict:
    """Score a single scalar field. Returns dict with tp, fp, fn counts."""
    tp = fp = fn = 0
    match = False

    if expected is None and predicted is None:
        tp += 1  # Both null — correct
        match = True
    elif expected is None and predicted is not None:
        fp += 1  # Predicted something when there was nothing
    elif expected is not None and predicted is None:
        fn += 1  # Missed a value
    elif predicted == expected:
        tp += 1  # Exact match
        match = True
    else:
        fp += 1  # Wrong value
        fn += 1

    return {"field": field_name, "expected": expected, "predicted": predicted,
            "match": match, "tp": tp, "fp": fp, "fn": fn}


def score_line_items(predicted_items: list[dict], expected_items: list[dict]) -> list[dict]:
    """Score line items positionally. Each sub-field (date, location, miles) scored independently."""
    results = []
    max_len = max(len(predicted_items), len(expected_items))

    for i in range(max_len):
        pred = predicted_items[i] if i < len(predicted_items) else {}
        exp = expected_items[i] if i < len(expected_items) else {}

        for field in ["date", "location", "miles"]:
            pred_val = pred.get(field)
            exp_val = exp.get(field)

            # For strings, do a normalized comparison
            if isinstance(pred_val, str) and isinstance(exp_val, str):
                is_match = normalize_str(pred_val) == normalize_str(exp_val)
                tp = 1 if is_match else 0
                fp = 0 if is_match else 1
                fn = 0 if is_match else 1
            else:
                r = score_scalar(pred_val, exp_val, "")
                tp, fp, fn = r["tp"], r["fp"], r["fn"]
                is_match = r["match"]

            results.append({
                "field": f"line_items[{i}].{field}",
                "expected": exp_val,
                "predicted": pred_val,
                "match": is_match,
                "tp": tp, "fp": fp, "fn": fn,
            })

    return results


def calculate_f1(results: list[dict]) -> dict:
    """Aggregate TP/FP/FN into precision, recall, and F1."""
    tp = sum(r["tp"] for r in results)
    fp = sum(r["fp"] for r in results)
    fn = sum(r["fn"] for r in results)

    precision = tp / (tp + fp) if (tp + fp) > 0 else 0.0
    recall = tp / (tp + fn) if (tp + fn) > 0 else 0.0
    f1 = (2 * precision * recall / (precision + recall)) if (precision + recall) > 0 else 0.0

    return {"tp": tp, "fp": fp, "fn": fn,
            "precision": precision, "recall": recall, "f1": f1}


def score_extraction(extracted: dict, ground_truth: dict) -> dict:
    """Compare an extraction result against its ground truth and return F1 metrics."""
    field_results = []

    # Score scalar fields
    for field in ["odometer_open", "odometer_close", "total_miles"]:
        field_results.append(
            score_scalar(extracted.get(field), ground_truth.get(field), field)
        )

    # Score line items
    pred_items = extracted.get("line_items", [])
    exp_items = ground_truth.get("line_items", [])
    field_results.extend(score_line_items(pred_items, exp_items))

    metrics = calculate_f1(field_results)
    return {"field_results": field_results, "metrics": metrics}


def print_scorecard(image_name: str, score: dict):
    """Pretty-print the per-image scorecard."""
    m = score["metrics"]
    print(f"\n{'─'*60}")
    print(f"📊 SCORECARD: {image_name}")
    print(f"{'─'*60}")

    # Field-by-field breakdown
    print(f"  {'Field':<30} {'Expected':<20} {'Predicted':<20} {'Match'}")
    print(f"  {'─'*90}")
    for r in score["field_results"]:
        marker = "✅" if r["match"] else "❌"
        exp_str = str(r["expected"])[:18]
        pred_str = str(r["predicted"])[:18]
        print(f"  {r['field']:<30} {exp_str:<20} {pred_str:<20} {marker}")

    # Aggregate
    print(f"\n  TP: {m['tp']}  |  FP: {m['fp']}  |  FN: {m['fn']}")
    print(f"  Precision: {m['precision']:.2%}  |  Recall: {m['recall']:.2%}  |  F1: {m['f1']:.2%}")


# ---------------------------------------------------------------------------
# VLM Extraction
# ---------------------------------------------------------------------------
def process_image(client: genai.Client, image_path: Path) -> dict | None:
    """Send an image to the VLM and return the parsed dict, or None on failure."""
    print(f"\n{'='*60}")
    print(f"Processing: {image_path.name}")
    print(f"{'='*60}")

    # Read image bytes
    with open(image_path, "rb") as f:
        image_bytes = f.read()
    print(f"  Image size: {len(image_bytes):,} bytes")

    mime_type = "image/jpeg" if image_path.suffix.lower() in [".jpg", ".jpeg"] else "image/png"
    image_part = types.Part.from_bytes(data=image_bytes, mime_type=mime_type)

    print("  Sending request to Gemini 3.5 Flash Lite ...")
    start = time.time()

    try:
        response = client.models.generate_content(
            model="gemini-3.5-flash-lite",
            contents=[
                image_part,
                "Extract the trip sheet data from this image. Return ONLY valid JSON.",
            ],
            config=types.GenerateContentConfig(
                system_instruction=SYSTEM_PROMPT,
                response_mime_type="application/json",
                temperature=0.0,
            ),
        )
    except Exception as e:
        elapsed = time.time() - start
        print(f"  ❌ API call failed after {elapsed:.1f}s: {type(e).__name__}: {e}")
        return None

    elapsed = time.time() - start
    print(f"  ✅ Response received in {elapsed:.1f}s")

    # ---- Parse & validate ----
    print("\n--- Extracted JSON ---")
    try:
        data = json.loads(response.text)
        print(json.dumps(data, indent=2))

        trip_sheet = TripSheet.model_validate(data)
        print("\n✅ Pydantic Validation Passed!")
        print(f"   Odometer: {trip_sheet.odometer_open} → {trip_sheet.odometer_close}")
        print(f"   Total Miles: {trip_sheet.total_miles}")
        print(f"   Line Items: {len(trip_sheet.line_items)}")
        print(f"   Confidence: {trip_sheet.confidence_score}")
        if trip_sheet.flagged_fields:
            print(f"   ⚠️  Flagged: {trip_sheet.flagged_fields}")
        return data

    except json.JSONDecodeError as e:
        print(f"\n❌ JSON Parse Failed: {e}")
        print("Raw Output:", response.text)
        return None
    except Exception as e:
        print(f"\n❌ Validation Failed: {e}")
        print("Raw Output:", response.text)
        return None


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
def main():
    api_key = os.environ.get("GEMINI_API_KEY")
    if not api_key:
        print("❌ Error: Please set GEMINI_API_KEY environment variable.")
        sys.exit(1)

    print(f"API Key loaded: {api_key[:8]}...{api_key[-4:]}")
    client = genai.Client(api_key=api_key)

    base_dir = Path(__file__).parent.parent
    images_dir = base_dir / "test_data" / "images"
    gt_path = base_dir / "test_data" / "ground_truth.json"

    if not images_dir.exists():
        print(f"❌ Error: Directory not found -> {images_dir}")
        sys.exit(1)

    # Load ground truth
    ground_truth = {}
    if gt_path.exists():
        with open(gt_path) as f:
            ground_truth = json.load(f)
        print(f"Ground truth loaded: {len(ground_truth)} entries")
    else:
        print("⚠️  No ground_truth.json found — skipping F1 scoring.")

    image_files = sorted([
        f for f in images_dir.iterdir()
        if f.suffix.lower() in (".png", ".jpg", ".jpeg")
    ])

    if not image_files:
        print(f"⚠️ No images found in {images_dir}. Please add some sample images.")
        sys.exit(1)

    print(f"Found {len(image_files)} image(s) to process.\n")

    all_scores = []
    for img_path in image_files:
        extracted = process_image(client, img_path)

        # F1 scoring if we have both extraction and ground truth
        if extracted and img_path.name in ground_truth:
            score = score_extraction(extracted, ground_truth[img_path.name])
            print_scorecard(img_path.name, score)
            all_scores.append((img_path.name, score))
        elif extracted and img_path.name not in ground_truth:
            print(f"\n  ⚠️  No ground truth for {img_path.name} — skipping F1.")

    # ---- Overall Summary ----
    if all_scores:
        print(f"\n\n{'═'*60}")
        print(f"📈 OVERALL BENCHMARK SUMMARY")
        print(f"{'═'*60}")

        total_tp = total_fp = total_fn = 0
        for name, score in all_scores:
            m = score["metrics"]
            total_tp += m["tp"]
            total_fp += m["fp"]
            total_fn += m["fn"]
            print(f"  {name:<30} F1: {m['f1']:.2%}  (P: {m['precision']:.2%}  R: {m['recall']:.2%})")

        # Micro-averaged F1
        overall_p = total_tp / (total_tp + total_fp) if (total_tp + total_fp) > 0 else 0.0
        overall_r = total_tp / (total_tp + total_fn) if (total_tp + total_fn) > 0 else 0.0
        overall_f1 = (2 * overall_p * overall_r / (overall_p + overall_r)) if (overall_p + overall_r) > 0 else 0.0

        print(f"\n  {'MICRO-AVERAGED':<30} F1: {overall_f1:.2%}  (P: {overall_p:.2%}  R: {overall_r:.2%})")
        print(f"  Total fields scored: {total_tp + total_fp + total_fn}")
        print(f"{'═'*60}")


if __name__ == "__main__":
    main()
