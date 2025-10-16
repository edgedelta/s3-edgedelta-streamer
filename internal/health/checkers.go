package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/config"
	"github.com/redis/go-redis/v9"
)

// S3HealthChecker checks S3 connectivity
type S3HealthChecker struct {
	client *s3.Client
	bucket string
}

// NewS3HealthChecker creates a new S3 health checker
func NewS3HealthChecker(client *s3.Client, bucket string) *S3HealthChecker {
	return &S3HealthChecker{
		client: client,
		bucket: bucket,
	}
}

// Check performs the S3 health check
func (c *S3HealthChecker) Check(ctx context.Context) error {
	// Try to list objects with a short timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	})

	if err != nil {
		return fmt.Errorf("S3 bucket access failed: %w", err)
	}

	return nil
}

// Name returns the checker name
func (c *S3HealthChecker) Name() string {
	return "s3"
}

// HTTPHealthChecker checks HTTP endpoint connectivity
type HTTPHealthChecker struct {
	endpoint string
	client   *http.Client
}

// NewHTTPHealthChecker creates a new HTTP health checker
func NewHTTPHealthChecker(endpoint string) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Check performs the HTTP health check
func (c *HTTPHealthChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", c.endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	return nil
}

// Name returns the checker name
func (c *HTTPHealthChecker) Name() string {
	return "http"
}

// RedisHealthChecker checks Redis connectivity
type RedisHealthChecker struct {
	client *redis.Client
}

// NewRedisHealthChecker creates a new Redis health checker
func NewRedisHealthChecker(redisConfig config.RedisConfig) *RedisHealthChecker {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port),
		Password: redisConfig.Password,
		DB:       redisConfig.Database,
	})

	return &RedisHealthChecker{
		client: client,
	}
}

// Check performs the Redis health check
func (c *RedisHealthChecker) Check(ctx context.Context) error {
	// Try to ping Redis with a short timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	return nil
}

// Name returns the checker name
func (c *RedisHealthChecker) Name() string {
	return "redis"
}

// BasicHealthChecker provides a simple always-healthy check
type BasicHealthChecker struct{}

// NewBasicHealthChecker creates a basic health checker
func NewBasicHealthChecker() *BasicHealthChecker {
	return &BasicHealthChecker{}
}

// Check always returns healthy
func (c *BasicHealthChecker) Check(ctx context.Context) error {
	return nil
}

// Name returns the checker name
func (c *BasicHealthChecker) Name() string {
	return "basic"
}
