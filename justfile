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
    go build -o ocm .

# Build backend only (skip frontend)
build-backend:
    go build -o ocm .

# Run locally (backend only)
run: build-backend
    ./ocm serve

# Development mode
dev:
    go run . serve

# Clean build artifacts
clean:
    rm -f ocm
    rm -rf web/build/
    rm -rf web/node_modules/
    rm -rf web/.svelte-kit/

# Release build (optimized)
release: web
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X github.com/openclaw/ocm/cmd.Version={{VERSION}} -X github.com/openclaw/ocm/cmd.Commit={{COMMIT}}" -o ocm-linux-amd64 .
    CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ocm-darwin-amd64 .
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ocm-darwin-arm64 .

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
    OCM_MASTER_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef ./ocm serve
