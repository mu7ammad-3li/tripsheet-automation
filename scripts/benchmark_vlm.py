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

def process_image(client: genai.Client, image_path: Path):
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
        return

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

    except json.JSONDecodeError as e:
        print(f"\n❌ JSON Parse Failed: {e}")
        print("Raw Output:", response.text)
    except Exception as e:
        print(f"\n❌ Validation Failed: {e}")
        print("Raw Output:", response.text)


def main():
    api_key = os.environ.get("GEMINI_API_KEY")
    if not api_key:
        print("❌ Error: Please set GEMINI_API_KEY environment variable.")
        sys.exit(1)

    print(f"API Key loaded: {api_key[:8]}...{api_key[-4:]}")
    client = genai.Client(api_key=api_key)

    base_dir = Path(__file__).parent.parent
    images_dir = base_dir / "test_data" / "images"

    if not images_dir.exists():
        print(f"❌ Error: Directory not found -> {images_dir}")
        sys.exit(1)

    image_files = sorted([
        f for f in images_dir.iterdir()
        if f.suffix.lower() in (".png", ".jpg", ".jpeg")
    ])

    if not image_files:
        print(f"⚠️ No images found in {images_dir}. Please add some sample images.")
        sys.exit(1)

    print(f"Found {len(image_files)} image(s) to process.\n")
    for img_path in image_files:
        process_image(client, img_path)


if __name__ == "__main__":
    main()
