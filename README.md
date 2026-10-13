# Periscope

[![Latest Release](https://img.shields.io/github/v/release/divmora/periscope?logo=github)](https://github.com/divmora/periscope/releases)
[![License: BSL 1.1](https://img.shields.io/badge/License-BSL_1.1-blue.svg)](https://github.com/divmora/.github/blob/main/LICENSING.md)
[![CI/CD](https://github.com/divmora/periscope/actions/workflows/ci.yml/badge.svg)](https://github.com/divmora/periscope/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/divmora/periscope)](go.mod)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/divmora/periscope)
[![Security Policy](https://img.shields.io/badge/Security-Policy-green.svg)](SECURITY.md)

**Periscope** is a performant, enterprise-grade telemetry lakehouse and observability dashboard. It collects telemetry data (requests, queries, logs, exceptions, and jobs) from remote Laravel applications and database activity streams, storing them as highly compressed Parquet files in S3-compatible object storage (AWS S3, MinIO, Cloudflare R2) or local disk. The Go backend embeds **DuckDB** as an OLAP query engine to query Parquet files directly with zero-copy partition pruning, while PostgreSQL is strictly reserved for RBAC, configuration, encrypted credentials, and watermarks.

---

## 🏛️ Architecture & Tech Stack

- **Backend:** Go 1.25+ (Single static Cobra binary)
- **Metadata Database:** PostgreSQL 17+ (Configuration, RBAC, Users, Watermarks)
- **Lakehouse Storage:** Apache Parquet with ZSTD compression in S3 / MinIO / Cloudflare R2 / Local Disk
- **OLAP Engine:** Embedded DuckDB in Go
- **Frontend:** React + Vite + Tailwind CSS + TanStack Table/Query (Embedded into the Go binary)

For full architectural specifications, sequence diagrams, and storage layouts, see [docs/architecture.md](docs/architecture.md).

```mermaid
graph TB
    subgraph "External Sources"
        L_DB["Laravel DB (MySQL/PG)"]
        DAS["Aurora DAS / Postgres Audit"]
    end

    subgraph "Single Periscope Binary"
        CLI["CLI Sync Engine (periscope sync)"]
        COL["Collector & ZSTD Parquet Encoder"]
        DUCK["Embedded DuckDB Engine"]
        API["HTTP API Server (:8006)"]
        UI["Embedded React SPA"]
    end

    subgraph "Storage"
        PG_META[("PostgreSQL 17<br/>Metadata, RBAC & Watermarks")]
        LAKEHOUSE[("S3 / MinIO / Disk Lakehouse<br/>projectId=.../source=.../type=.../")]
    end

    L_DB -->|"Incremental Drain"| COL
    DAS -->|"Stream Ingest"| COL
    COL -->|"ZSTD Parquet Batches"| LAKEHOUSE
    COL -->|"Update Watermarks"| PG_META
    API -->|"Zero-Copy Parquet Query"| DUCK
    DUCK --> LAKEHOUSE
    UI --> API
```

---

## 🚀 Single Binary Deployment

The UI is compiled via Vite and completely embedded into the Go binary using `//go:embed`.
- All requests starting with `/api` are handled by the Go JSON router.
- All other requests serve the static React frontend.
- The entire application runs natively on port `8006` with no separate UI container or reverse proxy needed.

---

## ⚙️ Configuration & Environment Variables

Periscope is configured using environment variables or a local `.env` file:

| Variable | Description | Default | Example |
| :--- | :--- | :--- | :--- |
| `PORT` | HTTP server listening port | `8006` | `8006` |
| `APP_ENV` | Application environment | `local` | `production` |
| `APP_DEBUG` | Enable debug logging | `true` | `false` |
| `APP_URL` | Base application URL | `http://localhost:8006` | `https://telemetry.example.com` |
| `JWT_SECRET` | Secret key used for signing authentication JWTs | — | `openssl rand -base64 32` |
| `AES_ENCRYPTION_KEY` | 32-byte AES key used to encrypt source DSNs & S3 secrets | — | `openssl rand -base64 32` |
| `DB_HOST` | PostgreSQL metadata host | `postgres` | `127.0.0.1` |
| `DB_PORT` | PostgreSQL metadata port | `5432` | `5432` |
| `DB_USER` | PostgreSQL user | `periscope` | `periscope` |
| `DB_PASSWORD` | PostgreSQL password | `secret` | `mypassword` |
| `DB_NAME` | PostgreSQL database name | `periscope_db` | `periscope_db` |
| `DB_SSLMODE` | PostgreSQL SSL mode | `disable` | `require` |
| `S3_ENDPOINT` | S3-compatible API endpoint | `localhost:9000` | `s3.us-east-1.amazonaws.com` |
| `S3_ACCESS_KEY_ID` | S3 access key ID | `minioadmin` | `AKIAIOSFODNN7EXAMPLE` |
| `S3_SECRET_ACCESS_KEY` | S3 secret access key | `minioadmin` | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY` |
| `S3_BUCKET` | Destination telemetry lakehouse bucket name | `periscope-telemetry` | `corp-telemetry-lakehouse` |
| `S3_REGION` | S3 storage region | `us-east-1` | `us-east-1` |
| `S3_USE_SSL` | Enable TLS/SSL for S3 endpoint | `false` | `true` |

---

## 🔐 IAM Permissions (AWS S3 Lakehouse)

When deploying Periscope with AWS S3, grant the following least-privilege IAM policy to the Periscope execution role or IAM user:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "PeriscopeLakehouseBucketAccess",
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket",
                "s3:GetBucketLocation"
            ],
            "Resource": "arn:aws:s3:::periscope-telemetry"
        },
        {
            "Sid": "PeriscopeLakehouseObjectAccess",
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject"
            ],
            "Resource": "arn:aws:s3:::periscope-telemetry/*"
        }
    ]
}
```

---

## 🛠️ Development & Building

### Prerequisites
- **Go 1.25+**
- **Docker & Docker Compose**
- **Make**
- **Node.js 26+ & pnpm** (for UI development)

### Common Make Commands

```bash
# Build the native Go binary into bin/
make build

# Run unit tests
make test

# Run tests with race detection
make test-race

# Format Go source code
make fmt

# Run linters
make lint

# Install frontend dependencies
make ui-install

# Build embedded frontend assets
make ui-build

# Clean build artifacts
make clean
```

### Local Development Workflow

To get the best of both worlds (fast backend recompilation + fast frontend hot-reloading):
1. **Backend:** Run `docker-compose up -d --build`. This starts PostgreSQL, MinIO, and uses `air` to hot-reload the Go binary on port `8006`.
2. **Frontend:** Run `pnpm dev` inside the `ui/` directory. This starts Vite on port `5173` with instant HMR and proxies `/api` calls to `8006`.

### Database Migrations

We use `pressly/goose` for database migrations. Migrations reside in `internal/store/migrations/*.sql` and are embedded directly into the CLI tool:

```bash
go run cmd/periscope/main.go migrate status
go run cmd/periscope/main.go migrate up
go run cmd/periscope/main.go migrate down
```

### Admin User Setup

To seed your initial administrator user:
```bash
go run cmd/periscope/main.go make:user
```

---

## 🤝 Community & Contributing

- **[Contributing Guide](CONTRIBUTING.md)**: Details on local dev setup, Makefile targets, and Conventional Commit guidelines.
- **[Code of Conduct](https://github.com/divmora/.github/blob/main/CODE_OF_CONDUCT.md)**: Contributor Covenant Code of Conduct.
- **[Security Policy](SECURITY.md)**: Vulnerability disclosure procedure and response SLA.

---

## 📄 License & Commercial Use

This project is licensed under the **Business Source License 1.1 (BSL 1.1)**.

- **Non-Production & Evaluation:** Free to use, modify, and test in non-production environments (local development, staging, QA, CI/CD pipelines, and proof-of-concept evaluation).
- **Production & Commercial Use:** Deploying or executing in production environments, embedding into commercial products, or offering as a managed service requires a commercial license (EULA) from **DIVMORA Technologies**.
- **Open Source Transition:** Each release automatically converts to the **Apache License, Version 2.0** three (3) years after its release date.

For commercial licensing inquiries, enterprise support, or questions, please contact **licensing@divmora.com** or visit [divmora.com](https://divmora.com).
