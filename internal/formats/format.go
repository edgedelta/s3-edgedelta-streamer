package formats

import (
	"fmt"
	"strings"

	"github.com/edgedelta/s3-edgedelta-streamer/internal/config"
)

// LogFormat defines the interface for handling different log formats
type LogFormat interface {
	// Name returns the format name (e.g., "zscaler", "cisco_umbrella")
	Name() string

	// ParseTimestamp extracts timestamp from filename
	ParseTimestamp(filename string) (int64, error)

	// ProcessContent processes a line of content (e.g., skip headers for CSV)
	// isFirstLine indicates if this is the first line of the file
	ProcessContent(line []byte, isFirstLine bool) ([]byte, error)

	// GetContentType returns the HTTP Content-Type for this format
	GetContentType() string

	// DetectFromFilename returns true if filename matches this format
	DetectFromFilename(filename string) bool

	// DetectFromContent returns true if content sample matches this format
	DetectFromContent(sample []byte) bool
}

// FormatType represents the configured log format
type FormatType string

const (
	FormatZscaler       FormatType = "zscaler"
	FormatCiscoUmbrella FormatType = "cisco_umbrella"
	FormatAuto          FormatType = "auto"
)

// Registry holds all available log formats
type Registry struct {
	formats map[string]LogFormat
}

// NewRegistry creates a new format registry with all supported formats
func NewRegistry() *Registry {
	r := &Registry{
		formats: make(map[string]LogFormat),
	}

	// Register built-in formats
	r.Register(NewZscalerFormat())
	r.Register(NewCiscoUmbrellaFormat())

	return r
}

// NewRegistryFromConfig creates a registry with custom formats from config
func NewRegistryFromConfig(formatConfigs []config.FormatConfig) *Registry {
	r := &Registry{
		formats: make(map[string]LogFormat),
	}

	// Register custom formats
	for _, cfg := range formatConfigs {
		r.Register(NewGenericFormat(cfg))
	}

	// Also register built-in formats as fallbacks
	r.Register(NewZscalerFormat())
	r.Register(NewCiscoUmbrellaFormat())

	return r
}

// Register adds a format to the registry
func (r *Registry) Register(format LogFormat) {
	r.formats[format.Name()] = format
}

// GetFormat returns a format by name
func (r *Registry) GetFormat(name string) (LogFormat, error) {
	format, exists := r.formats[name]
	if !exists {
		return nil, fmt.Errorf("unknown log format: %s", name)
	}
	return format, nil
}

// GetFormats returns all registered formats
func (r *Registry) GetFormats() map[string]LogFormat {
	return r.formats
}

// DetectFormat attempts to detect the format from filename and content
func (r *Registry) DetectFormat(filename string, contentSample []byte) LogFormat {
	// First try filename detection
	for _, format := range r.formats {
		if format.DetectFromFilename(filename) {
			return format
		}
	}

	// Fallback to content detection
	for _, format := range r.formats {
		if format.DetectFromContent(contentSample) {
			return format
		}
	}

	// Default to Zscaler for backward compatibility
	return r.formats["zscaler"]
}

// ParseFormatType converts string to FormatType
func ParseFormatType(s string) (FormatType, error) {
	switch strings.ToLower(s) {
	case "zscaler":
		return FormatZscaler, nil
	case "cisco_umbrella":
		return FormatCiscoUmbrella, nil
	case "auto":
		return FormatAuto, nil
	default:
		return "", fmt.Errorf("invalid format type: %s (must be 'zscaler', 'cisco_umbrella', or 'auto')", s)
	}
}

// String returns the string representation of FormatType
func (ft FormatType) String() string {
	return string(ft)
}
