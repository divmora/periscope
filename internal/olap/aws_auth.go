package olap

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// LoadAWSConfig builds an aws.Config using the per-source authentication strategy:
// 1. "keys": Static AccessKeyID, SecretAccessKey, and optional SessionToken.
// 2. "profile": Named profile loaded from ~/.aws/credentials and ~/.aws/config.
// 3. "role": STS AssumeRole via an IAM Role ARN and optional ExternalID.
// 4. "default": Fallback to standard AWS credential chain (IRSA, ECS, EC2 IMDS, env).
func LoadAWSConfig(ctx context.Context, region string, cfgMap map[string]any) (aws.Config, error) {
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-east-1"
		}
	}

	authMode, _ := cfgMap["aws_auth_mode"].(string)
	profile, _ := cfgMap["aws_profile"].(string)
	accessKey, _ := cfgMap["aws_access_key_id"].(string)
	secretKey, _ := cfgMap["aws_secret_access_key"].(string)
	sessionToken, _ := cfgMap["aws_session_token"].(string)
	roleARN, _ := cfgMap["aws_role_arn"].(string)
	externalID, _ := cfgMap["aws_external_id"].(string)

	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(region))

	// Detect mode if not explicitly declared
	if authMode == "" {
		if roleARN != "" {
			authMode = "role"
		} else if accessKey != "" && secretKey != "" {
			authMode = "keys"
		} else if profile != "" {
			authMode = "profile"
		} else {
			authMode = "default"
		}
	}

	switch authMode {
	case "profile":
		if profile != "" {
			opts = append(opts, config.WithSharedConfigProfile(profile))
		}
		return config.LoadDefaultConfig(ctx, opts...)

	case "keys":
		if accessKey != "" && secretKey != "" {
			opts = append(opts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken),
			))
		}
		return config.LoadDefaultConfig(ctx, opts...)

	case "role":
		// First load the base AWS configuration (which provides the principal doing the assume)
		baseCfg, err := config.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to load base AWS config for STS assume role: %w", err)
		}

		if roleARN != "" {
			stsClient := sts.NewFromConfig(baseCfg)
			assumeOptions := func(o *stscreds.AssumeRoleOptions) {
				if externalID != "" {
					o.ExternalID = aws.String(externalID)
				}
				o.RoleSessionName = "periscope-das-session"
			}
			roleProvider := stscreds.NewAssumeRoleProvider(stsClient, roleARN, assumeOptions)
			baseCfg.Credentials = aws.NewCredentialsCache(roleProvider)
		}
		return baseCfg, nil

	default:
		return config.LoadDefaultConfig(ctx, opts...)
	}
}

// ResolveDuckDBS3Credentials retrieves credentials from the resolved AWS config
// and formats DuckDB SQL commands to configure the S3/httpfs extension.
func ResolveDuckDBS3Credentials(ctx context.Context, awsCfg aws.Config, endpoint string) string {
	sql := fmt.Sprintf("SET s3_region='%s';", awsCfg.Region)
	if endpoint != "" {
		sql += fmt.Sprintf(" SET s3_endpoint='%s'; SET s3_use_ssl=false; SET s3_url_style='path';", endpoint)
	}

	if awsCfg.Credentials != nil {
		creds, err := awsCfg.Credentials.Retrieve(ctx)
		if err == nil && creds.AccessKeyID != "" {
			sql += fmt.Sprintf(" SET s3_access_key_id='%s'; SET s3_secret_access_key='%s';", creds.AccessKeyID, creds.SecretAccessKey)
			if creds.SessionToken != "" {
				sql += fmt.Sprintf(" SET s3_session_token='%s';", creds.SessionToken)
			}
		}
	}

	return sql
}
