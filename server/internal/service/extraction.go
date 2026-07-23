package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/trucking-poc/server/internal/domain"
)

const systemPrompt = `You are a precise data extraction system for trucking trip sheets.

RULES:
1. Extract the information EXACTLY as it appears on the document.
2. Return ONLY a single valid JSON object — no markdown fences, no explanation.
3. Do NOT guess or infer missing data. If a field is blank or illegible, set its value to null and add the field name to the "flagged_fields" array.
4. Provide a "confidence_score" between 0.0 and 1.0 reflecting overall legibility.

REQUIRED JSON SCHEMA:
{
  "odometer_open": <int or null>,
  "odometer_close": <int or null>,
  "total_miles": <int or null>,
  "line_items": [
    {"date": "<string or null>", "location": "<string or null>", "miles": <int or null>}
  ],
  "confidence_score": <float 0.0-1.0>,
  "flagged_fields": ["<field_name>", ...]
}`

// ExtractionService handles communication with the Gemini VLM API.
type ExtractionService struct {
	client *genai.Client
}

// NewExtractionService creates a new service with an initialized Gemini client.
func NewExtractionService(ctx context.Context) (*ExtractionService, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &ExtractionService{
		client: client,
	}, nil
}

// Close releases the Gemini client resources.
func (s *ExtractionService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// ExtractFromImage sends image bytes to the Gemini VLM and returns a parsed TripSheet.
func (s *ExtractionService) ExtractFromImage(ctx context.Context, imageBytes []byte, mimeType string, modelName string) (*domain.TripSheet, error) {
	if modelName == "" {
		modelName = "gemini-3.5-flash-lite"
	}

	// Initialize the model dynamically on this request
	model := s.client.GenerativeModel(modelName)
	model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	model.ResponseMIMEType = "application/json"
	model.SetTemperature(0.0)

	// genai.ImageData expects format ("jpeg", "png"), not full MIME type ("image/jpeg")
	format := strings.TrimPrefix(mimeType, "image/")
	imgData := genai.ImageData(format, imageBytes)

	resp, err := model.GenerateContent(ctx,
		imgData,
		genai.Text("Extract the trip sheet data from this image. Return ONLY valid JSON."),
	)
	if err != nil {
		return nil, fmt.Errorf("Gemini API call failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini API")
	}

	// Extract the text content from the response
	textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("unexpected response part type from Gemini API")
	}

	// Unmarshal JSON into the domain struct
	var tripSheet domain.TripSheet
	if err := json.Unmarshal([]byte(textPart), &tripSheet); err != nil {
		return nil, fmt.Errorf("failed to parse VLM response as TripSheet: %w\nRaw output: %s", err, string(textPart))
	}

	return &tripSheet, nil
}
