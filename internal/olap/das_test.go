package olap

import (
	"context"
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
