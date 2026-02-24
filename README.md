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

```yaml
services:
  ocm:
    image: openclaw/ocm:latest
    volumes:
      - ocm-data:/data
    environment:
      - OCM_MASTER_KEY_FILE=/run/secrets/ocm_key
    ports:
      - "127.0.0.1:8080:8080"
    secrets:
      - ocm_key
```

## Security

- Credentials encrypted at rest (AES-256-GCM)
- Master key from environment or file
- Elevation requires out-of-band human approval
- Time-limited access with auto-revocation
- Full audit trail

## License

MIT
