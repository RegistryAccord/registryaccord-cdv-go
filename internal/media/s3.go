// internal/media/s3.go
// Package media provides S3-compatible storage implementation for media assets.
// It handles presigned URL generation and object verification for secure media operations.
package media

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client wraps the AWS S3 client for media operations.
// It provides methods for generating presigned URLs and verifying media objects.
type S3Client struct {
	client *s3.Client // AWS S3 client
	bucket string     // S3 bucket name for media storage
}

// NewS3Client creates a new S3 client for media operations.
// It supports both AWS S3 and S3-compatible services like MinIO.
// Parameters:
//   - endpoint: S3 service endpoint URL
//   - region: AWS region (or equivalent for S3-compatible services)
//   - bucket: S3 bucket name for media storage
//   - accessKey: Access key for authentication
//   - secretKey: Secret key for authentication
// Returns:
//   - *S3Client: Initialized S3 client
//   - error: Any error that occurred during initialization
func NewS3Client(endpoint, region, bucket, accessKey, secretKey string) (*S3Client, error) {
	// Load AWS configuration with custom endpoint and credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithBaseEndpoint(endpoint),
		// Configure static credentials
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(
			func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
				}, nil
			})),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with path-style addressing for compatibility
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // Required for MinIO and other S3-compatible services
	})

	return &S3Client{
		client: client,
		bucket: bucket,
	}, nil
}

// GenerateUploadURL generates a presigned URL for uploading media.
// This allows clients to upload directly to S3 without streaming through the CDV service.
// Parameters:
//   - ctx: Context for the operation
//   - key: S3 object key where the file will be stored
//   - expires: Duration until the presigned URL expires
// Returns:
//   - string: Presigned URL for uploading
//   - error: Any error that occurred during URL generation
func (s *S3Client) GenerateUploadURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	// Create a presign client from the S3 client
	presignClient := s3.NewPresignClient(s.client)
	
	// Generate a presigned PUT URL for direct client upload
	presignResult, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), // Target S3 bucket
		Key:    aws.String(key),      // Object key in the bucket
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires // URL expiration time
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignResult.URL, nil
}

// VerifyObject verifies that an object exists and matches the expected checksum.
// This ensures data integrity after upload completion.
// Parameters:
//   - ctx: Context for the operation
//   - key: S3 object key to verify
//   - expectedChecksum: Expected SHA-256 checksum
// Returns:
//   - bool: True if object exists and checksum matches
//   - int64: Object size in bytes
//   - error: Any error that occurred during verification
func (s *S3Client) VerifyObject(ctx context.Context, key, expectedChecksum string) (bool, int64, error) {
	// Get object metadata using HEAD request
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), // Target S3 bucket
		Key:    aws.String(key),      // Object key in the bucket
	})
	if err != nil {
		return false, 0, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Download the object to calculate its checksum
	getObjectOutput, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, 0, fmt.Errorf("failed to download object: %w", err)
	}
	defer getObjectOutput.Body.Close()

	// Calculate SHA-256 checksum of the object
	hash := sha256.New()
	if _, err := io.Copy(hash, getObjectOutput.Body); err != nil {
		return false, 0, fmt.Errorf("failed to calculate checksum: %w", err)
	}
	actualChecksum := fmt.Sprintf("%x", hash.Sum(nil))

	// Compare checksums
	if actualChecksum != expectedChecksum {
		return false, *result.ContentLength, nil
	}

	return true, *result.ContentLength, nil
}
