package scanner

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/formats"
)

// FileJob represents a file to be processed
type FileJob struct {
	S3Key     string
	Timestamp int64
	Size      int64
}

// Scanner scans S3 for files to process
type Scanner struct {
	s3Client       *s3.Client
	bucket         string
	prefix         string
	delayWindow    time.Duration
	logFormat      formats.LogFormat // Configured format (nil for auto-detection)
	formatRegistry *formats.Registry // Registry for auto-detection
}

// NewScanner creates a new S3 scanner
func NewScanner(s3Client *s3.Client, bucket, prefix string, delayWindow time.Duration, logFormat formats.LogFormat, formatRegistry *formats.Registry) *Scanner {
	// Remove s3:// prefix from bucket if present
	bucket = strings.TrimPrefix(bucket, "s3://")

	// Remove leading slash from prefix (S3 keys don't have leading slashes)
	prefix = strings.TrimPrefix(prefix, "/")

	return &Scanner{
		s3Client:       s3Client,
		bucket:         bucket,
		prefix:         prefix,
		delayWindow:    delayWindow,
		logFormat:      logFormat,
		formatRegistry: formatRegistry,
	}
}

// Scan scans S3 for files in the given time range
func (s *Scanner) Scan(ctx context.Context, fromTimestamp int64, lastProcessedFile string) ([]FileJob, error) {
	// Calculate the time range
	now := time.Now()
	endTime := now.Add(-s.delayWindow)
	endTimestamp := endTime.Unix()

	// If fromTimestamp is 0, start from 1 minute before the delay window endpoint
	// This ensures we scan recent data while respecting the delay window
	if fromTimestamp == 0 {
		fromTimestamp = endTime.Add(-1 * time.Minute).Unix()
	}

	// Generate S3 prefixes to scan based on time range
	// Files are organized: prefix/year=YYYY/month=M/day=D/
	prefixesToScan := s.generatePrefixes(fromTimestamp, endTimestamp)

	var jobs []FileJob

	for _, prefix := range prefixesToScan {
		files, err := s.listFiles(ctx, prefix, lastProcessedFile, fromTimestamp, endTimestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to list files for prefix %s: %w", prefix, err)
		}
		jobs = append(jobs, files...)
	}

	return jobs, nil
}

// listFiles lists all files under a given prefix, using StartAfter to skip already-processed files
func (s *Scanner) listFiles(ctx context.Context, prefix string, lastProcessedFile string, fromTimestamp, endTimestamp int64) ([]FileJob, error) {
	var jobs []FileJob

	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}

	// If lastProcessedFile is in this prefix, use StartAfter to skip already-processed files
	// This optimizes scanning by using the filename timestamp to filter at the S3 API level
	if lastProcessedFile != "" && strings.HasPrefix(lastProcessedFile, prefix) {
		listInput.StartAfter = aws.String(lastProcessedFile)
	}

	paginator := s3.NewListObjectsV2Paginator(s.s3Client, listInput)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			// Parse timestamp from filename using format-specific parser
			var timestamp int64
			var err error

			if s.logFormat != nil {
				// Use configured format
				timestamp, err = s.logFormat.ParseTimestamp(*obj.Key)
			} else {
				// Auto-detection mode - try all formats
				timestamp, err = s.detectAndParseTimestamp(*obj.Key)
			}

			if err != nil {
				// Skip files we can't parse
				continue
			}

			// Filter by timestamp range (using filename timestamp)
			if timestamp < fromTimestamp || timestamp > endTimestamp {
				continue
			}

			jobs = append(jobs, FileJob{
				S3Key:     *obj.Key,
				Timestamp: timestamp,
				Size:      *obj.Size,
			})
		}
	}

	return jobs, nil
}

// generatePrefixes generates S3 prefixes for the time range
func (s *Scanner) generatePrefixes(fromTimestamp, toTimestamp int64) []string {
	var prefixes []string

	fromTime := time.Unix(fromTimestamp, 0).UTC()
	toTime := time.Unix(toTimestamp, 0).UTC()

	// Generate prefixes for each day in the range
	current := time.Date(fromTime.Year(), fromTime.Month(), fromTime.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(toTime.Year(), toTime.Month(), toTime.Day(), 23, 59, 59, 0, time.UTC)

	for current.Before(end) || current.Equal(end) {
		prefix := fmt.Sprintf("%syear=%d/month=%d/day=%d/",
			s.prefix,
			current.Year(),
			int(current.Month()),
			current.Day(),
		)
		prefixes = append(prefixes, prefix)
		current = current.Add(24 * time.Hour)
	}

	return prefixes
}

// detectAndParseTimestamp attempts to detect the format and parse timestamp
func (s *Scanner) detectAndParseTimestamp(key string) (int64, error) {
	if s.formatRegistry == nil {
		return 0, fmt.Errorf("format registry not available for auto-detection")
	}

	// Use registry's detection logic
	detectedFormat := s.formatRegistry.DetectFormat(key, nil) // No content sample available
	if detectedFormat == nil {
		return 0, fmt.Errorf("could not detect format for key: %s", key)
	}

	return detectedFormat.ParseTimestamp(key)
}

// parseTimestampFromKey extracts the Unix timestamp from S3 key
// Format: .../<timestamp>_<id>_<id>_<seq>[.gz]
func parseTimestampFromKey(key string) (int64, error) {
	// Get the filename from the key
	filename := path.Base(key)

	// Remove .gz extension if present
	filename = strings.TrimSuffix(filename, ".gz")

	// Split by underscore
	parts := strings.Split(filename, "_")
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid filename format: %s", filename)
	}

	// First part is the timestamp
	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse timestamp from %s: %w", filename, err)
	}

	return timestamp, nil
}
