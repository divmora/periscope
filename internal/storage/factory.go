package storage

import (
	"context"
	"fmt"
	"strings"
)

// NewDriverFromConfig instantiates a storage Driver based on the provided Config.
// Supported driver types: "local", "s3", "minio", "r2".
func NewDriverFromConfig(ctx context.Context, cfg Config) (Driver, error) {
	driverType := strings.ToLower(strings.TrimSpace(cfg.Type))

	// If type is not explicitly set, infer from present configuration fields
	if driverType == "" {
		if cfg.BasePath != "" {
			driverType = "local"
		} else if cfg.Bucket != "" {
			driverType = "s3"
		} else {
			return nil, fmt.Errorf("%w: driver type or configuration must be specified", ErrInvalidConfig)
		}
	}

	switch driverType {
	case "local":
		if strings.TrimSpace(cfg.BasePath) == "" {
			return nil, fmt.Errorf("%w: base_path is required for local storage driver", ErrInvalidConfig)
		}
		return NewLocalDriver(LocalConfig{
			BasePath: cfg.BasePath,
		})

	case "s3", "minio", "r2":
		if strings.TrimSpace(cfg.Bucket) == "" {
			return nil, fmt.Errorf("%w: bucket is required for %s storage driver", ErrInvalidConfig, driverType)
		}

		usePathStyle := cfg.UsePathStyle
		if driverType == "minio" {
			usePathStyle = true
		}

		region := cfg.Region
		if region == "" && driverType == "r2" {
			region = "auto"
		}

		return NewS3Driver(ctx, S3Config{
			Type:            driverType,
			Bucket:          cfg.Bucket,
			Region:          region,
			Endpoint:        cfg.Endpoint,
			AccessKeyID:     cfg.AccessKeyID,
			SecretAccessKey: cfg.SecretAccessKey,
			UsePathStyle:    usePathStyle,
			DisableSSL:      cfg.DisableSSL,
		})

	default:
		return nil, fmt.Errorf("%w: '%s'", ErrUnsupportedDriver, cfg.Type)
	}
}
