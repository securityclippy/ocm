# OCM - OpenClaw Credential Manager

A secure credential management sidecar for OpenClaw instances.

## Overview

OCM solves a fundamental security problem with AI agents: **credentials in the agent's environment can be compromised**. By moving credential management to a separate service with just-in-time elevation:

- **True isolation** — Agent can't access what it doesn't have
- **Human oversight** — Sensitive operations require explicit approval
- **Time-limited access** — Credentials auto-expire after approval
- **Visibility** — Dashboard shows all credentials and active elevations
- **Audit trail** — Full log of what was accessed, when, and by whom

## Architecture

```
┌─────────────────────┐         ┌─────────────────────────┐
│     OpenClaw        │         │          OCM            │
│     Gateway         │◄───────►│     (Go sidecar)        │
│                     │  :9999  │                         │
│  Agent requests     │         │  • Credential store     │
│  credentials from   │         │  • Elevation workflow   │
│  OCM via env vars   │         │  • Admin UI (:8080)     │
│                     │         │  • Gateway integration  │
└─────────────────────┘         └─────────────────────────┘
         │                                  │
         │      On approval, OCM writes     │
         │      credentials to .env and     │
         └──────── restarts Gateway ────────┘
```

## Quick Start

### One-Command Setup

```bash
# OCM standalone (no OpenClaw dependency)
./scripts/quickstart.sh docker-ocm

# OCM + OpenClaw (requires building OpenClaw image first)
./scripts/quickstart.sh docker

# Local development (requires Go + Node)
./scripts/quickstart.sh local
```

### Option 1: Docker - OCM Only (Easiest)

```bash
./scripts/docker.sh ocm-only

# Admin UI: http://localhost:8080
```

### Option 2: Docker - OCM + OpenClaw

```bash
./scripts/docker.sh

# If openclaw:local image doesn't exist, you'll be prompted:
#   "Build it now? (will clone and build from GitHub) [Y/n]"
# 
# Say yes - it clones OpenClaw to a temp dir, builds the image, cleans up.

# Admin UI: http://localhost:8080
# Gateway:  http://localhost:18789
```

Or build the OpenClaw image separately:
```bash
./scripts/build-openclaw.sh
./scripts/docker.sh
```

### Option 3: Local Development

**Prerequisites:** Go 1.22+, Node.js 20+, pnpm

```bash
# Setup and run
./scripts/dev.sh

# This will:
# - Check prerequisites
# - Generate master key (~/.ocm/master.key)
# - Install frontend dependencies
# - Build frontend + backend
# - Start the server

# Admin UI: http://localhost:8080
# Agent API: http://localhost:9999
```

### Manual Setup

If you prefer to do things manually:

```bash
# 1. Setup environment
cp .env.example .env
./ocm keygen --stdout  # Copy output to .env as OCM_MASTER_KEY

# 2. Build (requires just: https://github.com/casey/just)
just build

# 3. Run
./ocm serve
```

### Key Management

OCM uses a 256-bit master key to encrypt all stored credentials.

```bash
# Generate key to default location (~/.ocm/master.key)
./ocm keygen

# Generate to stdout (for Docker/env var)
./ocm keygen --stdout

# Generate to custom path
./ocm keygen -o /path/to/master.key

# Use custom key file
./ocm serve --master-key-file /path/to/master.key

# Or via environment variable
export OCM_MASTER_KEY=<64-hex-chars>
./ocm serve
```

## Admin UI

The web interface at `:8080` provides:

- **Dashboard** — Overview of credentials, pending requests, recent activity
- **Credentials** — Add/edit/delete with templates for common services:
  - Messaging: Discord, Telegram, Slack (bot or personal token)
  - AI Providers: OpenRouter, Anthropic, OpenAI, Groq
  - Tools: Brave Search, ElevenLabs, Deepgram
  - Integrations: Gmail, Google Calendar, Linear, GitHub, Twitter, Notion
- **Requests** — Approve or deny elevation requests with custom TTL
- **Audit Log** — Full history of all credential access

Each credential template includes setup instructions and links to documentation.

## API

### Agent API (`:9999`)

Limited surface area for agent use:

```
POST /api/v1/elevate
  Request elevation for a service/scope

GET /api/v1/elevate/:id
  Poll elevation status (pending/approved/denied)

GET /api/v1/credentials/:service/:scope
  Get credential value (if permanent or elevated)

GET /api/v1/scopes
  List available services and scopes
```

### Admin API (`:8080`)

Full credential management (UI backend):

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

## Configuration

```bash
./ocm serve \
  --agent-addr :9999 \           # Agent API (internal)
  --admin-addr :8080 \           # Admin UI (expose carefully)
  --db ocm.db \                  # SQLite database path
  --master-key-file ~/.ocm/master.key \  # Encryption key
  --gateway-url http://localhost:18789 \ # OpenClaw Gateway
  --env-file ~/.openclaw/.env    # Where to inject credentials
```

## Development

Requires [just](https://github.com/casey/just) (`brew install just` or `cargo install just`).

```bash
just build          # Build frontend + backend
just build-backend  # Backend only (faster)
just test           # Run tests
just dev            # Run without building
just clean          # Clean artifacts
just run            # Build backend + run
```

## Docker

### Standalone

```bash
docker build -t ocm:local .
docker run -d \
  -e OCM_MASTER_KEY=$(openssl rand -hex 32) \
  -p 8080:8080 \
  -v ocm-data:/data \
  ocm:local
```

### With OpenClaw

See `docker-compose.openclaw.yml` for the full setup:

```bash
cp .env.example .env
# Edit .env with your tokens

docker compose -f docker-compose.openclaw.yml up -d
```

This runs:
- OpenClaw Gateway on port 18789
- OCM Admin UI on port 8080
- Internal network for Agent API (9999)
- Shared volume for credential injection

## How It Works

1. **Store credentials** in OCM via Admin UI (encrypted at rest)
2. **Configure access**: permanent (always available) or requires-approval
3. **Agent requests** elevation when it needs a protected credential
4. **You approve** via Admin UI with a time limit (e.g., 30 min)
5. **OCM injects** the credential into OpenClaw's `.env` and restarts Gateway
6. **Credential expires** automatically, removed from environment

## Security

- **Encryption**: AES-256-GCM for all stored credentials
- **Key management**: Master key never touches the agent
- **Isolation**: Agent API has minimal surface area
- **Approval**: Human-in-the-loop for sensitive operations
- **TTL**: Auto-expiration prevents credential accumulation
- **Audit**: Complete log of all access and approvals

## License

MIT - See [LICENSE](LICENSE)
