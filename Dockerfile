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

# Stage 3: Minimal Runtime
FROM alpine:3.22
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /bin/periscope /usr/local/bin/periscope

EXPOSE 8006
ENTRYPOINT ["/usr/local/bin/periscope"]
CMD ["serve"]
