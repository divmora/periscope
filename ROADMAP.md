# Periscope Roadmap

**Periscope** (CLI binary: `periscope`) is an enterprise-grade multi-tenant telemetry, database audit, and log lakehouse observability platform.

While originally designed for Laravel Telescope telemetry, Periscope is architected as a universal, high-throughput lakehouse engine for database logs, audit trails, and application observability. It periodically drains entries from source databases and streams (Laravel Telescope, **AWS Aurora Database Activity Streams (DAS)**, **PostgreSQL Audit Logs (`pgaudit`)**, and custom DB log tables), denormalizes and extracts structured dimensions, saves compressed Parquet batches to project-isolated storage targets (AWS S3, Cloudflare R2, MinIO, or Local Disk), indexes searchable watermarks and metadata into PostgreSQL, and serves an embedded DuckDB-powered analytics dashboard.

## 🎯 Core Objectives

1.  **Multi-Source Data Ingestion & Archiving (CLI):** Drain telemetry, audit logs, and activity events from multiple database sources (Laravel Telescope, Aurora DAS, PostgreSQL audit tables, generic DB log schemas) into compressed, Hive-partitioned Parquet batches in cold storage (S3, R2, MinIO, Local Disk).
2.  **Pluggable Schema & Dimension Extraction:** Extract typed dimensions (users, queries, execution duration, status codes, audit classes, statements, IPs) from known log formats while preserving full raw JSON/event payloads. Extensible to custom DB log table structures.
3.  **Embedded OLAP Querying (DuckDB):** Embed DuckDB directly in Go to execute high-performance SQL queries over partitioned Parquet files in S3/MinIO/Disk without duplicating telemetry or audit rows in PostgreSQL.
4.  **Multi-Tenant Dashboard & Unified Observability:** Serve a modern, multi-tenant web dashboard scoped per project (`:projectId`) with dedicated views for Application Telemetry (Requests, Queries, Exceptions, Jobs, Logs) and Database Audit/Activity Logs (Aurora DAS, Postgres Audit).
5.  **Metadata, Watermarking & RBAC:** Index checkpoint watermarks, project configurations, encrypted credentials, and role-based permissions (Admin, Member, Viewer) in a central PostgreSQL instance.

---

## 🛠️ Technical Stack & Constraints

- **Backend / Daemon:** Go 1.25+ (residing at repository root `go.mod`)
- **Metadata & Query Index:** PostgreSQL 17+ using `github.com/jackc/pgx/v5`
- **Cold Storage Format:** Apache Parquet with ZSTD compression using `github.com/parquet-go/parquet-go`
- **Frontend / UI:** Vite + React + Tailwind CSS + Lucide Icons + TanStack Table/Query, located strictly inside `/ui`, managed with **`pnpm`**
- **Binary Bundling:** Single executable shipping the compiled frontend `/ui/dist` via Go `embed.FS`
- **Local Dev Environment:** Fully Dockerized (`docker-compose.yml` orchestrating Go with Air live reload, Vite HMR, PostgreSQL, and MinIO)

---

## 🔐 Role-Based Access Control (RBAC)

RBAC is strictly scoped per project and stored in a `project_memberships` table in PostgreSQL.

- **Viewer:** Read-only access to logs, analytics, traces, and JSON payloads.
- **Member:** All `viewer` capabilities + ability to trigger manual syncs, run ad-hoc queries, and export reports.
- **Admin:** Full control. Includes all `member` capabilities + configuring Laravel DB DSNs, storage credentials, sync intervals, data pruning, and managing project members.

---

## 🏗️ Project Architecture & Layout

```text
periscope/
├── cmd/
│   └── periscope/
│       └── main.go                 # Cobra CLI entrypoint (server, run, backfill, project)
├── internal/
│   ├── api/                        # HTTP Handlers, Go 1.22+ ServeMux routing
│   ├── auth/                       # JWT, Passwords, RBAC Middleware (admin, member, viewer)
│   ├── collector/                  # Background pollers, Watermark checkpoints, DB Drainers
│   ├── storage/                    # Pluggable Storage Drivers (S3, R2, MinIO, Local Disk)
│   └── store/                      # PostgreSQL schemas, migrations, pgx queries, CopyFrom
├── ui/                             # Frontend SPA (Vite + React + Tailwind)
│   ├── src/
│   │   ├── components/             # Reusable UI elements (ProjectSwitcher, JSON Viewer, SQL highlighter)
│   │   ├── layouts/                # ProjectLayout (Sidebar with Project Switcher dropdown)
│   │   ├── pages/                  # ProjectSelector (/projects), Dashboard, Entries, Users, Settings
│   │   └── lib/                    # API client, Auth Context, State Management
│   ├── package.json
│   ├── pnpm-lock.yaml
│   └── vite.config.ts
├── docker-compose.yml              # Local multi-service environment (Postgres, MinIO, Go, UI)
├── Dockerfile                      # Multi-stage production build (builds UI + Go into 1 binary)
├── Dockerfile.dev                  # Go development image with Air
├── .air.toml                       # Air configuration for Go live-reload
├── go.mod                          # go 1.25
├── go.sum
└── embed.go                        # embed.FS exposing ui/dist to HTTP server
```

---

## 🗺️ Roadmap Phases

### Phase 1: Project Initialization & Core Architecture
- [x] Initialize Golang module (`periscope`, Go 1.25+).
- [x] Setup Cobra CLI framework (`cmd/periscope/main.go`) with commands (`server`, `run`, `backfill`, `project`).
- [x] Set up PostgreSQL 17+ schemas and migrations for users, projects/tenants, and metadata (`internal/store`).
- [x] Implement database interactions using `github.com/jackc/pgx/v5` and setup AES-GCM encryption for credentials.
- [x] Configure `docker-compose.yml` for local development (Postgres, MinIO, Go Air, Vite HMR).
- [x] Configure `.air.toml` and `Dockerfile.dev` for Go live-reload.

### Phase 2: Authentication & API Foundation
- [x] Implement Go routing (`github.com/go-chi/chi/v5`) (`internal/api`).
  - [x] Implement URL Path-Based scoping (e.g. `/api/projects/{projectId}/...`).
- [x] Implement User Management and Authentication (`internal/auth`).
  - [x] Username/Password based login.
  - [x] HTTP-Only Cookie and JWT generation/validation (with Bearer token fallback).
  - [x] Password hashing (bcrypt).
  - *Note: OAuth2/SSO and MFA (Multi-Factor Authentication) support are planned for a later phase.*
- [x] Implement RBAC Middleware (Admin, Member, Viewer roles) and Multi-tenant isolation.
- [x] Implement Project Memberships Management and Role delegation.

### Phase 3: Data Extraction & Storage Layer (The Collector Engine)
- [x] Implement Pluggable Storage Drivers (`internal/storage`):
  - [x] Local Disk, AWS S3, Cloudflare R2, MinIO.
- [x] Build the `collector` package (`/internal/collector`):
  - [x] Multi-dialect source database support for Laravel Telescope (MySQL & PostgreSQL).
  - [x] Incremental sequence polling using `telescope_watermarks` (`sequence > last_sequence`).
  - [x] Aggregate `telescope_entries_tags` and extract indexed dimensions for structured querying (requests, queries, exceptions, logs, jobs, commands, models, etc.).
  - [x] Write ZSTD-compressed Parquet chunks with Hive partitioning (`projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/`).
  - [x] Hybrid flush triggers (10,000 rows / 1 hour / 100MB).
  - [x] Checkpoint watermarks in PostgreSQL and safely purge processed rows from source Laravel database if `prune_enabled` is true.
- [x] Implement the `sync` CLI Command:
  - [x] Add the `sync` (and `telescope:sync`) subcommand to the `periscope` CLI binary (`periscope sync <project_iid>`).
  - [x] Accepts project UUID, decrypts DSN and storage credentials via AES-256-GCM, and executes collector pipeline.
  - [x] Structured JSON logging and deterministic exit codes for cron runners and external orchestrators.

### Phase 4: Embedded DuckDB OLAP Engine & Telemetry Query API
- [ ] Embed DuckDB engine into Go backend (`internal/olap` or `internal/query`) for zero-copy querying of Parquet files.
- [ ] Support direct SQL queries over Parquet partitions in AWS S3, MinIO, Cloudflare R2, and Local Disk (using DuckDB `httpfs` extension and local file scans).
- [ ] Replace mock handlers in `internal/api/handlers/` (`requests.go`, `exceptions.go`, `queries.go`, `logs.go`) with live DuckDB query execution.
- [ ] Implement robust query filtering (date/time ranges, status codes, methods, duration thresholds, tags, full-text search) and pagination.
- [ ] Connect dashboard charts and metrics aggregations (throughput, error rates, latency percentiles) to DuckDB queries.

### Phase 5: Multi-Source Database Log & Audit Ingestion
- [ ] **Extensible Collector Ingestion Framework**:
  - [ ] Define generic `LogSource` and `LogRecord` interfaces in `internal/collector/` to support diverse database log sources beyond Laravel Telescope.
  - [ ] Schema mapping configuration per project (timestamp column, sequence/watermark identifier, payload format, type classification).
- [ ] **AWS Aurora Database Activity Streams (DAS)**:
  - [ ] Aurora DAS Stream consumer (AWS Kinesis / CloudWatch / S3 activity stream format).
  - [ ] AWS KMS decryption of encrypted DAS activity records.
  - [ ] Dimension extraction: DB User, Client IP, Session ID, Database Name, SQL Statement, Error Codes, Execution Time, Client Application Name.
  - [ ] Partitioning scheme: `projectId={id}/source=aurora_das/year={YYYY}/month={MM}/day={DD}/hour={HH}/`.
- [ ] **PostgreSQL Audit Logs (`pgaudit` & Custom Audit Tables)**:
  - [ ] Support `pgaudit` log format draining and custom PostgreSQL audit log tables.
  - [ ] Dimension extraction: Audit Type (READ, WRITE, FUNCTION, ROLE, DDL, MISC), Statement, User, Schema, Relation/Table, Command, Query Duration.
  - [ ] Incremental sequence/timestamp watermarking and optional pruning.
  - [ ] Partitioning scheme: `projectId={id}/source=postgres_audit/year={YYYY}/month={MM}/day={DD}/hour={HH}/`.
- [ ] **Generic Database Log Table Adapter**:
  - [ ] Configurable table-based drainer for custom MySQL/PostgreSQL log and event tables.
  - [ ] Dynamic JSON dimension extractor and schema evolution handling.
- [ ] **Audit & Activity Log Dashboard UI**:
  - [ ] Dedicated UI views for Database Activity Streams and Postgres Audit logs.
  - [ ] SQL syntax highlighting, user filtering, IP geolocation/client tracking, and query pattern breakdown.

### Phase 6: Frontend UI Polish & Advanced Components
- [x] Initialize Vite frontend in `ui/` directory with Tailwind CSS and Shadcn UI.
- [x] Implement standard API wrappers (`src/api`) and types (`src/types`).
- [x] Implement React Router DOM with `/:projectId` scoped URLs.
- [x] Scaffold layout mirroring the FinGrid Dark Theme aesthetic (Sidebar, Topbar).
- [x] Build robust data table components with pagination using TanStack Table & Query.
- [x] Build core views (Dashboard, Settings, Requests, Exceptions, Queries, Logs) scoped to the project ID.
- [ ] Implement specialized components:
  - [ ] Interactive JSON tree viewer with copy & search.
  - [ ] Formatted SQL query highlighter with copy and execution plan drawer.
  - [ ] Time-range picker (Last 15m, 1h, 24h, 7d, Custom Range) with auto-refresh.

### Phase 7: Automated Testing & Verification
- [x] **Collector & Storage Test Suites**: 5-Tier comprehensive unit, integration, and adversarial stress tests (100% pass across storage, collector, watermark, and CLI sync).
- [ ] **Backend Unit Testing**: Go tests for `internal/crypto`, `internal/auth`, `internal/store`, and `internal/olap`.
- [ ] **Frontend E2E Testing**: Initialize Playwright in `ui/` and write automated scripts to validate UI flows (Login, Project Selection, Navigation, Data Tables, Settings).

### Phase 8: Packaging & Deployment
- [ ] Set up `embed.go` to serve the compiled `ui/dist` directory via the single Go HTTP server.
- [ ] Create the multi-stage production `Dockerfile` (builds UI + Go into 1 executable binary).
- [ ] Set up GitHub Actions CI/CD with Google's **Release Please** for automated semantic versioning and changelogs.
- [ ] Finalize documentation, architecture diagrams, and deployment guides (Docker, Kubernetes, Bare Metal, Systemd).
