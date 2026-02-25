# OCM build tasks
# Install just: https://github.com/casey/just

# Default recipe
default: build

# Build frontend
web:
    cd web && pnpm install && pnpm build

# Run tests
test:
    go test -v ./...

# Build Go binary with embedded frontend
build: web
    go build -o bin/ocm ./cmd/ocm

# Build backend only (skip frontend)
build-backend:
    go build -o bin/ocm ./cmd/ocm

# Run locally (backend only)
run: build-backend
    ./bin/ocm

# Development mode
dev:
    go run ./cmd/ocm

# Clean build artifacts
clean:
    rm -rf bin/
    rm -rf web/build/
    rm -rf web/node_modules/
    rm -rf web/.svelte-kit/

# Release build (optimized)
release: web
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/ocm-linux-amd64 ./cmd/ocm
    CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/ocm-darwin-amd64 ./cmd/ocm
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/ocm-darwin-arm64 ./cmd/ocm

# Format code
fmt:
    go fmt ./...
    cd web && pnpm format

# Lint
lint:
    go vet ./...
    cd web && pnpm lint

# Run with test credentials (for quick testing)
test-run: build-backend
    @echo "Starting OCM with test key..."
    OCM_MASTER_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef ./bin/ocm
