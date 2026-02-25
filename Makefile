# OCM Makefile
# For those who prefer make over just

.PHONY: all build web build-backend test run dev clean fmt lint docker

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

all: build

# Build frontend
web:
	cd web && pnpm install && pnpm build
	rm -rf internal/web/build/_app internal/web/build/*.html internal/web/build/*.png
	cp -r web/build/* internal/web/build/

# Build Go binary with embedded frontend
build: web
	go build -ldflags="-X github.com/openclaw/ocm/cmd.Version=$(VERSION) -X github.com/openclaw/ocm/cmd.Commit=$(COMMIT)" -o ocm .

# Build backend only (skip frontend rebuild)
build-backend:
	go build -o ocm .

# Run tests
test:
	go test -v ./...

# Run locally
run: build-backend
	./ocm serve

# Development mode (no build)
dev:
	go run . serve

# Clean build artifacts
clean:
	rm -f ocm ocm-*
	rm -rf web/build web/node_modules web/.svelte-kit

# Format code
fmt:
	go fmt ./...
	cd web && pnpm format 2>/dev/null || true

# Lint
lint:
	go vet ./...
	cd web && pnpm lint 2>/dev/null || true

# Build Docker image
docker:
	docker build -t ocm:local .

# Generate master key
keygen:
	@openssl rand -hex 32
