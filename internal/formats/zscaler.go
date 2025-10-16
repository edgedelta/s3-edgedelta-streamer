package formats

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ZscalerFormat handles Zscaler NSS web logs (JSONL format)
type ZscalerFormat struct{}

// NewZscalerFormat creates a new Zscaler format handler
func NewZscalerFormat() *ZscalerFormat {
	return &ZscalerFormat{}
}

// Name returns the format name
func (f *ZscalerFormat) Name() string {
	return "zscaler"
}

// ParseTimestamp extracts Unix timestamp from Zscaler filename
// Format: <unix_timestamp>_<id>_<id>_<seq>[.gz]
func (f *ZscalerFormat) ParseTimestamp(filename string) (int64, error) {
	// Remove .gz extension if present
	filename = strings.TrimSuffix(filename, ".gz")

	// Split by underscore
	parts := strings.Split(filename, "_")
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid Zscaler filename format: %s", filename)
	}

	// First part is the timestamp
	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse timestamp from Zscaler filename %s: %w", filename, err)
	}

	return timestamp, nil
}

// ProcessContent processes a line of Zscaler content (JSONL)
// For Zscaler, we pass through all lines as-is
func (f *ZscalerFormat) ProcessContent(line []byte, isFirstLine bool) ([]byte, error) {
	// Zscaler logs are already in JSONL format, no processing needed
	// But we should validate it's valid JSON
	if len(line) == 0 {
		return nil, nil // Skip empty lines
	}

	// Basic JSON validation (optional, but good practice)
	trimmed := strings.TrimSpace(string(line))
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var jsonTest interface{}
		if err := json.Unmarshal(line, &jsonTest); err != nil {
			return nil, fmt.Errorf("invalid JSON in Zscaler log line: %w", err)
		}
	}

	return line, nil
}

// GetContentType returns the HTTP Content-Type for Zscaler logs
func (f *ZscalerFormat) GetContentType() string {
	return "application/x-ndjson"
}

// DetectFromFilename returns true if filename matches Zscaler pattern
func (f *ZscalerFormat) DetectFromFilename(filename string) bool {
	// Zscaler filenames: <unix_timestamp>_<id>_<id>_<seq>[.gz]
	// Look for underscore-separated parts where first part is numeric timestamp
	filename = strings.TrimSuffix(filename, ".gz")
	parts := strings.Split(filename, "_")

	if len(parts) < 4 {
		return false
	}

	// First part should be numeric Unix timestamp
	_, err := strconv.ParseInt(parts[0], 10, 64)
	return err == nil
}

// DetectFromContent returns true if content sample matches Zscaler format
func (f *ZscalerFormat) DetectFromContent(sample []byte) bool {
	if len(sample) == 0 {
		return false
	}

	// Zscaler logs are JSON objects, one per line
	lines := strings.Split(string(sample), "\n")
	if len(lines) == 0 {
		return false
	}

	// Check if first non-empty line looks like JSON
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Should start and end with braces
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			var jsonTest interface{}
			return json.Unmarshal([]byte(line), &jsonTest) == nil
		}
		break // Only check first non-empty line
	}

	return false
}
