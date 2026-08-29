# Periscope Agent Instructions

Welcome to the `periscope` repository. If you are an AI agent working in this codebase, you MUST adhere to the following strict architectural rules, tech stack choices, and workflows.

## 🛠️ Tech Stack & Dependencies

- **Backend:** Go 1.25+
- **CLI Framework:** Cobra (`github.com/spf13/cobra`)
- **Metadata Database:** PostgreSQL 17+
- **Postgres Driver:** `github.com/jackc/pgx/v5`
- **Lakehouse / Cold Storage:** Apache Parquet with ZSTD compression (`github.com/parquet-go/parquet-go`)
- **OLAP Engine:** DuckDB (embedded in the Go backend)
- **Frontend:** Vite + React + Tailwind CSS + Lucide Icons + TanStack Table/Query (located strictly inside `/ui`, managed with `pnpm`)
- **Communication:** gRPC for communication between `telescope:sync` workers and the central server.

## 🏗️ Architectural Directives

### 1. Parquet is the Single Source of Truth
**CRITICAL RULE:** Do NOT duplicate entry metadata (like `status_code`, `duration_ms`, `uri`) into PostgreSQL.
- PostgreSQL is strictly for configuration (Users, Projects, RBAC Memberships, S3 credentials, Laravel DSNs, and watermarks).
- To filter and query data for the UI, the Go server must embed **DuckDB** to execute SQL queries directly against the Parquet files in S3/MinIO.

### 2. Parquet Partitioning & Batching
- **Partitioning Scheme:** Parquet files MUST be partitioned in S3/MinIO using the exact structure: `projectId={id}/source={source}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/` (e.g. `source=laravel_telescope`, `source=aurora_das`, `source=postgres_audit`). Schema evolution is handled via internal Parquet metadata, NOT path-based versioning.
- **Batching Strategy:** The `telescope:sync` collector flushes Parquet files based on a hybrid trigger: **10,000 rows**, a **1-hour time window**, OR a **100MB file size**. We do NOT want thousands of tiny KB-sized files.

### 3. CLI Structure
There is only **ONE** executable binary: `periscope`.
- `cmd/periscope/main.go` is the entry point.
- All functional boundaries are built as Cobra subcommands (e.g., `periscope serve`, `periscope telescope:sync [project_iid]`).

### 4. Database Identifiers (`iid` vs `id`)
- **CRITICAL RULE:** The `id` (integer) columns in PostgreSQL (e.g., in `users` and `projects` tables) are strictly for internal database foreign-key relationships.
- **NEVER** expose the integer `id` to the API or Frontend. You MUST always use the `iid` (UUID v7/v4) column for public identification, routing, and API payloads.

### 5. API Response Standards
- All HTTP endpoints MUST return data using the standard JSON envelope defined in `docs/api_standards.md` (`{ success, data, error, meta }`).
- Use the helper functions in `internal/api/response.go` to construct these responses. Do not write manual `json.NewEncoder(w)` logic in the route handlers.

### 6. Frontend Architecture
- **API Calls:** Never use raw `fetch()` or `axios` directly inside React components. All network requests must go through the typed wrappers in `ui/src/api/` and utilize `@tanstack/react-query`.
- **RBAC UI:** The sidebar navigation (`RootLayout.tsx`) relies on an `allowedRoles` array. Ensure new routes are properly gated by the user's role (`viewer`, `member`, `admin`).
- **Theming:** The application utilizes a customized dark-mode aesthetic. Only use standard Tailwind utility classes or Shadcn components. Do not write custom CSS unless absolutely necessary for the theme variables in `index.css`.

### 7. Security
- Target Laravel Database Connection Strings (DSNs) and S3 Secrets MUST be encrypted in the PostgreSQL `projects` table using AES-GCM. 
- The encryption master key is injected into the server via the `AES_ENCRYPTION_KEY` environment variable. Never store plain text credentials in the metadata DB.

### 8. Multi-Tenancy & RBAC
- All views, API endpoints, and collector routines are strictly isolated by `project_iid` (which maps to the `iid` column).
- Ensure role-based checks (Admin, Member, Viewer) are enforced via middleware on all API routes based on the `project_memberships` table.

## 🧪 Development Workflow

- **Local Infrastructure:** Use `docker-compose up -d --build` to spin up PostgreSQL and MinIO.
- **Go Hot-Reloading:** The Go server runs inside Docker using `air-verse/air` on port `8006`.
- **UI Compilation:** Ultimately, the frontend will be built and shipped inside the single Go binary using `embed.FS`.

## 📦 Versioning & Releases (Release Please)
**CRITICAL RULE:** All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/) specification (e.g., `feat:`, `fix:`, `chore:`, `docs:`).
- This repository utilizes Google's **Release Please** to automatically version the Go binary and generate changelogs based on commit history.

*By strictly following these guidelines, you will ensure `periscope` remains a performant, enterprise-grade telemetry lakehouse!*
