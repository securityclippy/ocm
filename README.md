# OCM - OpenClaw Credential Manager

A secure credential management sidecar for OpenClaw instances.

## Quick Start

```bash
git clone https://github.com/openclaw/ocm.git
cd ocm
./scripts/quickstart.sh
```

That's it. This will:
1. Generate secure encryption keys
2. Build the OpenClaw image (if needed)
3. Build OCM
4. Start the full stack

**URLs after startup:**
- 🔐 **OCM Admin UI:** http://localhost:8080
- 🌐 **OpenClaw Gateway:** http://localhost:18789

### What's Next?

1. Open the **OCM Admin UI** to add your first credential
2. Connect to the **OpenClaw Gateway** from the web UI or your preferred client
3. Your agent can now use credentials without ever seeing them!

---

## Why OCM?

AI agents with credential access can be manipulated to misuse or exfiltrate them. **Rules aren't security boundaries.** OCM solves this by:

- **True isolation** — Agent can't access what it doesn't have
- **Human oversight** — Sensitive operations require explicit approval
- **Time-limited access** — Credentials auto-expire after approval
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

## Alternative Setup Methods

### Docker - OCM Only (without OpenClaw)

If you already have OpenClaw running elsewhere:

```bash
./scripts/setup.sh
./scripts/docker.sh ocm-only

# Admin UI: http://localhost:8080
```

### Local Development

**Prerequisites:** Go 1.22+, Node.js 20+, pnpm

```bash
./scripts/dev.sh

# Admin UI: http://localhost:8080
# Agent API: http://localhost:9999
```

### Manual Setup

```bash
# 1. Setup environment
./scripts/setup.sh

# 2. Build (requires just: https://github.com/casey/just)
just build

# 3. Run
./ocm serve
```

### Cleanup & Reset

```bash
# Stop containers, remove OCM image
./scripts/clean.sh

# Stop all, remove all images (OCM + OpenClaw)
./scripts/clean.sh all

# Remove data volumes (⚠️ deletes credentials!)
./scripts/clean.sh volumes

# Complete teardown (⚠️ deletes everything!)
./scripts/clean.sh full

# Nuclear option: teardown + fresh start
./scripts/reset.sh
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

### Security Notes

**Back up your `.env` file!** The master key encrypts all stored credentials. If lost, they cannot be recovered.

**Protect the Admin UI.** It has full credential access. In production:
- Run behind a reverse proxy with authentication
- Or bind to localhost only and use SSH tunneling
- Never expose directly to the internet

**Container permissions:**
- OCM runs as non-root (UID 1000) by default
- The `.env` file is created with mode 600 (owner read/write only)
- Config directories are mode 700

## Troubleshooting

### "Master key not found"

Run setup to generate keys:
```bash
./scripts/setup.sh
```

### "Permission denied" on database

The volume may have wrong ownership. Fix with:
```bash
docker compose -f docker-compose.openclaw.yml down
docker volume rm ocm_ocm-data
docker compose -f docker-compose.openclaw.yml up -d
```

### Credentials not appearing in OpenClaw

1. Check the `.env` file exists:
   ```bash
   docker exec openclaw cat /home/node/.openclaw/.env
   ```

2. Check the env var is loaded:
   ```bash
   docker exec openclaw env | grep YOUR_VAR_NAME
   ```

3. If file exists but var doesn't, restart OpenClaw:
   ```bash
   docker restart openclaw
   ```

### Existing OpenClaw Installation

If you already have OpenClaw running and want to add OCM:

1. Note your OpenClaw config directory (usually `~/.openclaw`)
2. Set it before running setup:
   ```bash
   export OPENCLAW_CONFIG_DIR=/path/to/your/.openclaw
   ./scripts/setup.sh
   ```
3. The setup will detect and use your existing configuration

## License

MIT - See [LICENSE](LICENSE)
