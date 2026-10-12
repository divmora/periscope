.PHONY: build clean test test-race test-coverage dev-setup fmt lint ui-install ui-build ui-dev docker-build docker-build-multiarch

build:
	@go build -o bin/periscope ./cmd/periscope

test:
	@go test -v ./...

test-race:
	@go test -v -race ./...

test-coverage:
	@go test -v -coverprofile=coverage.out ./...

fmt:
	@go fmt ./...

lint:
	@golangci-lint run

clean:
	@rm -rf bin/ coverage.out

dev-setup:
	@go mod download

ui-install:
	@cd ui && pnpm install

ui-build:
	@cd ui && pnpm build

ui-dev:
	@cd ui && pnpm dev

docker-build:
	@docker build -t periscope:latest .

docker-build-multiarch:
	@docker buildx build --platform linux/amd64,linux/arm64 -t periscope:latest .
