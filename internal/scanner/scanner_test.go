package scanner

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/formats"
)

func TestNewScanner(t *testing.T) {
	s3Client := &s3.Client{}
	bucket := "s3://test-bucket"
	prefix := "/logs/"
	delayWindow := 5 * time.Minute

	// Create a format registry for testing
	formatRegistry := formats.NewRegistry()

	scanner := NewScanner(s3Client, bucket, prefix, delayWindow, nil, formatRegistry)

	if scanner == nil {
		t.Fatal("NewScanner returned nil")
	}

	// Bucket should have s3:// prefix stripped
	if scanner.bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got '%s'", scanner.bucket)
	}

	// Prefix should have leading slash removed
	if scanner.prefix != "logs/" {
		t.Errorf("Expected prefix 'logs/', got '%s'", scanner.prefix)
	}

	if scanner.delayWindow != delayWindow {
		t.Errorf("Expected delayWindow %v, got %v", delayWindow, scanner.delayWindow)
	}

	if scanner.s3Client != s3Client {
		t.Error("s3Client not set correctly")
	}
}

func TestParseTimestampFromKey(t *testing.T) {
	tests := []struct {
		key       string
		expected  int64
		expectErr bool
	}{
		{
			key:       "logs/year=2024/month=1/day=1/1704067200_123_456_001.gz",
			expected:  1704067200,
			expectErr: false,
		},
		{
			key:       "logs/year=2024/month=1/day=1/1704067200_123_456_001",
			expected:  1704067200,
			expectErr: false,
		},
		{
			key:       "1704067200_123_456_001.gz",
			expected:  1704067200,
			expectErr: false,
		},
		{
			key:       "invalid_key",
			expected:  0,
			expectErr: true,
		},
		{
			key:       "not_a_timestamp_123_456_001.gz",
			expected:  0,
			expectErr: true,
		},
		{
			key:       "",
			expected:  0,
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			result, err := parseTimestampFromKey(test.key)

			if test.expectErr {
				if err == nil {
					t.Errorf("Expected error for key %s, but got none", test.key)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for key %s: %v", test.key, err)
				}
				if result != test.expected {
					t.Errorf("Expected timestamp %d, got %d", test.expected, result)
				}
			}
		})
	}
}

func TestGeneratePrefixes(t *testing.T) {
	s3Client := &s3.Client{}
	bucket := "test-bucket"
	prefix := "logs/"
	delayWindow := 5 * time.Minute

	// Create a format registry for testing
	formatRegistry := formats.NewRegistry()

	scanner := NewScanner(s3Client, bucket, prefix, delayWindow, nil, formatRegistry)

	// Test single day
	fromTimestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	toTimestamp := time.Date(2024, 1, 1, 23, 59, 59, 0, time.UTC).Unix()

	prefixes := scanner.generatePrefixes(fromTimestamp, toTimestamp)

	expected := []string{"logs/year=2024/month=1/day=1/"}
	if len(prefixes) != len(expected) {
		t.Errorf("Expected %d prefixes, got %d", len(expected), len(prefixes))
	}

	for i, exp := range expected {
		if i >= len(prefixes) || prefixes[i] != exp {
			t.Errorf("Expected prefix[%d]='%s', got '%s'", i, exp, prefixes[i])
		}
	}

	// Test multiple days
	fromTimestamp = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	toTimestamp = time.Date(2024, 1, 3, 23, 59, 59, 0, time.UTC).Unix()

	prefixes = scanner.generatePrefixes(fromTimestamp, toTimestamp)

	expected = []string{
		"logs/year=2024/month=1/day=1/",
		"logs/year=2024/month=1/day=2/",
		"logs/year=2024/month=1/day=3/",
	}

	if len(prefixes) != len(expected) {
		t.Errorf("Expected %d prefixes, got %d", len(expected), len(prefixes))
	}

	for i, exp := range expected {
		if i >= len(prefixes) || prefixes[i] != exp {
			t.Errorf("Expected prefix[%d]='%s', got '%s'", i, exp, prefixes[i])
		}
	}
}

func TestGeneratePrefixes_EmptyPrefix(t *testing.T) {
	s3Client := &s3.Client{}
	bucket := "test-bucket"
	prefix := ""
	delayWindow := 5 * time.Minute

	// Create a format registry for testing
	formatRegistry := formats.NewRegistry()

	scanner := NewScanner(s3Client, bucket, prefix, delayWindow, nil, formatRegistry)

	fromTimestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	toTimestamp := time.Date(2024, 1, 1, 23, 59, 59, 0, time.UTC).Unix()

	prefixes := scanner.generatePrefixes(fromTimestamp, toTimestamp)

	expected := []string{"year=2024/month=1/day=1/"}
	if len(prefixes) != len(expected) {
		t.Errorf("Expected %d prefixes, got %d", len(expected), len(prefixes))
	}

	if len(prefixes) > 0 && prefixes[0] != expected[0] {
		t.Errorf("Expected prefix '%s', got '%s'", expected[0], prefixes[0])
	}
}
