package store_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/store"
)

func TestProjectStructJSONExclusion(t *testing.T) {
	p := store.Project{
		ID:              42,
		IID:             "01912345-6789-7abc-def0-1234567890ab",
		Name:            "Production App",
		StorageType:     "s3",
		StorageBucket:   "telemetry",
		StorageRegion:   "us-east-1",
		StorageEndpoint: "http://localhost:9000",
		PruneEnabled:    true,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		SourceDSN:       "root:secret@tcp(localhost:3306)/laravel",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal project: %v", err)
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		t.Fatalf("failed to unmarshal project JSON: %v", err)
	}

	// Architectural Rule: id (integer) must NEVER be exposed in JSON
	if _, exists := jsonMap["id"]; exists {
		t.Errorf("expected 'id' to be excluded from JSON serialization, but it was found: %v", jsonMap["id"])
	}
	if _, exists := jsonMap["ID"]; exists {
		t.Errorf("expected 'ID' to be excluded from JSON serialization, but it was found: %v", jsonMap["ID"])
	}

	// iid must be present
	if jsonMap["iid"] != p.IID {
		t.Errorf("expected 'iid' to be %s, got %v", p.IID, jsonMap["iid"])
	}
}

func TestWatermarkUninitializedPool(t *testing.T) {
	// When pool is nil, functions should return descriptive error rather than panic
	originalPool := store.Pool
	store.Pool = nil
	defer func() { store.Pool = originalPool }()

	ctx := context.Background()

	_, err := store.GetWatermark(ctx, 1, "request")
	if err == nil {
		t.Error("expected error when Pool is nil in GetWatermark, got nil")
	}

	err = store.UpdateWatermark(ctx, 1, "request", 100)
	if err == nil {
		t.Error("expected error when Pool is nil in UpdateWatermark, got nil")
	}

	_, err = store.GetAllWatermarks(ctx, 1)
	if err == nil {
		t.Error("expected error when Pool is nil in GetAllWatermarks, got nil")
	}

	_, err = store.GetMaxWatermark(ctx, 1)
	if err == nil {
		t.Error("expected error when Pool is nil in GetMaxWatermark, got nil")
	}
}

func TestWatermarkIntegrationWithDB(t *testing.T) {
	if os.Getenv("DB_HOST") == "" && os.Getenv("INTEGRATION_TESTS") == "" {
		t.Skip("Skipping watermark DB integration test: DB_HOST not set")
	}

	ctx := context.Background()
	if err := store.Connect(ctx); err != nil {
		t.Skipf("Skipping watermark DB integration test: connection failed: %v", err)
	}

	projectID := 99999
	entryType := "request_test"

	// Cleanup test data before and after
	cleanup := func() {
		if store.Pool != nil {
			_, _ = store.Pool.Exec(ctx, "DELETE FROM telescope_watermarks WHERE project_id = $1", projectID)
		}
	}
	cleanup()
	defer cleanup()

	// Initial watermark should be 0
	seq, err := store.GetWatermark(ctx, projectID, entryType)
	if err != nil {
		t.Fatalf("GetWatermark failed: %v", err)
	}
	if seq != 0 {
		t.Errorf("expected initial watermark 0, got %d", seq)
	}

	// Update watermark to 100
	if err := store.UpdateWatermark(ctx, projectID, entryType, 100); err != nil {
		t.Fatalf("UpdateWatermark(100) failed: %v", err)
	}

	seq, err = store.GetWatermark(ctx, projectID, entryType)
	if err != nil {
		t.Fatalf("GetWatermark after update failed: %v", err)
	}
	if seq != 100 {
		t.Errorf("expected watermark 100, got %d", seq)
	}

	// Update watermark to 50 (lower sequence) - GREATEST should prevent regression
	if err := store.UpdateWatermark(ctx, projectID, entryType, 50); err != nil {
		t.Fatalf("UpdateWatermark(50) failed: %v", err)
	}

	seq, err = store.GetWatermark(ctx, projectID, entryType)
	if err != nil {
		t.Fatalf("GetWatermark after smaller update failed: %v", err)
	}
	if seq != 100 {
		t.Errorf("expected watermark to remain 100 (GREATEST monotonic guarantee), got %d", seq)
	}

	// Update watermark to 200 (higher sequence)
	if err := store.UpdateWatermark(ctx, projectID, entryType, 200); err != nil {
		t.Fatalf("UpdateWatermark(200) failed: %v", err)
	}

	seq, err = store.GetWatermark(ctx, projectID, entryType)
	if err != nil {
		t.Fatalf("GetWatermark after higher update failed: %v", err)
	}
	if seq != 200 {
		t.Errorf("expected watermark 200, got %d", seq)
	}

	// Test GetAllWatermarks & GetMaxWatermark
	if err := store.UpdateWatermark(ctx, projectID, "query_test", 350); err != nil {
		t.Fatalf("UpdateWatermark(query_test, 350) failed: %v", err)
	}

	all, err := store.GetAllWatermarks(ctx, projectID)
	if err != nil {
		t.Fatalf("GetAllWatermarks failed: %v", err)
	}
	if all[entryType] != 200 || all["query_test"] != 350 {
		t.Errorf("unexpected GetAllWatermarks result: %v", all)
	}

	maxSeq, err := store.GetMaxWatermark(ctx, projectID)
	if err != nil {
		t.Fatalf("GetMaxWatermark failed: %v", err)
	}
	if maxSeq != 350 {
		t.Errorf("expected max watermark 350, got %d", maxSeq)
	}
}
