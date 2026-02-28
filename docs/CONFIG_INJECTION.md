# Config Injection Spec

## Overview

OCM needs to inject credentials into two places:
1. **Environment variables** (`.env` file) - for tokens with env var support
2. **OpenClaw config** (`openclaw.json`) - for config-only tokens

This document specifies how OCM handles both injection types.

## Service Template Schema

### Current Schema

```typescript
interface ServiceTemplate {
  name: string;
  category: string;
  description: string;
  fields: FieldDefinition[];
  envVar?: string;           // Single env var (legacy)
  requiredScopes?: string[]; // For OAuth-style services
}
```

### Extended Schema

```typescript
interface ServiceTemplate {
  name: string;
  category: string;
  description: string;
  fields: FieldDefinition[];
  
  // Injection target (replaces envVar)
  injection: InjectionTarget;
  
  // Optional: secondary injections (e.g., refresh tokens)
  secondaryInjections?: Record<string, InjectionTarget>;
  
  requiredScopes?: string[];
}

type InjectionTarget = EnvInjection | ConfigInjection;

interface EnvInjection {
  type: "env";
  var: string;              // e.g., "SLACK_BOT_TOKEN"
}

interface ConfigInjection {
  type: "config";
  path: string;             // JSON path, e.g., "channels.slack.userToken"
}
```

### Field-Level Injection

For services with multiple tokens that go to different places:

```typescript
interface FieldDefinition {
  key: string;
  label: string;
  type: "text" | "password" | "select";
  required?: boolean;
  placeholder?: string;
  
  // Per-field injection override
  injection?: InjectionTarget;
}
```

If a field has its own `injection`, it overrides the service-level default.

## Examples

### Slack (Multiple Tokens, Mixed Injection)

```typescript
{
  name: "Slack",
  category: "messaging",
  description: "Slack workspace connection",
  fields: [
    {
      key: "botToken",
      label: "Bot Token (xoxb-...)",
      type: "password",
      required: true,
      injection: { type: "env", var: "SLACK_BOT_TOKEN" }
    },
    {
      key: "appToken",
      label: "App Token (xapp-...)",
      type: "password",
      required: true,
      injection: { type: "env", var: "SLACK_APP_TOKEN" }
    },
    {
      key: "userToken",
      label: "User Token (xoxp-...) - for reading your messages",
      type: "password",
      required: false,
      injection: { type: "config", path: "channels.slack.userToken" }
    }
  ],
  injection: { type: "env", var: "SLACK_BOT_TOKEN" }  // Default (legacy compat)
}
```

### Simple Service (Single Env Var)

```typescript
{
  name: "OpenRouter",
  category: "ai",
  description: "OpenRouter API access",
  fields: [
    { key: "apiKey", label: "API Key", type: "password", required: true }
  ],
  injection: { type: "env", var: "OPENROUTER_API_KEY" }
}
```

### Config-Only Service

```typescript
{
  name: "Custom Integration",
  category: "integrations",
  description: "Custom API integration",
  fields: [
    { key: "apiKey", label: "API Key", type: "password", required: true },
    { key: "endpoint", label: "API Endpoint", type: "text", required: true }
  ],
  injection: { type: "config", path: "integrations.custom.apiKey" },
  secondaryInjections: {
    "endpoint": { type: "config", path: "integrations.custom.endpoint" }
  }
}
```

## Injection Logic

### Write Flow

```
User saves credential in OCM UI
        │
        ▼
┌─────────────────────────┐
│  Store encrypted in DB  │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  For each field:        │
│  - Get injection target │
│  - Collect by type      │
└───────────┬─────────────┘
            │
            ▼
    ┌───────┴───────┐
    │               │
    ▼               ▼
┌────────┐    ┌──────────┐
│  .env  │    │  config  │
│ writes │    │ patches  │
└────┬───┘    └────┬─────┘
     │             │
     └──────┬──────┘
            │
            ▼
┌─────────────────────────┐
│   Restart Gateway       │
│   (single restart for   │
│    all changes)         │
└─────────────────────────┘
```

### Config Patch Format

OCM calls `config.patch` RPC with a JSON5 payload:

```json5
{
  channels: {
    slack: {
      userToken: "xoxp-..."
    }
  }
}
```

For nested paths like `channels.slack.userToken`, OCM builds the nested object structure.

### Credential Removal

When a credential is deleted:
- **Env injection**: Remove line from `.env`
- **Config injection**: Patch with `null` to delete the key

```json5
{
  channels: {
    slack: {
      userToken: null  // Removes the key
    }
  }
}
```

## Implementation Checklist

### Backend Changes

1. [ ] Update `ServiceTemplate` type in `internal/store/types.go`
2. [ ] Add `InjectionTarget` types
3. [ ] Update credential writer to handle both injection types
4. [ ] Add config patch builder (path → nested object)
5. [ ] Batch changes: collect all env + config changes, then single restart
6. [ ] Handle removal for both types

### Frontend Changes

1. [ ] Update `serviceTemplates.ts` with new schema
2. [ ] Update credential form to show injection info (optional)
3. [ ] No major UI changes needed (injection is backend concern)

### Service Template Updates

Update all templates in `web/src/lib/serviceTemplates.ts`:
- Add `injection` field to each template
- For Slack: add per-field injection for userToken
- Ensure backwards compatibility (envVar still works)

## Security Considerations

1. **Config file permissions**: OpenClaw config may contain other settings. OCM only patches specific paths, never overwrites the whole file.

2. **Sensitive data in config**: Config injection means tokens appear in `openclaw.json`. This is already the case for userToken today. The config file should have restricted permissions (600).

3. **Audit trail**: Log all config patches with the paths modified (not values).

4. **Validation**: Validate paths before patching. Don't allow arbitrary paths that could break config.

## Migration

Existing credentials with `envVar` continue to work. The system treats:
```typescript
{ envVar: "FOO" }
```
as equivalent to:
```typescript
{ injection: { type: "env", var: "FOO" } }
```

New credentials should use the `injection` field.
