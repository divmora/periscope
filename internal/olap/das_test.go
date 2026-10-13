package olap

import (
	"context"
	"strings"
	"testing"
)

func TestQueryDASDirect_MockFallback(t *testing.T) {
	ctx := context.Background()
	config := map[string]any{
		"s3_bucket": "test-bucket",
		"s3_prefix": "das-logs/",
		"s3_region": "us-east-1",
	}

	filter := DASFilter{
		Limit:  10,
		Offset: 0,
	}

	records, total, err := QueryDASDirect(ctx, config, filter)
	if err != nil {
		t.Fatalf("unexpected error from QueryDASDirect: %v", err)
	}

	if len(records) == 0 {
		t.Errorf("expected mock records, got 0")
	}

	if total != len(records) {
		t.Errorf("expected total %d, got %d", len(records), total)
	}

	// Verify record contents
	first := records[0]
	if first.DatabaseName == "" {
		t.Errorf("expected non-empty database name")
	}
	if first.Command == "" {
		t.Errorf("expected non-empty command")
	}
}

func TestQueryDASDirect_FilterSimulation(t *testing.T) {
	ctx := context.Background()
	config := map[string]any{}

	// Test command filter
	records, _, err := QueryDASDirect(ctx, config, DASFilter{Command: "SELECT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range records {
		if r.Command != "SELECT" {
			t.Errorf("expected command SELECT, got %s", r.Command)
		}
	}

	// Test status filter
	records, _, err = QueryDASDirect(ctx, config, DASFilter{Status: "failure"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range records {
		if r.Status != "failure" {
			t.Errorf("expected status failure, got %s", r.Status)
		}
	}

	// Test search filter
	records, _, err = QueryDASDirect(ctx, config, DASFilter{Search: "users"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) == 0 {
		t.Errorf("expected matching records for search 'users'")
	}
}

func TestGetDASStatsDirect(t *testing.T) {
	ctx := context.Background()
	config := map[string]any{}

	stats, err := GetDASStatsDirect(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error from GetDASStatsDirect: %v", err)
	}

	if stats == nil {
		t.Fatalf("expected non-nil stats")
	}

	if stats.Engine != "direct" {
		t.Errorf("expected engine 'direct', got %s", stats.Engine)
	}

	if stats.TotalQueries <= 0 {
		t.Errorf("expected positive total queries, got %d", stats.TotalQueries)
	}

	if len(stats.CommandCounts) == 0 {
		t.Errorf("expected non-empty command counts")
	}
}

func TestLoadAWSConfig_Modes(t *testing.T) {
	ctx := context.Background()

	// 1. Static keys mode
	keysCfg := map[string]any{
		"aws_auth_mode":         "keys",
		"aws_access_key_id":     "AKIAIOSFODNN7EXAMPLE",
		"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"aws_session_token":     "token123",
	}
	cfg, err := LoadAWSConfig(ctx, "eu-west-1", keysCfg)
	if err != nil {
		t.Fatalf("unexpected error for keys mode: %v", err)
	}
	if cfg.Region != "eu-west-1" {
		t.Errorf("expected region eu-west-1, got %s", cfg.Region)
	}
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		t.Fatalf("failed to retrieve static credentials: %v", err)
	}
	if creds.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("expected access key AKIAIOSFODNN7EXAMPLE, got %s", creds.AccessKeyID)
	}
	if creds.SessionToken != "token123" {
		t.Errorf("expected session token token123, got %s", creds.SessionToken)
	}

	// 2. Resolve DuckDB SQL with static credentials
	duckDBSQL := ResolveDuckDBS3Credentials(ctx, cfg, "https://s3.custom.endpoint")
	if !strings.Contains(duckDBSQL, "SET s3_access_key_id='AKIAIOSFODNN7EXAMPLE'") {
		t.Errorf("expected duckdb sql to contain access key, got: %s", duckDBSQL)
	}
	if !strings.Contains(duckDBSQL, "SET s3_session_token='token123'") {
		t.Errorf("expected duckdb sql to contain session token, got: %s", duckDBSQL)
	}
	if !strings.Contains(duckDBSQL, "SET s3_region='eu-west-1'") {
		t.Errorf("expected duckdb sql to contain region, got: %s", duckDBSQL)
	}

	// 3. Default fallback mode
	defaultCfg := map[string]any{}
	defCfg, err := LoadAWSConfig(ctx, "us-east-2", defaultCfg)
	if err != nil {
		t.Fatalf("unexpected error for default mode: %v", err)
	}
	if defCfg.Region != "us-east-2" {
		t.Errorf("expected region us-east-2, got %s", defCfg.Region)
	}

	// 4. IAM Role mode
	roleCfg := map[string]any{
		"aws_auth_mode":   "role",
		"aws_role_arn":    "arn:aws:iam::123456789012:role/DasAuditReader",
		"aws_external_id": "ext-999",
	}
	rCfg, err := LoadAWSConfig(ctx, "us-east-1", roleCfg)
	if err != nil {
		t.Fatalf("unexpected error for role mode: %v", err)
	}
	if rCfg.Region != "us-east-1" {
		t.Errorf("expected region us-east-1, got %s", rCfg.Region)
	}
}
