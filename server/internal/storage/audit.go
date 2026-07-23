package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// AuditStore handles saving raw trip sheet images for audit purposes.
type AuditStore struct {
	basePath string
}

// NewAuditStore creates a new store rooted at the given directory.
// The directory is created if it does not exist.
func NewAuditStore(basePath string) (*AuditStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit directory %s: %w", basePath, err)
	}
	return &AuditStore{basePath: basePath}, nil
}

// SaveImage writes the raw image bytes to disk, keyed by trip ID.
// Returns the file path where the image was stored.
func (s *AuditStore) SaveImage(tripID string, imageBytes []byte, mimeType string) (string, error) {
	ext := ".jpg"
	if mimeType == "image/png" {
		ext = ".png"
	}

	filename := tripID + ext
	fullPath := filepath.Join(s.basePath, filename)

	if err := os.WriteFile(fullPath, imageBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write audit image: %w", err)
	}

	return fullPath, nil
}
