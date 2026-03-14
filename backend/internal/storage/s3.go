package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/yourusername/media-share/config"
)

type S3Client struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	cdnBase   string
}

func NewS3Client(cfg config.AWSConfig) (*S3Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)
	return &S3Client{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.BucketName,
		cdnBase:   cfg.CDNBaseURL,
	}, nil
}

// PresignPut generates a presigned PUT URL for direct client upload.
func (c *S3Client) PresignPut(ctx context.Context, key, contentType string, sizeBytes int64, ttl time.Duration) (string, error) {
	req, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		// ContentLength intentionally omitted — including it adds content-length
		// to SignedHeaders which breaks browser CORS preflight checks.
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return req.URL, nil
}

// PresignGet generates a presigned GET URL for private object access.
func (c *S3Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return req.URL, nil
}

// CDNUrl returns the CDN URL for a key if CDN_BASE_URL is set,
// otherwise falls back to a direct S3 HTTPS URL.
// Swap to CloudFront later by simply setting CDN_BASE_URL in .env.
func (c *S3Client) CDNUrl(key string) string {
	if c.cdnBase != "" {
		return c.cdnBase + "/" + key
	}
	// Direct S3 URL (public-read objects only)
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", c.bucket, key)
}

// HeadObject checks if an object exists.
func (c *S3Client) HeadObject(ctx context.Context, key string) error {
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

// GetObject downloads an object's contents.
func (c *S3Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

// PutObject uploads an object.
func (c *S3Client) PutObject(ctx context.Context, key, contentType string, body io.Reader) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Body:        body,
	})
	return err
}

// DeleteObject removes an object from S3.
func (c *S3Client) DeleteObject(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}
