// Package live provides R2 upload for the evolution live.json feed.
package live

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Config holds R2 configuration for live.json uploads.
type R2Config struct {
	AccessKey string
	SecretKey string
	Endpoint  string
	Bucket    string
}

// R2ConfigFromEnv loads R2 configuration from environment variables.
func R2ConfigFromEnv() *R2Config {
	return &R2Config{
		AccessKey: os.Getenv("ACB_R2_ACCESS_KEY"),
		SecretKey: os.Getenv("ACB_R2_SECRET_KEY"),
		Endpoint:  getEnvOrDefault("ACB_R2_ENDPOINT", ""),
		Bucket:    os.Getenv("ACB_R2_BUCKET"),
	}
}

// HasCredentials returns true if R2 credentials are configured.
func (c *R2Config) HasCredentials() bool {
	return c.AccessKey != "" && c.SecretKey != "" && c.Endpoint != "" && c.Bucket != ""
}

// R2Client handles R2 bucket operations for live.json uploads.
type R2Client struct {
	client   *s3.Client
	bucket   string
	endpoint string
}

// NewR2Client creates a new R2 client.
func NewR2Client(cfg *R2Config) (*R2Client, error) {
	if !cfg.HasCredentials() {
		return nil, fmt.Errorf("R2 credentials not configured")
	}

	// Create custom endpoint resolver for R2
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           cfg.Endpoint,
			SigningRegion: "auto",
		}, nil
	})

	// Load AWS config with R2 credentials
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &R2Client{
		client:   s3.NewFromConfig(awsCfg),
		bucket:   cfg.Bucket,
		endpoint: cfg.Endpoint,
	}, nil
}

// UploadLiveJSON uploads the live.json data to R2 at evolution/live.json.
// The file is served with Cache-Control: max-age=10 for near-real-time updates.
func (c *R2Client) UploadLiveJSON(ctx context.Context, data *LiveData) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String("evolution/live.json"),
		Body:         bytes.NewReader(b),
		ContentType:  aws.String("application/json"),
		CacheControl: aws.String("public, max-age=10"),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}

	return nil
}

// Upload uploads data to R2 at the specified key.
func (c *R2Client) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=10"),
	})
	return err
}

// Endpoint returns the R2 endpoint URL.
func (c *R2Client) Endpoint() string {
	return c.endpoint
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
