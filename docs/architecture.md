# Periscope Architecture & Application Flow

This document details the high-level system architecture, component relationships, data ingestion pipeline, and query flow for **Periscope**.

---

## 🏛️ 1. High-Level System Architecture

```mermaid
graph TB
    subgraph "External Data Sources"
        L_PG["Remote Laravel DB (PostgreSQL)"]
        L_MY["Remote Laravel DB (MySQL)"]
        DAS["AWS Aurora DAS (Activity Stream / Kinesis)"]
        AUD["PostgreSQL Audit Logs (pgaudit / audit tables)"]
    end

    subgraph "Single Periscope Executable (Go Binary)"
        CLI["Cobra CLI Engine<br/>(cmd/periscope)"]
        
        subgraph "Ingestion Subsystem"
            DRAIN["Multi-Dialect Source Drainer<br/>(internal/collector/source.go)"]
            EXTRACT["Typed Dimension Extractor<br/>(internal/collector/extractor.go)"]
            FLUSH["Hybrid Batch Flusher<br/>(internal/collector/flusher.go)"]
            ENC["ZSTD Parquet Encoder<br/>(internal/collector/parquet.go)"]
        end

        subgraph "Storage Driver Layer"
            S_DRV["Pluggable Storage Factory<br/>(internal/storage)"]
            S_LOC["Local Disk Driver"]
            S_S3["AWS S3 / MinIO / R2 Driver"]
        end

        subgraph "Security & API Server"
            CRYPT["AES-256-GCM Crypto<br/>(internal/crypto)"]
            AUTH["JWT / RBAC Middleware<br/>(internal/auth)"]
            API["Chi HTTP API Handlers<br/>(internal/api/handlers)"]
        end

        subgraph "Analytics & Query Subsystem"
            DUCK["Embedded DuckDB OLAP Engine<br/>(internal/olap)"]
        end

        subgraph "Embedded Frontend"
            SPA["React + Vite + Tailwind SPA<br/>(embed.FS)"]
        end
    end

    subgraph "Persistent Storage Targets"
        PG_META[("Metadata Database (PostgreSQL 17)<br/>• Users & Passwords<br/>• Projects & Encrypted DSNs<br/>• RBAC Memberships<br/>• Sequence Watermarks")]
        LAKEHOUSE[("Telemetry Lakehouse Storage<br/>(S3 / MinIO / Local Disk)<br/>projectId=.../source=.../type=.../")]
    end

    subgraph "Users & Operators"
        BROWSER["Web Browser (Admin / Member / Viewer)"]
        CRON["Cron / Daemon Runner"]
    end

    %% CLI execution
    CRON -->|"periscope sync <project_iid>"| CLI
    CLI --> DRAIN
    CLI --> CRYPT

    %% Ingestion Flow
    CRYPT -->|"Decrypt DSN & S3 Keys"| PG_META
    DRAIN -->|"Poll entries & tags"| L_PG
    DRAIN -->|"Poll entries & tags"| L_MY
    DRAIN --> EXTRACT
    EXTRACT --> FLUSH
    FLUSH --> ENC
    ENC --> S_DRV
    S_DRV --> S_LOC
    S_DRV --> S_S3
    S_LOC --> LAKEHOUSE
    S_S3 --> LAKEHOUSE
    FLUSH -->|"1. Checkpoint Watermark"| PG_META
    FLUSH -->|"2. Safe Prune (if enabled)"| L_PG
    FLUSH -->|"2. Safe Prune (if enabled)"| L_MY

    %% Query Flow
    BROWSER -->|"HTTPS (:8006)"| API
    BROWSER -->|"Static Assets"| SPA
    API --> AUTH
    AUTH -->|"Check RBAC & Membership"| PG_META
    API --> DUCK
    DUCK -->|"Zero-Copy Partition Pruning (read_parquet)"| LAKEHOUSE
```

---

## 🔄 2. Ingestion & Draining Pipeline Flow (`periscope sync <project_iid>`)

```mermaid
sequenceDiagram
    autonumber
    participant CLI as CLI / Sync Daemon
    participant MetaDB as Metadata DB (PostgreSQL)
    participant Crypto as AES-256 Crypto
    participant SourceDB as Remote Laravel DB
    participant Collector as Collector / Flusher
    participant Storage as Lakehouse Storage (S3 / MinIO)

    CLI->>MetaDB: Query Project by public UUID (`iid`)
    MetaDB-->>CLI: Encrypted Source DSN & Storage Config
    CLI->>Crypto: Decrypt DSN & S3 credentials
    Crypto-->>CLI: Plaintext credentials in memory
    
    CLI->>MetaDB: Fetch latest watermarks (`last_sequence` per type)
    MetaDB-->>CLI: e.g. request: 104500, query: 92100
    
    CLI->>SourceDB: Connect (MySQL / PostgreSQL pool)
    
    loop For Each Entry Type (request, query, exception, etc.)
        CLI->>SourceDB: SELECT entries & tags WHERE sequence > last_sequence LIMIT batch_size
        SourceDB-->>CLI: Batch of raw rows + tags
        
        CLI->>Collector: Extract typed dimensions + raw JSON content
        Collector->>Collector: Buffer records & check hybrid triggers (10k rows / 1hr / 100MB)
        
        alt Buffer Triggered
            Collector->>Collector: Encode batch to ZSTD columnar Parquet
            Collector->>Storage: Upload to `projectId={id}/source=laravel_telescope/type={type}/year=.../part-....parquet`
            Storage-->>Collector: 200 OK (Write Confirmed)
            
            Collector->>MetaDB: Checkpoint Watermark (update last_sequence)
            MetaDB-->>Collector: OK
            
            opt If prune_enabled == true
                Collector->>SourceDB: DELETE FROM entries & tags WHERE sequence <= max_sequence
                SourceDB-->>Collector: Rows purged
            end
        end
    end
    
    Collector-->>CLI: Flush remaining buffers & report SyncResult
    CLI-->>CLI: Log structured JSON & exit(0)
```

---

## ⚡ 3. Analytics & Query Flow (Embedded DuckDB)

```mermaid
sequenceDiagram
    autonumber
    participant User as Web Browser (UI)
    participant Chi as Chi Router & Middleware
    participant Auth as RBAC / Session Auth
    participant MetaDB as Metadata DB (PostgreSQL)
    participant DuckDB as Embedded DuckDB Engine
    participant Lakehouse as S3 / MinIO / Disk

    User->>Chi: GET /api/projects/:projectId/telescope/requests?status=500&limit=50
    Chi->>Auth: Validate JWT / Session Cookie
    Auth->>MetaDB: Check user role in `project_memberships`
    MetaDB-->>Auth: Role: `member` (Access Granted)
    
    Chi->>DuckDB: Execute SQL query with partition pruning
    Note over DuckDB,Lakehouse: DuckDB executes read_parquet() against S3/Disk<br/>Targeting: projectId={id}/source=laravel_telescope/type=request/year=2026/...
    DuckDB->>Lakehouse: Stream only matching Parquet column chunks
    Lakehouse-->>DuckDB: Binary Parquet columns
    DuckDB->>DuckDB: Apply filters (status=500), sort (created_at DESC), paginate
    DuckDB-->>Chi: Result rows + total count
    
    Chi-->>User: Standard JSON Envelope { success: true, data: [...], meta: {...} }
```

---

## 🗄️ 4. Lakehouse Storage Hierarchy (Hive Partitioning)

```text
periscope-bucket/
├── projectId=01912345-6789-7abc-def0-1234567890ab/
│   │
│   ├── source=laravel_telescope/
│   │   ├── type=request/
│   │   │   └── year=2026/month=08/day=29/hour=14/
│   │   │       └── part-000000000001-000000010000-a1b2c3d4.parquet
│   │   ├── type=query/
│   │   ├── type=exception/
│   │   └── type=log/
│   │
│   ├── source=aurora_das/
│   │   └── type=activity/
│   │       └── year=2026/month=08/day=29/hour=14/
│   │           └── part-000000000001-000000005000-f1e2d3c4.parquet
│   │
│   └── source=postgres_audit/
│       └── type=audit/
│           └── year=2026/month=08/day=29/hour=14/
│               └── part-000000000001-000000008000-7e8f9a0b.parquet
```

---

## 🧩 5. Component Responsibility Matrix

| Component | Directory | Responsibility |
| :--- | :--- | :--- |
| **CLI Framework** | `cmd/periscope/` | Entrypoint for all subcommands: `serve`, `sync`, `migrate`, `make:user`. |
| **Crypto Subsystem** | `internal/crypto/` | AES-256-GCM symmetric encryption for sensitive connection strings and secrets in PostgreSQL. |
| **Metadata Store** | `internal/store/` | PostgreSQL interactions via `pgx/v5`: User auth, Projects, RBAC memberships, and Watermark checkpoints. |
| **Auth & RBAC** | `internal/auth/` | Password hashing (bcrypt), JWT generation, HttpOnly session cookies, and project-scoped access guards (`admin`, `member`, `viewer`). |
| **Storage Drivers** | `internal/storage/` | Pluggable file storage abstraction: `LocalDriver` (atomic disk operations) and `S3Driver` (AWS S3, MinIO, Cloudflare R2). |
| **Collector & Drainer** | `internal/collector/` | Drains Laravel MySQL/Postgres DBs, extracts 16+ typed dimensions, encodes ZSTD Parquet, flushes hybrid batches, updates watermarks, and prunes source DB. |
| **OLAP Query Engine** | `internal/olap/` | Embedded DuckDB engine executing fast zero-copy SQL over S3/MinIO/Local Parquet files. |
| **API Server** | `internal/api/` | Chi router implementing standard `{ success, data, error, meta }` JSON envelopes. |
| **Frontend SPA** | `ui/` | React 18 + Vite + Tailwind CSS dashboard embedded into the Go executable via `embed.FS`. |
