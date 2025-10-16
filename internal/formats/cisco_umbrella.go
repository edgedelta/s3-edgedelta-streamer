package formats

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CiscoUmbrellaFormat handles Cisco Umbrella logs (CSV format)
type CiscoUmbrellaFormat struct{}

// NewCiscoUmbrellaFormat creates a new Cisco Umbrella format handler
func NewCiscoUmbrellaFormat() *CiscoUmbrellaFormat {
	return &CiscoUmbrellaFormat{}
}

// Name returns the format name
func (f *CiscoUmbrellaFormat) Name() string {
	return "cisco_umbrella"
}

// ParseTimestamp extracts timestamp from Cisco Umbrella filename
// Format: <year>-<month>-<day>-<hour>-<minute>-<xxxx>.csv.gz
func (f *CiscoUmbrellaFormat) ParseTimestamp(filename string) (int64, error) {
	// Remove .csv.gz extension if present
	filename = strings.TrimSuffix(filename, ".csv.gz")
	filename = strings.TrimSuffix(filename, ".gz")

	// Split by dash
	parts := strings.Split(filename, "-")
	if len(parts) < 5 {
		return 0, fmt.Errorf("invalid Cisco Umbrella filename format: %s", filename)
	}

	// Parse date-time components: YYYY-MM-DD-HH-MM
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid year in filename %s: %w", filename, err)
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid month in filename %s: %w", filename, err)
	}

	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, fmt.Errorf("invalid day in filename %s: %w", filename, err)
	}

	hour, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, fmt.Errorf("invalid hour in filename %s: %w", filename, err)
	}

	minute, err := strconv.Atoi(parts[4])
	if err != nil {
		return 0, fmt.Errorf("invalid minute in filename %s: %w", filename, err)
	}

	// Create time and convert to Unix timestamp
	t := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC)
	return t.Unix(), nil
}

// ProcessContent processes a line of Cisco Umbrella content (CSV)
// Skips the header row (first line of each file)
func (f *CiscoUmbrellaFormat) ProcessContent(line []byte, isFirstLine bool) ([]byte, error) {
	// Skip header row
	if isFirstLine {
		return nil, nil
	}

	// Skip empty or whitespace-only lines
	trimmed := strings.TrimSpace(string(line))
	if len(trimmed) == 0 {
		return nil, nil
	}

	return line, nil
}

// GetContentType returns the HTTP Content-Type for Cisco Umbrella logs
func (f *CiscoUmbrellaFormat) GetContentType() string {
	return "text/plain"
}

// DetectFromFilename returns true if filename matches Cisco Umbrella pattern
func (f *CiscoUmbrellaFormat) DetectFromFilename(filename string) bool {
	// Cisco filenames: <year>-<month>-<day>-<hour>-<minute>-<xxxx>.csv[.gz]
	// Look for dash-separated parts ending with .csv or .csv.gz
	if !strings.HasSuffix(filename, ".csv") && !strings.HasSuffix(filename, ".csv.gz") {
		return false
	}

	// Remove extension and split by dash
	var base string
	if strings.HasSuffix(filename, ".csv.gz") {
		base = strings.TrimSuffix(filename, ".csv.gz")
	} else {
		base = strings.TrimSuffix(filename, ".csv")
	}
	parts := strings.Split(base, "-")

	if len(parts) < 6 { // YYYY-MM-DD-HH-MM-xxxx
		return false
	}

	// First 5 parts should be numeric
	for i := 0; i < 5; i++ {
		if _, err := strconv.Atoi(parts[i]); err != nil {
			return false
		}
	}

	return true
}

// DetectFromContent returns true if content sample matches Cisco Umbrella format
func (f *CiscoUmbrellaFormat) DetectFromContent(sample []byte) bool {
	if len(sample) == 0 {
		return false
	}

	lines := strings.Split(string(sample), "\n")
	if len(lines) < 2 { // Need at least header + one data row
		return false
	}

	// Check if first line looks like CSV (contains commas and doesn't start with { or [)
	header := strings.TrimSpace(lines[0])
	if !strings.Contains(header, ",") || strings.HasPrefix(header, "{") || strings.HasPrefix(header, "[") {
		return false
	}

	// Check if header looks like log field names
	// Common Cisco Umbrella headers contain words like "timestamp", "domain", "action", etc.
	headerLower := strings.ToLower(header)
	ciscoKeywords := []string{"timestamp", "domain", "action", "identity", "categories"}

	matches := 0
	for _, keyword := range ciscoKeywords {
		if strings.Contains(headerLower, keyword) {
			matches++
		}
	}

	// Also check that at least one data line looks like CSV
	if len(lines) >= 2 {
		dataLine := strings.TrimSpace(lines[1])
		if !strings.Contains(dataLine, ",") || strings.HasPrefix(dataLine, "{") || strings.HasPrefix(dataLine, "[") {
			return false
		}
	}

	return matches >= 2 // At least 2 matching keywords suggest Cisco format
}
