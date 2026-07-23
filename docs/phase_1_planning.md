# Phase 1: AI Extraction Core (VLM -> JSON) Planning

## Objective
Isolate the Vision Language Model (VLM) extraction to ensure reliable, structured, and schema-compliant JSON output from raw trucking trip sheet images, prior to building the deterministic Go backend.

## Engineering Decisions & Best Practices

### 1. Technology Choice for Benchmarking
While the production backend (Phase 2) will be built in Go, **Python** is recommended for Phase 1 benchmarking. Python provides the best ecosystem for rapid prompt iteration, schema generation (via Pydantic), and evaluating model responses. Once the schema and prompt are finalized, they will be easily ported to the Go service.

### 2. Enforcing Structured Output
To avoid "naive" prompting issues (like markdown wrappers or hallucinated keys), we will enforce constrained generation:
*   **Native Structured Outputs**: Use provider-specific API features (e.g., Google Gemini's `response_mime_type: "application/json"` and `response_schema` or OpenAI's `response_format: { type: "json_schema" }`). This forces the model to adhere strictly to the schema at the token level.
*   **Pydantic for Schema Definition**: Define the trip sheet schema as a Pydantic model in Python, and export its JSON Schema for the VLM API. This gives us a single source of truth for validation during the benchmarking phase.

### 3. Prompt Engineering Strategy
The prompt will act as a strict contract. Key principles to follow:
*   **Role and Output Contract**: "You are a precise data extraction system for trucking trip sheets. Extract the information exactly as it appears. Return ONLY a valid JSON object matching the provided schema. Do not include markdown formatting or explanations."
*   **Handling Ambiguity & Missing Data**: "Do not guess or infer missing data. If a field is blank or illegible due to messy handwriting, set its value to `null` and add the field name to the `flagged_fields` array."
*   **Confidence Scoring**: Instruct the model to provide a `confidence_score` (0.0 to 1.0) based on the clarity of the image and the legibility of the handwriting.
*   **Few-Shot Prompting**: If zero-shot performance is lacking on difficult images (e.g., truck cab photos with shadows), we will introduce 1-3 few-shot examples pairing a difficult image with its expected JSON output.

### 4. Data Schema (Draft)
The core fields required, based on the MVP plan, include odometer readings and line items:
```json
{
  "type": "object",
  "properties": {
    "odometer_open": { "type": ["integer", "null"] },
    "odometer_close": { "type": ["integer", "null"] },
    "total_miles": { "type": ["integer", "null"] },
    "line_items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "date": { "type": ["string", "null"] },
          "location": { "type": ["string", "null"] },
          "miles": { "type": ["integer", "null"] }
        }
      }
    },
    "confidence_score": { "type": "number", "minimum": 0, "maximum": 1 },
    "flagged_fields": {
      "type": "array",
      "items": { "type": "string" },
      "description": "List of fields that were illegible or null."
    }
  },
  "required": ["odometer_open", "odometer_close", "total_miles", "line_items", "confidence_score", "flagged_fields"]
}
```

### 5. Benchmarking Execution Plan
1.  **Test Data Curation**: Gather a dataset of 10-15 images.
    *   *Happy Path*: 5-8 clean, flat office scans.
    *   *Edge Cases*: 5-7 simulated "truck cab" photos (shadows, skewed angles, poor lighting).
2.  **Evaluation Metrics**:
    *   **Accuracy / F1 Score**: Compare the extracted values against human-labeled ground truth for each image.
    *   **Schema Adherence**: 100% success rate required for parsing the output as valid JSON matching the schema.
    *   **Exception Handling**: Verify the model correctly populates `flagged_fields` and lowers the `confidence_score` for the intentionally messy edge-case images.
3.  **Benchmarking Script**: A Python script will iterate through the test dataset, invoke the VLM API, save the outputs, validate against the Pydantic schema, and calculate the success metrics.
