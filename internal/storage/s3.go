package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// S3Config holds configuration settings for S3-compatible storage backends
// (AWS S3, MinIO, Cloudflare R2, LocalStack).
type S3Config struct {
	Type            string `json:"type,omitempty"`             // "s3", "minio", "r2"
	Bucket          string `json:"bucket"`
	Region          string `json:"region,omitempty"`           // e.g. "us-east-1", "auto" for R2
	Endpoint        string `json:"endpoint,omitempty"`         // Custom endpoint URL (e.g. "http://localhost:9000")
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	UsePathStyle    bool   `json:"use_path_style,omitempty"`   // Mandatory for MinIO
	DisableSSL      bool   `json:"disable_ssl,omitempty"`      // Force HTTP for local dev
}

// S3Driver implements the Driver interface using AWS SDK for Go v2.
type S3Driver struct {
	client     *s3.Client
	bucket     string
	driverType string
}

// NewS3Driver instantiates a new S3Driver configured for AWS S3, MinIO, or Cloudflare R2.
func NewS3Driver(ctx context.Context, cfg S3Config) (*S3Driver, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("%w: bucket name is required", ErrInvalidConfig)
	}

	driverType := strings.ToLower(strings.TrimSpace(cfg.Type))
	if driverType == "" {
		driverType = "s3"
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		if driverType == "r2" {
			region = "auto"
		} else {
			region = "us-east-1"
		}
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	usePathStyle := cfg.UsePathStyle
	if driverType == "minio" {
		usePathStyle = true
	}

	var loadOpts []func(*config.LoadOptions) error
	loadOpts = append(loadOpts, config.WithRegion(region))

	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	if endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(
			func(service, reg string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpoint,
					HostnameImmutable: true,
					SigningRegion:     region,
				}, nil
			},
		)
		loadOpts = append(loadOpts, config.WithEndpointResolverWithOptions(customResolver))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to load AWS config: %v", ErrInvalidConfig, err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = usePathStyle
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &S3Driver{
		client:     client,
		bucket:     cfg.Bucket,
		driverType: driverType,
	}, nil
}

// NewS3DriverWithClient creates an S3Driver with an explicit S3 client (primarily used for unit testing).
func NewS3DriverWithClient(client *s3.Client, bucket string, driverType string) (*S3Driver, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("%w: bucket name is required", ErrInvalidConfig)
	}
	if driverType == "" {
		driverType = "s3"
	}
	return &S3Driver{
		client:     client,
		bucket:     bucket,
		driverType: driverType,
	}, nil
}

// Type returns the driver identifier ("s3", "minio", "r2").
func (d *S3Driver) Type() string {
	return d.driverType
}

// Upload writes data from reader r to the specified S3 key.
func (d *S3Driver) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	cleanKey := strings.TrimPrefix(strings.TrimSpace(key), "/")
	if cleanKey == "" {
		return fmt.Errorf("%w: object key cannot be empty", ErrInvalidConfig)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(d.bucket),
		Key:         aws.String(cleanKey),
		Body:        r,
		ContentType: aws.String(contentType),
	}

	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}

	_, err := d.client.PutObject(ctx, input)
	if err != nil {
		return translateS3Error(err)
	}

	return nil
}

// Get retrieves an object from S3. The caller must close the returned ReadCloser.
func (d *S3Driver) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	cleanKey := strings.TrimPrefix(strings.TrimSpace(key), "/")
	if cleanKey == "" {
		return nil, fmt.Errorf("%w: object key cannot be empty", ErrInvalidConfig)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	output, err := d.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		return nil, translateS3Error(err)
	}

	return output.Body, nil
}

// Exists checks if an object exists at the specified key.
func (d *S3Driver) Exists(ctx context.Context, key string) (bool, error) {
	cleanKey := strings.TrimPrefix(strings.TrimSpace(key), "/")
	if cleanKey == "" {
		return false, fmt.Errorf("%w: object key cannot be empty", ErrInvalidConfig)
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	_, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		// Check for NotFound / NoSuchKey
		var notFound *types.NotFound
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &notFound) || errors.As(err, &noSuchKey) {
			return false, nil
		}

		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}

		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NotFound", "NoSuchKey", "404":
				return false, nil
			case "AccessDenied", "Forbidden", "403":
				return false, ErrPermissionDenied
			case "NoSuchBucket":
				return false, ErrBucketNotFound
			}
		}

		if strings.Contains(err.Error(), "404") ||
			strings.Contains(err.Error(), "NotFound") ||
			strings.Contains(err.Error(), "NoSuchKey") {
			return false, nil
		}

		return false, translateS3Error(err)
	}

	return true, nil
}

// Delete removes an object from S3. Delete is idempotent and returns nil if the object did not exist.
func (d *S3Driver) Delete(ctx context.Context, key string) error {
	cleanKey := strings.TrimPrefix(strings.TrimSpace(key), "/")
	if cleanKey == "" {
		return fmt.Errorf("%w: object key cannot be empty", ErrInvalidConfig)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := d.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		return translateS3Error(err)
	}

	return nil
}

// List returns all objects matching the specified prefix.
func (d *S3Driver) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cleanPrefix := strings.TrimPrefix(strings.TrimSpace(prefix), "/")

	var objects []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(d.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(d.bucket),
		Prefix: aws.String(cleanPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, translateS3Error(err)
		}

		for _, item := range page.Contents {
			objects = append(objects, ObjectInfo{
				Key:          aws.ToString(item.Key),
				Size:         aws.ToInt64(item.Size),
				LastModified: aws.ToTime(item.LastModified).UTC(),
				ETag:         strings.Trim(aws.ToString(item.ETag), `"`),
			})
		}
	}

	return objects, nil
}

// Ping checks if the S3 bucket is reachable and accessible.
func (d *S3Driver) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := d.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(d.bucket),
	})
	if err != nil {
		return translateS3Error(err)
	}

	return nil
}

// translateS3Error maps AWS SDK v2 errors and HTTP response status codes
// to standard storage error definitions.
func translateS3Error(err error) error {
	if err == nil {
		return nil
	}

	var notFound *types.NotFound
	var noSuchKey *types.NoSuchKey
	var noSuchBucket *types.NoSuchBucket
	if errors.As(err, &notFound) || errors.As(err, &noSuchKey) {
		return ErrObjectNotFound
	}
	if errors.As(err, &noSuchBucket) {
		return ErrBucketNotFound
	}

	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.HTTPStatusCode() {
		case http.StatusNotFound:
			return ErrObjectNotFound
		case http.StatusForbidden, http.StatusUnauthorized:
			return ErrPermissionDenied
		}
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return ErrObjectNotFound
		case "NoSuchBucket":
			return ErrBucketNotFound
		case "AccessDenied", "Forbidden", "InvalidAccessKeyId", "SignatureDoesNotMatch":
			return ErrPermissionDenied
		}
	}

	errStr := err.Error()
	if strings.Contains(errStr, "NoSuchBucket") {
		return ErrBucketNotFound
	}
	if strings.Contains(errStr, "NoSuchKey") || strings.Contains(errStr, "NotFound") {
		return ErrObjectNotFound
	}
	if strings.Contains(errStr, "AccessDenied") || strings.Contains(errStr, "Forbidden") ||
		strings.Contains(errStr, "403") {
		return ErrPermissionDenied
	}

	return err
}
