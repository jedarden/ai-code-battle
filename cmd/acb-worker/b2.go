// B2 client for uploading replays to Backblaze B2 (cold archive)
package main

import (
	"bytes"
	"context"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
)

// B2Client handles B2 bucket operations (S3-compatible).
type B2Client struct {
	client   *s3.Client
	bucket   string
	endpoint string
}

// NewB2Client creates a new B2 client.
func NewB2Client(cfg *Config) *B2Client {
	// Parse and validate the custom endpoint
	endpointURL, err := url.Parse(cfg.B2Endpoint)
	if err != nil {
		panic(fmt.Sprintf("failed to parse B2 endpoint: %v", err))
	}

	// Load AWS config with B2 credentials
	// For S3-compatible endpoints (ARMOR/B2), the region is not used
	// but must be set to a valid value for the SDK
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.B2AccessKey,
			cfg.B2SecretKey,
			"",
		)),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to load AWS config: %v", err))
	}

	// Create S3 client with custom endpoint (ARMOR proxy wrapping B2)
	// UsePathStyle is required: without it the SDK uses virtual-hosted style and
	// drops the bucket name from the URL path (bucket ends up in hostname which
	// the custom resolver replaces, losing it entirely).
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.EndpointResolverV2 = &customEndpointResolver{
			endpointURL: *endpointURL,
		}
		o.UsePathStyle = true
	})

	return &B2Client{
		client:   client,
		bucket:   cfg.B2Bucket,
		endpoint: cfg.B2Endpoint,
	}
}

// Upload uploads data to B2. Pass contentEncoding="" for uncompressed objects.
func (c *B2Client) Upload(ctx context.Context, key string, data []byte, contentType string, contentEncoding string) error {
	input := &s3.PutObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	}
	if contentEncoding != "" {
		input.ContentEncoding = aws.String(contentEncoding)
	}
	_, err := c.client.PutObject(ctx, input)
	return err
}

// Download downloads data from B2.
func (c *B2Client) Download(ctx context.Context, key string) ([]byte, error) {
	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Delete deletes an object from B2.
func (c *B2Client) Delete(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

// List lists objects with a prefix.
func (c *B2Client) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}

// Endpoint returns the B2 endpoint URL.
func (c *B2Client) Endpoint() string {
	return c.endpoint
}

// customEndpointResolver implements s3.EndpointResolverV2 for custom S3-compatible endpoints.
type customEndpointResolver struct {
	endpointURL url.URL
}

func (r *customEndpointResolver) ResolveEndpoint(ctx context.Context, params s3.EndpointParameters) (smithyendpoints.Endpoint, error) {
	return smithyendpoints.Endpoint{
		URI: r.endpointURL,
	}, nil
}
