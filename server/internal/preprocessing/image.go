package preprocessing

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"fmt"

	"github.com/disintegration/imaging"
)

// EnhanceImage applies grayscale conversion and contrast boost to improve
// OCR quality on low-quality mobile photos. Returns the processed image
// as JPEG bytes.
func EnhanceImage(imageBytes []byte, mimeType string) ([]byte, string, error) {
	var img image.Image
	var err error

	reader := bytes.NewReader(imageBytes)

	switch mimeType {
	case "image/jpeg":
		img, err = jpeg.Decode(reader)
	case "image/png":
		img, err = png.Decode(reader)
	default:
		return nil, "", fmt.Errorf("unsupported image type: %s", mimeType)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Convert to grayscale — reduces color noise
	img = imaging.Grayscale(img)

	// Boost contrast — improves text legibility on shadowed/dark images
	img = imaging.AdjustContrast(img, 30)

	// Sharpen slightly to improve edge definition on blurry photos
	img = imaging.Sharpen(img, 1.0)

	// Encode back to JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, "", fmt.Errorf("failed to encode processed image: %w", err)
	}

	return buf.Bytes(), "image/jpeg", nil
}
