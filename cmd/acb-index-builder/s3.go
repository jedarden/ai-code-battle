package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Client wraps S3 API operations for R2 and B2
type S3Client struct {
	client *s3.Client
	bucket string
}

// NewS3Client creates a new S3-compatible client
func NewS3Client(endpoint, accessKey, secretKey, bucket string) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		)),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // Use path-style URLs for R2/B2 compatibility
	})

	return &S3Client{
		client: client,
		bucket: bucket,
	}, nil
}

// listObjects lists all objects in the bucket with the given prefix
func (c *S3Client) listObjects(ctx context.Context, prefix string) ([]R2Object, error) {
	var objects []R2Object
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(c.bucket),
			Prefix: aws.String(prefix),
			ContinuationToken: continuationToken,
		}

		output, err := c.client.ListObjectsV2(ctx, input)
		if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
		}

		// Add objects to result
		for _, obj := range output.Contents {
			if obj.Key == nil || obj.LastModified == nil || obj.Size == nil {
				continue
			}

			objects = append(objects, R2Object{
				Key:          *obj.Key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
			})
		}

		// Set continuation token for next page
		continuationToken = output.NextContinuationToken
		if continuationToken == nil {
			break
		}
	}

	// Sort by LastModified (oldest first)
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.Before(objects[j].LastModified)
	})

	return objects, nil
}

// deleteObject deletes an object from the bucket
func (c *S3Client) deleteObject(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	_, err := c.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}

	return nil
}

// objectExists checks if an object exists in the bucket
func (c *S3Client) objectExists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	_, err := c.client.HeadObject(ctx, input)
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("head object %s: %w", key, err)
	}

	return true, nil
}

// uploadFile uploads a file to the bucket
func (c *S3Client) uploadFile(ctx context.Context, key string, body io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}

	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("upload object %s: %w", key, err)
	}

	return nil
}

// copyObject copies an object from another bucket (cross-account copy)
func (c *S3Client) copyObject(ctx context.Context, sourceBucket, sourceKey, destKey string) error {
	copySource := fmt.Sprintf("%s/%s", sourceBucket, sourceKey)

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(c.bucket),
		Key:        aws.String(destKey),
		CopySource: aws.String(copySource),
	}

	_, err := c.client.CopyObject(ctx, input)
	if err != nil {
		return fmt.Errorf("copy object from %s to %s: %w", sourceKey, destKey, err)
	}

	return nil
}

// downloadObject downloads an object from the bucket
func (c *S3Client) downloadObject(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	output, err := c.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("download object %s: %w", key, err)
	}

	return output.Body, nil
}

// getS3ContentType returns the content type for a file extension
func getS3ContentType(filename string) string {
	switch {
	case len(filename) >= 3 && filename[len(filename)-3:] == ".gz":
		return "application/gzip"
	case len(filename) >= 5 && filename[len(filename)-5:] == ".json":
		return "application/json"
	case len(filename) >= 4 && filename[len(filename)-4:] == ".png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}
