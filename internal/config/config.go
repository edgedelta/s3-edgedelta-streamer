package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FormatConfig defines a custom log format configuration
type FormatConfig struct {
	Name            string `yaml:"name"`              // Format name (e.g., "zscaler", "cisco_umbrella")
	FilenamePattern string `yaml:"filename_pattern"`  // Glob pattern for matching files (e.g., "*.json.gz")
	TimestampRegex  string `yaml:"timestamp_regex"`   // Regex with capture group for timestamp extraction
	TimestampFormat string `yaml:"timestamp_format"`  // Timestamp format: "unix", "unix_ms", or Go time layout
	ContentType     string `yaml:"content_type"`      // HTTP Content-Type header
	SkipHeaderLines int    `yaml:"skip_header_lines"` // Number of header lines to skip (0 = no headers)
	FieldSeparator  string `yaml:"field_separator"`   // Field separator for CSV-like formats (default: ",")
}

// RedisConfig holds Redis connection and state configuration
type RedisConfig struct {
	Enabled   bool   `yaml:"enabled"`    // Enable Redis state storage
	Host      string `yaml:"host"`       // Redis host (default: "localhost")
	Port      int    `yaml:"port"`       // Redis port (default: 6379)
	Password  string `yaml:"password"`   // Redis password (optional)
	Database  int    `yaml:"database"`   // Redis database number (default: 0)
	KeyPrefix string `yaml:"key_prefix"` // Key prefix for state keys (default: "s3-streamer")
}

// Config holds the application configuration
type Config struct {
	S3 struct {
		Bucket string `yaml:"bucket"`
		Prefix string `yaml:"prefix"`
		Region string `yaml:"region"`
	} `yaml:"s3"`

	HTTP struct {
		Endpoints             []string      `yaml:"endpoints"`               // EdgeDelta HTTP input endpoints (load balanced across workers)
		BatchLines            int           `yaml:"batch_lines"`             // Max lines per batch (default: 1000)
		BatchBytes            int           `yaml:"batch_bytes"`             // Max bytes per batch (default: 1MB)
		FlushInterval         time.Duration `yaml:"flush_interval"`          // Force flush after this duration (default: 1s)
		Workers               int           `yaml:"workers"`                 // Number of parallel HTTP senders (default: 10)
		BufferSize            int           `yaml:"buffer_size"`             // Size of line buffer (default: 10000)
		Timeout               time.Duration `yaml:"timeout"`                 // HTTP request timeout (default: 30s)
		MaxIdleConns          int           `yaml:"max_idle_conns"`          // HTTP connection pool size (default: 100)
		IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`       // How long idle connections stay alive (default: 90s)
		TLSHandshakeTimeout   time.Duration `yaml:"tls_handshake_timeout"`   // TLS handshake timeout (default: 10s)
		ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout"` // Response header timeout (default: 10s)
		ExpectContinueTimeout time.Duration `yaml:"expect_continue_timeout"` // Expect continue timeout (default: 1s)
	} `yaml:"http"`

	Processing struct {
		WorkerCount   int            `yaml:"worker_count"`
		QueueSize     int            `yaml:"queue_size"`
		ScanInterval  time.Duration  `yaml:"scan_interval"`
		DelayWindow   time.Duration  `yaml:"delay_window"`
		LogFormats    []FormatConfig `yaml:"log_formats"`    // Custom format definitions
		DefaultFormat string         `yaml:"default_format"` // Default format name or "auto"
		LogFormat     string         `yaml:"log_format"`     // DEPRECATED: Legacy single format field
	} `yaml:"processing"`

	State struct {
		FilePath     string        `yaml:"file_path"`
		SaveInterval time.Duration `yaml:"save_interval"`
		Redis        RedisConfig   `yaml:"redis"` // Redis configuration for state storage
	} `yaml:"state"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`

	OTLP struct {
		Enabled        bool          `yaml:"enabled"`         // Enable OTLP metrics export
		Endpoint       string        `yaml:"endpoint"`        // OTLP gRPC endpoint (e.g., "localhost:4317")
		ExportInterval time.Duration `yaml:"export_interval"` // How often to export metrics (default: 10s)
		ServiceName    string        `yaml:"service_name"`    // Service name for metrics (default: "s3-edgedelta-streamer")
		ServiceVersion string        `yaml:"service_version"` // Service version
		Insecure       bool          `yaml:"insecure"`        // Use insecure connection (no TLS)
	} `yaml:"otlp"`

	Health struct {
		Enabled bool   `yaml:"enabled"` // Enable health check server
		Address string `yaml:"address"` // Health check server address (default: ":8080")
		Path    string `yaml:"path"`    // Health check path (default: "/health")
	} `yaml:"health"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// Validate checks the configuration for required fields and valid values
func (c *Config) Validate() error {
	var errs []string

	// Validate S3 configuration
	if c.S3.Bucket == "" {
		errs = append(errs, "s3.bucket is required")
	}
	if c.S3.Region == "" {
		errs = append(errs, "s3.region is required")
	}

	// Validate HTTP configuration
	if len(c.HTTP.Endpoints) == 0 {
		errs = append(errs, "http.endpoints must contain at least one endpoint")
	}
	for i, endpoint := range c.HTTP.Endpoints {
		if endpoint == "" {
			errs = append(errs, fmt.Sprintf("http.endpoints[%d] cannot be empty", i))
			continue
		}
		if parsed, err := url.Parse(endpoint); err != nil {
			errs = append(errs, fmt.Sprintf("http.endpoints[%d] is not a valid URL: %v", i, err))
		} else if parsed.Scheme != "http" && parsed.Scheme != "https" {
			errs = append(errs, fmt.Sprintf("http.endpoints[%d] must use http or https scheme", i))
		}
	}

	// Validate batch settings
	if c.HTTP.BatchLines <= 0 {
		errs = append(errs, "http.batch_lines must be greater than 0")
	}
	if c.HTTP.BatchBytes <= 0 {
		errs = append(errs, "http.batch_bytes must be greater than 0")
	}
	if c.HTTP.BatchBytes > 10*1024*1024 { // 10MB limit
		errs = append(errs, "http.batch_bytes cannot exceed 10MB")
	}

	// Validate buffer settings
	if c.HTTP.BufferSize <= 0 {
		errs = append(errs, "http.buffer_size must be greater than 0")
	}
	if c.HTTP.BufferSize > 100000 { // 100K limit to prevent excessive memory usage
		errs = append(errs, "http.buffer_size cannot exceed 100,000")
	}

	// Validate worker settings
	if c.HTTP.Workers <= 0 {
		errs = append(errs, "http.workers must be greater than 0")
	}
	if c.Processing.WorkerCount <= 0 {
		errs = append(errs, "processing.worker_count must be greater than 0")
	}

	// Validate timing settings
	if c.HTTP.FlushInterval <= 0 {
		errs = append(errs, "http.flush_interval must be greater than 0")
	}
	if c.Processing.DelayWindow <= 0 {
		errs = append(errs, "processing.delay_window must be greater than 0")
	}
	if c.Processing.ScanInterval <= 0 {
		errs = append(errs, "processing.scan_interval must be greater than 0")
	}

	// Validate log format configuration
	if len(c.Processing.LogFormats) > 0 {
		// New format: validate custom formats
		for i, format := range c.Processing.LogFormats {
			if format.Name == "" {
				errs = append(errs, fmt.Sprintf("processing.log_formats[%d].name is required", i))
			}
			if format.FilenamePattern == "" {
				errs = append(errs, fmt.Sprintf("processing.log_formats[%d].filename_pattern is required", i))
			}
			if format.TimestampRegex == "" {
				errs = append(errs, fmt.Sprintf("processing.log_formats[%d].timestamp_regex is required", i))
			}
			if format.TimestampFormat == "" {
				format.TimestampFormat = "unix" // Default
			}
			if format.ContentType == "" {
				format.ContentType = "text/plain" // Default
			}
			// Update the format in the slice
			c.Processing.LogFormats[i] = format
		}

		// Validate default_format
		if c.Processing.DefaultFormat == "" {
			c.Processing.DefaultFormat = "auto"
		}

	} else if c.Processing.LogFormat != "" {
		// Legacy format: validate old single format field
		validFormats := []string{"zscaler", "cisco_umbrella", "auto"}
		valid := false
		for _, format := range validFormats {
			if c.Processing.LogFormat == format {
				valid = true
				break
			}
		}
		if !valid {
			errs = append(errs, "processing.log_format must be one of: zscaler, cisco_umbrella, auto")
		}

		// Set default format for backward compatibility
		if c.Processing.DefaultFormat == "" {
			c.Processing.DefaultFormat = c.Processing.LogFormat
		}

	} else {
		// No format specified: use defaults
		c.Processing.DefaultFormat = "zscaler" // Backward compatibility
	}

	// Validate OTLP configuration if enabled
	if c.OTLP.Enabled {
		if c.OTLP.Endpoint == "" {
			errs = append(errs, "otlp.endpoint is required when otlp.enabled is true")
		}
		if c.OTLP.ServiceName == "" {
			errs = append(errs, "otlp.service_name is required when otlp.enabled is true")
		}
		if c.OTLP.ExportInterval <= 0 {
			errs = append(errs, "otlp.export_interval must be greater than 0")
		}
	}

	// Validate Redis configuration if enabled
	if c.State.Redis.Enabled {
		if c.State.Redis.Host == "" {
			c.State.Redis.Host = "localhost" // Default
		}
		if c.State.Redis.Port == 0 {
			c.State.Redis.Port = 6379 // Default Redis port
		}
		if c.State.Redis.KeyPrefix == "" {
			c.State.Redis.KeyPrefix = "s3-streamer" // Default key prefix
		}
		if c.State.Redis.Database < 0 || c.State.Redis.Database > 15 {
			errs = append(errs, "state.redis.database must be between 0 and 15")
		}
	}

	// Validate logging configuration
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[strings.ToLower(c.Logging.Level)] {
		errs = append(errs, "logging.level must be one of: debug, info, warn, error")
	}
	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[strings.ToLower(c.Logging.Format)] {
		errs = append(errs, "logging.format must be one of: json, text")
	}

	if len(errs) > 0 {
		return errors.New("configuration validation failed:\n" + strings.Join(errs, "\n"))
	}

	return nil
}
