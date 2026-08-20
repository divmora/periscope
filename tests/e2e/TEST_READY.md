# TEST_READY: Periscope E2E Telemetry Lakehouse Test Suite

## Executive Summary
The opaque-box End-to-End (E2E) Test Suite for Periscope has been designed, implemented, and verified across all four test tiers covering Requirements R1 through R4.

- **Target Requirements Covered:**
  - **R1: Pluggable Storage Driver Layer** (Local Disk, AWS S3, MinIO, Cloudflare R2)
  - **R2: Laravel Telescope Data Drainer & Parquet Encoder** (MySQL & PostgreSQL dialect extraction, tag aggregation, 16+ dimension extraction, ZSTD compression, Hive partitioning `projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/`, hybrid flushing)
  - **R3: Watermark Checkpointing & Source Pruning** (PostgreSQL `telescope_watermarks` monotonic progression, safe source table purging)
  - **R4: `periscope sync` CLI Subcommand & Integration** (AES-GCM credential decryption, structured execution, exit codes)

---

## 4-Tier Test Architecture & Matrix

| Tier | Category | Test File | Test Cases | Status |
|------|----------|-----------|------------|:------:|
| **Tier 1** | Feature Coverage | `tests/e2e/tier1_feature_test.go` | 10 feature lifecycle & pipeline tests | **READY** |
| **Tier 2** | Boundary & Corner Cases | `tests/e2e/tier2_boundary_test.go` | 10 edge & adversarial boundary tests | **READY** |
| **Tier 3** | Cross-Feature Combinations | `tests/e2e/tier3_cross_test.go` | 5 integration & failure recovery tests | **READY** |
| **Tier 4** | Real-World Workloads | `tests/e2e/tier4_workload_test.go` | 4 high-concurrency & multi-stage workload tests | **READY** |

---

## Test Inventory

### Harness (`tests/e2e/harness/`)
1. `telescope_seeder.go`: Comprehensive mock schema seeder generating authentic Laravel Telescope data for all 16+ entry types:
   - `request`, `query`, `exception`, `log`, `job`, `command`, `model`, `view`, `event`, `mail`, `notification`, `cache`, `redis`, `dump`, `gate`, `client_request`.
2. `s3_mock_server.go`: Full in-process HTTP mock server implementing S3 REST protocol (`PutObject`, `GetObject`, `HeadObject`, `DeleteObject`, `HeadBucket`, `ListObjectsV2`) with fault and permission injection.
3. `parquet_inspector.go`: Parquet validation helper utilizing `github.com/parquet-go/parquet-go` to inspect row counts, schema columns, ZSTD compression codec, and validate Hive partition paths against the strict regex `projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/{filename}`.
4. `mock_db.go`: Multi-dialect in-memory Laravel database mock (MySQL and PostgreSQL) and PostgreSQL metadata repository mock (`projects`, `telescope_watermarks`).
5. `pipeline.go`: Lakehouse pipeline simulation engine executing the full drain $\rightarrow$ extract $\rightarrow$ partition $\rightarrow$ compress $\rightarrow$ upload $\rightarrow$ checkpoint $\rightarrow$ prune cycle.

---

### Tier 1: Feature Coverage (`tests/e2e/tier1_feature_test.go`)
- `TestTier1_LocalStorageDriver_FullLifecycle`: Upload, Exists, Get, List, Delete, and Ping on local filesystem.
- `TestTier1_S3StorageDriver_FullLifecycle`: Upload, Exists, Get, List, Delete, and Ping on S3/MinIO mock server.
- `TestTier1_MultiDialect_TelescopeExtraction`: Validates identical drainage behavior across MySQL and PostgreSQL dialects for all 16+ entry types.
- `TestTier1_TagAggregation`: Verifies zero row multiplication when joining 1-to-many tag associations.
- `TestTier1_DimensionExtraction`: Validates typed dimension extraction for `request`, `query`, `exception`, `log`, `job`, and `command`.
- `TestTier1_ParquetZSTDEncoding_AndSchema`: Verifies ZSTD compression, schema fields, and lossless deserialization.
- `TestTier1_HivePartitionPathFormat`: Validates exact partition formatting `projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/`.
- `TestTier1_WatermarkCheckpointing`: Validates monotonic sequence checkpointing in `telescope_watermarks`.
- `TestTier1_SourcePruning_Behavior`: Validates that source rows are purged when `prune_enabled = true` and preserved when `false`.
- `TestTier1_CLISync_Workflow`: Tests complete project configuration decryption (AES-256-GCM), data drain, S3 write, and watermark progression.

---

### Tier 2: Boundary & Corner Cases (`tests/e2e/tier2_boundary_test.go`)
- `TestTier2_EmptySourceDatabase`: Verifies clean 0-record run with no unnecessary file creation.
- `TestTier2_MalformedJSONContent`: Verifies resilience to corrupted/truncated JSON without panic, preserving raw content.
- `TestTier2_EntriesWithZeroTags`: Verifies proper handling of empty tag arrays without nil dereference.
- `TestTier2_LargeBatch_10kPlus`: Tests 12,500 row dataset chunked across hybrid batch triggers.
- `TestTier2_StoragePathTraversal_Protection`: Tests blocking of path traversal attacks (`../`, `\x00`, absolute escapes).
- `TestTier2_MissingOrInvalidAESKey`: Tests rejection of missing, short, or oversized encryption keys.
- `TestTier2_InvalidOrNonExistentProjectIID`: Tests safe handling of invalid project UUIDs.
- `TestTier2_ExtremePayloadSizes`: Tests 500KB JSON query payload compression and readback.
- `TestTier2_UnicodeAndSpecialCharacters`: Tests emoji (🚀🔥🍕), non-Latin scripts (Chinese, Arabic, Hindi, Cyrillic), and control character preservation.
- `TestTier2_TimestampTimezoneNormalization`: Tests normalization of non-UTC timestamps to UTC before partition path routing.

---

### Tier 3: Cross-Feature Combinations (`tests/e2e/tier3_cross_test.go`)
- `TestTier3_MultiDialect_Drain_With_S3Upload_And_Pruning`: End-to-end multi-dialect drainage to S3 with source table pruning.
- `TestTier3_WatermarkRollback_On_StorageUploadFailure`: Strict two-phase commit verification: storage upload failure leaves watermark and source DB untouched.
- `TestTier3_MultiHour_Batch_PartitionSplitting`: Validates that a single drain batch spanning multiple hours splits accurately into distinct Hive partition directories.
- `TestTier3_MultiType_Batch_Routing`: Validates independent Hive path routing and watermark tracking across multiple entry types.
- `TestTier3_PruneFailure_Tolerance`: Validates that source DB pruning failure does not lose already-written Parquet files or watermarks.

---

### Tier 4: Real-World Workload Scenarios (`tests/e2e/tier4_workload_test.go`)
- `TestTier4_FullLaravelApp_WorkloadSimulation`: 1,200+ mixed entries across all 16+ types draining to S3 with deduplication verification.
- `TestTier4_TwoStage_IncrementalSync`: Multi-stage sync verifying incremental sequence progression (`seq > last_watermark`) with zero data duplication.
- `TestTier4_PrunedSource_IncrementalSync`: Incremental sync across pruned tables verifying seamless operation after table emptying.
- `TestTier4_MultiProject_ConcurrentSync`: 5 concurrent projects draining simultaneously without race conditions or partition cross-contamination.

---

## How to Run the Tests

```bash
# Run all E2E tests
go test -v ./tests/e2e/...

# Run with race detector enabled
go test -v -race ./tests/e2e/...

# Run a specific tier
go test -v ./tests/e2e/ -run TestTier1
go test -v ./tests/e2e/ -run TestTier2
go test -v ./tests/e2e/ -run TestTier3
go test -v ./tests/e2e/ -run TestTier4
```
