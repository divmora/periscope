# Periscope Roadmap

**Periscope** (CLI binary: `periscope`) is an enterprise-grade multi-tenant telemetry lakehouse and observability platform for Laravel applications. 

Instead of keeping heavy telemetry in the operational MySQL/PostgreSQL databases forever, `periscope` periodically drains entries from each Laravel application's Telescope tables, denormalizes them, saves compressed Parquet batches to project-isolated storage targets (AWS S3, Cloudflare R2, MinIO, or Local Disk), indexes searchable metadata into a central PostgreSQL instance, and serves a modern, multi-tenant web dashboard.

## 🎯 Core Objectives

1.  **Data Export & Archiving (CLI):** Drain entries from Laravel Telescope, denormalize them, and save compressed Parquet batches to project-isolated storage targets (S3, R2, MinIO, Local Disk) using Hive-partitioning.
2.  **Data Parsing & Optimization:** Parse known JSON content from Telescope into structured columns/fields to enable advanced filtering and querying. Unknown or dynamic content will remain stored as JSON.
3.  **Visualization & Querying (Server/UI):** Serve a modern, multi-tenant web dashboard that closely mimics the original Laravel Telescope interface, operating on the archived data.
4.  **Metadata & User Management:** Index searchable metadata and manage user accounts, multi-tenancy, and RBAC using a central PostgreSQL instance.

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
- [ ] Implement Pluggable Storage Drivers (`internal/storage`):
  - Local Disk, S3, R2, MinIO.
- [ ] Build the `collector` package (`/internal/collector`):
  - Read active projects from PostgreSQL `projects` table.
  - Spawn independent goroutine workers per project with dedicated connection pools for each source Laravel database (MySQL & PostgreSQL support).
  - Use `telescope_watermarks` table per project to incrementally poll entries (`sequence > last_sequence`).
  - Aggregate `telescope_entries_tags` and extract indexed dimensions for structured querying:
    - `user_id` & `user_email`
    - `status_code`, `duration_ms`, `uri` (for HTTP requests)
    - `duration_ms`, SQL query snippet (for queries)
    - `exception_class`, `message` (for exceptions)
  - Write compressed Parquet chunks to the project's designated storage driver with Hive partitioning (e.g., `projectId=...`) to support optimal querying. Schema evolution will be handled internally via Parquet metadata rather than path-based partitioning.
  - Ingest queryable index metadata directly into the central PostgreSQL instance using `pgx.CopyFrom` (binary protocol) for maximum throughput.
  - Safely purge processed rows from the source Laravel database if `prune_enabled` is set to true.
- [ ] Implement the `sync` CLI Command:
  - Add the `sync` subcommand to the single `periscope` CLI binary (`periscope sync`).
  - Accepts inputs (like Project ID) to fetch config data and execute the collector pipeline independently.
  - Provide instructions/snippets in the UI (Project Settings) for users to set this up via CRON or run it manually.

### Phase 4: UI Scaffolding (React + Vite)
- [x] Initialize Vite frontend in `ui/` directory.
- [x] Install Tailwind CSS and Shadcn UI.
- [x] Implement standard API wrappers (`src/api`) and types (`src/types`).
- [x] Implement React Router DOM with `/:projectId` scoped URLs.
- [x] Scaffold layout mirroring the FinGrid Dark Theme aesthetic (Sidebar, Topbar).
- [x] Build robust data table components with pagination using TanStack Table & Query.
- [x] Build core views (Dashboard, Settings, Requests, Exceptions) scoped to the project ID.
- [ ] Implement specialized components (JSON Viewer, SQL highlighter).

### Phase 5: Automated Testing
- [ ] **Backend Unit Testing**: Write Go tests for `internal/crypto`, `internal/auth`, and `internal/store`.
- [ ] **Frontend E2E Testing**: Initialize Playwright in `ui/` and write automated scripts to validate UI flows (e.g., Login, Project Selection).

### Phase 6: Packaging & Deployment
- [ ] Set up `embed.go` to serve the compiled `ui/dist` directory via the HTTP server.
- [ ] Create the multi-stage production `Dockerfile` (builds UI + Go into 1 executable).
- [ ] Set up GitHub Actions CI/CD with Google's **Release Please** for automated semantic versioning and changelogs.
- [ ] Finalize documentation and deployment guides.
