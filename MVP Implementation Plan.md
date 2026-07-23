# **MVP Implementation Plan: Trucking Trip Sheet Automation**

This plan outlines the steps to build the POC, architected as a headless, API-first service. By starting with the VLM extraction, we front-load the highest-risk technical component (the AI) and build the deterministic backend around it.

## **Phase 1: The AI Extraction Core (VLM \-\> JSON)**

This phase isolates the VLM to ensure we can consistently get structured, schema-compliant JSON before writing any complex backend logic.

> 1. **Test Data Curation:**  
   * Gather a baseline set of 10–15 sample trip sheets.  
   * Include a mix of clean, flat office scans (the primary happy path) and a few simulated "truck cab" photos (shadows, slight angles) to establish a baseline for the AI's OCR capabilities.  
> 2. **Prompt Engineering & Schema Binding:**  
   * Draft the system prompt instructing the model on its role (e.g., "You are a precise data extraction system for trucking trip sheets. Do not guess or infer missing data. If a field is illegible, set its value to null and flag it.").  
   * Bind the strict JSON Schema to the VLM's API call using structured output mode to guarantee the model cannot hallucinate keys or return malformed JSON.  
> 3. **VLM Benchmarking:**  
   * Write an isolated script to run the sample images through the VLM.  
   * Measure the F1 score, specifically tracking how often the model correctly calculates the confidence score and properly utilizes the flagged\_fields array when it encounters messy handwriting.

## **Phase 2: The Go Ingestion & Validation API**

Once the VLM consistently outputs valid JSON, we wrap it in a Go service to enforce business rules deterministically.

> 1. **Ingestion Endpoint:** Create the primary POST /api/v1/trips/extract endpoint accepting a multipart form payload (the scanned image).  
> 2. **Lightweight Preprocessing:** Implement a basic image assessment layer. If the image is a mobile photo, apply a fast grayscale and contrast boost before encoding it to base64 for the VLM.  
> 3. **Struct Unmarshaling:** Map the VLM's JSON response directly into the Go TripSheet struct.  
> 4. **Deterministic Cross-Checks (The Guardrails):**  
   * Use go-playground/validator to enforce required fields and data types.  
   * Implement the arithmetic business logic: Verify that Odometer Close \- Odometer Open ≈ Total Miles, and cross-check the sum of the line-item miles.  
   * **Routing Logic:** If validation passes and all field confidence scores are above the threshold (e.g., \> 0.85), mark the payload as Status: Validated. If math fails or confidence is low, mark as Status: Exception.

## **Phase 3: Persistence (Postgres)**

With the data extracted and validated, it moves to the persistence layer.

> 1. **Schema Migration:** Set up the Postgres database with tables for trips, trip\_line\_items, and exceptions.  
> 2. **Transaction Handling:** Write the Go repository logic to execute an atomic transaction:  
   * Insert the parent record into the trips table.  
   * Bulk insert the row data into trip\_line\_items.  
> 3. **Audit Storage:** Save the raw image file to an object store (like an AWS S3 bucket or local directory) and save the file path alongside the database record. This is strictly required so a human can view the original image if the trip is routed to the exception queue.

## **Phase 4: The TMS/Accounting Hand-off (POC Goal)**

The final step of the MVP proves the value by moving the data out of our silo and into the downstream systems.

> 1. **Payload Transformation:** Map the validated Postgres trips data into the specific JSON shapes required by the company's TMS and Accounting APIs.  
> 2. **Simulated Export:** For the POC, create a GET /api/v1/trips/export endpoint (or a simulated webhook) that outputs the final, sanitized payload. This demonstrates the end-to-end flow: physical paper in, validated payroll and dispatch data out.