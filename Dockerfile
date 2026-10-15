# Stage 1: Build Frontend Assets
FROM node:26-alpine AS ui-builder
WORKDIR /app/ui
RUN npm install -g pnpm
COPY ui/package.json ui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY ui/ ./
RUN pnpm build

# Stage 2: Build Static Go Binary
FROM golang:1.26-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/ui/dist ./ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/periscope ./cmd/periscope

# Stage 3: Minimal Runtime with DuckDB Support
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata curl unzip \
    && curl -sL -o /tmp/duckdb.zip https://github.com/duckdb/duckdb/releases/download/v1.1.3/duckdb_cli-linux-amd64.zip \
    && unzip -q -o /tmp/duckdb.zip -d /usr/local/bin \
    && chmod +x /usr/local/bin/duckdb \
    && rm -rf /tmp/duckdb.zip /var/lib/apt/lists/*
WORKDIR /app
COPY --from=go-builder /bin/periscope /usr/local/bin/periscope

EXPOSE 8006
ENTRYPOINT ["/usr/local/bin/periscope"]
CMD ["serve"]
