# OCM - OpenClaw Credential Manager

A secure credential management sidecar for OpenClaw instances.

## Overview

OCM solves a fundamental security problem with AI agents: **credentials that exist in the agent's environment can be compromised**. By moving credential management to a separate service with just-in-time elevation, we get:

- **True isolation** — Agent can't access what it doesn't have
- **Human oversight** — Write actions require explicit approval
- **Time-limited access** — Credentials auto-expire
- **Visibility** — Dashboard shows all permissions at a glance
- **Audit trail** — Full log of what was accessed and when

## Architecture

```
┌─────────────────────┐      ┌─────────────────────────┐
│     OpenClaw        │      │         OCM             │
│                     │◄────►│    (Go sidecar)         │
│  - Calls OCM for    │:9999 │                         │
│    credentials      │      │  - Credential store     │
│                     │      │  - Elevation workflow   │
│                     │      │  - Admin UI (:8080)     │
└─────────────────────┘      └─────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20+ (for frontend)
- SQLite3

### Build

```bash
# Install dependencies and build everything
make build

# Or build backend only (faster iteration)
make build-backend
```

### Run

```bash
# Generate a master key
openssl rand -hex 32 > ocm-master.key

# Run with key file
./bin/ocm --master-key-file ocm-master.key

# Or via environment
export OCM_MASTER_KEY=$(cat ocm-master.key)
./bin/ocm
```

### Configuration

```bash
./bin/ocm \
  --agent-addr :9999 \      # Agent API (internal)
  --admin-addr :8080 \      # Admin UI (external)
  --db ocm.db \             # Database path
  --master-key-file key.txt # Encryption key
```

## API

### Agent API (`:9999`)

Limited surface area — agents can only:

```
POST /api/v1/elevate
  Request elevation for a scope

GET /api/v1/elevate/:id
  Check elevation status

GET /api/v1/credentials/:service/:scope
  Get credential (if permanent or elevated)

GET /api/v1/scopes
  List available services/scopes
```

### Admin API (`:8080`)

Full credential management:

```
GET    /admin/api/dashboard
GET    /admin/api/credentials
POST   /admin/api/credentials
PUT    /admin/api/credentials/:service
DELETE /admin/api/credentials/:service

GET  /admin/api/requests
POST /admin/api/requests/:id/approve
POST /admin/api/requests/:id/deny
POST /admin/api/revoke/:service/:scope

GET /admin/api/audit
```

## Development

```bash
# Run tests
make test

# Development mode (no embedded frontend)
make dev

# Format code
make fmt

# Lint
make lint
```

## Docker

### OCM Standalone

```bash
# Build the image
docker build -t ocm:local .

# Generate master key
docker run --rm ocm:local keygen --stdout > .env
# Edit .env to set OCM_MASTER_KEY=<the generated key>

# Run
docker compose up -d
```

### With OpenClaw (Recommended)

Run OCM alongside OpenClaw using the combined compose file:

```bash
# Copy and edit environment
cp .env.example .env
# Fill in OPENCLAW_GATEWAY_TOKEN and OCM_MASTER_KEY

# Build OCM and start both services
docker compose -f docker-compose.openclaw.yml up -d

# View logs
docker compose -f docker-compose.openclaw.yml logs -f
```

This setup:
- Runs OpenClaw Gateway on port 18789
- Runs OCM Admin UI on port 8080
- OCM Agent API (9999) is internal to Docker network
- Credentials injected via shared `.env` file
- OCM triggers Gateway restart after approval

```yaml
# docker-compose.openclaw.yml (excerpt)
services:
  openclaw:
    image: ghcr.io/anthropics/openclaw:latest
    depends_on: [ocm]
    environment:
      OCM_AGENT_URL: http://ocm:9999
    
  ocm:
    build: .
    environment:
      OCM_MASTER_KEY: ${OCM_MASTER_KEY}
      OCM_GATEWAY_URL: http://openclaw:18789
```

## Security

- Credentials encrypted at rest (AES-256-GCM)
- Master key from environment or file
- Elevation requires out-of-band human approval
- Time-limited access with auto-revocation
- Full audit trail

## License

MIT
