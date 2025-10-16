package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	configContent := `
s3:
  bucket: "test-bucket"
  prefix: "test-prefix"
  region: "us-east-1"

http:
  endpoints:
    - "http://localhost:8080"
  batch_lines: 1000
  batch_bytes: 1048576
  flush_interval: 1s
  workers: 10
  buffer_size: 50000
  timeout: 30s
  max_idle_conns: 100
  idle_conn_timeout: 90s
  tls_handshake_timeout: 10s
  response_header_timeout: 10s
  expect_continue_timeout: 1s

processing:
  worker_count: 15
  queue_size: 1000
  scan_interval: 15s
  delay_window: 60s

state:
  file_path: "/tmp/state.json"
  save_interval: 30s

logging:
  level: "info"
  format: "json"

otlp:
  enabled: false
  endpoint: "localhost:4317"
  export_interval: 10s
  service_name: "test-service"
  service_version: "1.0.0"
  insecure: true

health:
  enabled: true
  address: ":8080"
  path: "/health"
`

	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	// Test loading the config
	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config values
	if cfg.S3.Bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got '%s'", cfg.S3.Bucket)
	}
	if cfg.HTTP.Workers != 10 {
		t.Errorf("Expected workers 10, got %d", cfg.HTTP.Workers)
	}
	if cfg.Health.Enabled != true {
		t.Errorf("Expected health enabled true, got %v", cfg.Health.Enabled)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				S3: struct {
					Bucket string `yaml:"bucket"`
					Prefix string `yaml:"prefix"`
					Region string `yaml:"region"`
				}{
					Bucket: "test-bucket",
					Region: "us-east-1",
				},
				HTTP: struct {
					Endpoints             []string      `yaml:"endpoints"`
					BatchLines            int           `yaml:"batch_lines"`
					BatchBytes            int           `yaml:"batch_bytes"`
					FlushInterval         time.Duration `yaml:"flush_interval"`
					Workers               int           `yaml:"workers"`
					BufferSize            int           `yaml:"buffer_size"`
					Timeout               time.Duration `yaml:"timeout"`
					MaxIdleConns          int           `yaml:"max_idle_conns"`
					IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`
					TLSHandshakeTimeout   time.Duration `yaml:"tls_handshake_timeout"`
					ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout"`
					ExpectContinueTimeout time.Duration `yaml:"expect_continue_timeout"`
				}{
					Endpoints:     []string{"http://localhost:8080"},
					BatchLines:    1000,
					BatchBytes:    1048576,
					FlushInterval: time.Second,
					Workers:       10,
					BufferSize:    50000,
					Timeout:       30 * time.Second,
					MaxIdleConns:  100,
				},
				Processing: struct {
					WorkerCount   int            `yaml:"worker_count"`
					QueueSize     int            `yaml:"queue_size"`
					ScanInterval  time.Duration  `yaml:"scan_interval"`
					DelayWindow   time.Duration  `yaml:"delay_window"`
					LogFormats    []FormatConfig `yaml:"log_formats"`
					DefaultFormat string         `yaml:"default_format"`
					LogFormat     string         `yaml:"log_format"`
				}{
					WorkerCount:  5,
					QueueSize:    1000,
					ScanInterval: 15 * time.Second,
					DelayWindow:  60 * time.Second,
				},
				State: struct {
					FilePath     string        `yaml:"file_path"`
					SaveInterval time.Duration `yaml:"save_interval"`
					Redis        RedisConfig   `yaml:"redis"`
				}{
					FilePath:     "/tmp/state.json",
					SaveInterval: 30 * time.Second,
				},
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid buffer size - too small",
			config: Config{
				HTTP: struct {
					Endpoints             []string      `yaml:"endpoints"`
					BatchLines            int           `yaml:"batch_lines"`
					BatchBytes            int           `yaml:"batch_bytes"`
					FlushInterval         time.Duration `yaml:"flush_interval"`
					Workers               int           `yaml:"workers"`
					BufferSize            int           `yaml:"buffer_size"`
					Timeout               time.Duration `yaml:"timeout"`
					MaxIdleConns          int           `yaml:"max_idle_conns"`
					IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`
					TLSHandshakeTimeout   time.Duration `yaml:"tls_handshake_timeout"`
					ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout"`
					ExpectContinueTimeout time.Duration `yaml:"expect_continue_timeout"`
				}{
					BufferSize: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid buffer size - too large",
			config: Config{
				HTTP: struct {
					Endpoints             []string      `yaml:"endpoints"`
					BatchLines            int           `yaml:"batch_lines"`
					BatchBytes            int           `yaml:"batch_bytes"`
					FlushInterval         time.Duration `yaml:"flush_interval"`
					Workers               int           `yaml:"workers"`
					BufferSize            int           `yaml:"buffer_size"`
					Timeout               time.Duration `yaml:"timeout"`
					MaxIdleConns          int           `yaml:"max_idle_conns"`
					IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`
					TLSHandshakeTimeout   time.Duration `yaml:"tls_handshake_timeout"`
					ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout"`
					ExpectContinueTimeout time.Duration `yaml:"expect_continue_timeout"`
				}{
					BufferSize: 200000,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
