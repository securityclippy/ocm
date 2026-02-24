.PHONY: build web test run clean dev

# Default target
all: build

# Build frontend (uses pnpm for reliability)
web:
	cd web && pnpm install && pnpm build

# Run tests
test:
	go test -v ./...

# Build Go binary (includes embedded frontend)
build: web
	go build -o bin/ocm ./cmd/ocm

# Build without frontend (for backend dev)
build-backend:
	go build -o bin/ocm ./cmd/ocm

# Run locally
run: build-backend
	./bin/ocm

# Development mode (backend only, no embed)
dev:
	go run ./cmd/ocm

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/build/
	rm -rf web/node_modules/
	rm -rf web/.svelte-kit/

# Release build (optimized, all platforms)
release: web
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/ocm-linux-amd64 ./cmd/ocm
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/ocm-darwin-amd64 ./cmd/ocm
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/ocm-darwin-arm64 ./cmd/ocm

# Format code
fmt:
	go fmt ./...
	cd web && npm run format

# Lint
lint:
	go vet ./...
	cd web && npm run lint
