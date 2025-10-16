package formats

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/edgedelta/s3-edgedelta-streamer/internal/config"
)

// GenericFormat implements LogFormat using configurable patterns
type GenericFormat struct {
	config config.FormatConfig
}

// NewGenericFormat creates a new generic format handler from config
func NewGenericFormat(config config.FormatConfig) *GenericFormat {
	// Set defaults
	if config.ContentType == "" {
		config.ContentType = "text/plain"
	}
	if config.FieldSeparator == "" {
		config.FieldSeparator = ","
	}

	return &GenericFormat{config: config}
}

// Name returns the format name
func (f *GenericFormat) Name() string {
	return f.config.Name
}

// ParseTimestamp extracts timestamp from filename using regex pattern
func (f *GenericFormat) ParseTimestamp(filename string) (int64, error) {
	re, err := regexp.Compile(f.config.TimestampRegex)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp regex for format %s: %w", f.config.Name, err)
	}

	matches := re.FindStringSubmatch(filename)
	if len(matches) < 2 {
		return 0, fmt.Errorf("timestamp regex did not match filename: %s", filename)
	}

	timestampStr := matches[1] // First capture group

	switch f.config.TimestampFormat {
	case "unix":
		return strconv.ParseInt(timestampStr, 10, 64)
	case "unix_ms":
		ms, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			return 0, err
		}
		return ms / 1000, nil
	default:
		// Custom Go time layout
		t, err := time.Parse(f.config.TimestampFormat, timestampStr)
		if err != nil {
			return 0, fmt.Errorf("failed to parse timestamp %s with layout %s: %w", timestampStr, f.config.TimestampFormat, err)
		}
		return t.Unix(), nil
	}
}

// ProcessContent processes content according to format rules
func (f *GenericFormat) ProcessContent(line []byte, isFirstLine bool) ([]byte, error) {
	// Skip header lines
	if isFirstLine && f.config.SkipHeaderLines > 0 {
		return nil, nil
	}

	// Skip empty lines
	lineStr := strings.TrimSpace(string(line))
	if lineStr == "" {
		return nil, nil
	}

	return line, nil
}

// GetContentType returns the configured content type
func (f *GenericFormat) GetContentType() string {
	return f.config.ContentType
}

// DetectFromFilename checks if filename matches the pattern
func (f *GenericFormat) DetectFromFilename(filename string) bool {
	matched, err := filepath.Match(f.config.FilenamePattern, filename)
	return err == nil && matched
}

// DetectFromContent performs basic content detection
func (f *GenericFormat) DetectFromContent(sample []byte) bool {
	if len(sample) == 0 {
		return false
	}

	lines := strings.Split(string(sample), "\n")
	if len(lines) < 1 {
		return false
	}

	// Basic validation based on content type
	switch f.config.ContentType {
	case "application/x-ndjson", "application/json":
		// Check if first non-empty line looks like JSON
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			return strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}")
		}
	case "text/plain":
		// For plain text, just check we have content
		return len(strings.TrimSpace(string(sample))) > 0
	}

	return true // Default to accepting
}
