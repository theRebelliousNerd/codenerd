---
name: antigravity-auth
description: This skill provides instructions and templates for implementing authentication and API client integration for Google's Antigravity (Unified Gateway API) in the NERDide Go backend. This skill should be used when implementing "Antigravity" or "Google CloudCode" authentication, adding support for `gemini-3-pro` or `claude-sonnet-4-5` (via Google), replicating opencode-antigravity-auth functionality, or connecting to `cloudcode-pa.googleapis.com`.
---

# Antigravity Auth Implementation

This skill enables NERDide to authenticate using Google OAuth credentials and access models like `claude-sonnet-4-5-thinking` and `gemini-3-pro` via the CloudCode PA gateway.

## Quick Reference

### OAuth Credentials (Public - embedded in official client)
```
Client ID:     1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com
Client Secret: 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
Redirect URI:  http://localhost:51121/oauth-callback
```

### Endpoints
- **Primary (Sandbox)**: `https://daily-cloudcode-pa.sandbox.googleapis.com`
- **Production**: `https://cloudcode-pa.googleapis.com`

### Required Scopes
- `https://www.googleapis.com/auth/cloud-platform`
- `https://www.googleapis.com/auth/userinfo.email`
- `https://www.googleapis.com/auth/userinfo.profile`
- `https://www.googleapis.com/auth/cclog`
- `https://www.googleapis.com/auth/experimentsandconfigs`

## Implementation Checklist

### 1. OAuth Flow (with PKCE)
- [ ] Implement Authorization Code flow with PKCE (S256).
- [ ] Use the public Client ID/Secret above.
- [ ] Store the `state` and `code_verifier` in a secure cookie/session.
- [ ] On callback, exchange the code for tokens using `code_verifier`.

### 2. Project ID Discovery
- [ ] After token exchange, call `/v1internal:loadCodeAssist` to discover the user's Google Cloud Project ID.
- [ ] If discovery fails, use the default project ID: `rising-fact-p41fc`.
- [ ] Store the Project ID alongside the tokens.

### 3. Token Storage
- [ ] Store `RefreshToken`, `AccessToken`, `Expiry`, `ProjectID`, and `Email` in BadgerDB (encrypted).
- [ ] Format: `refresh_token|project_id|managed_project_id` (pipe-separated).

### 4. Token Refresh
- [ ] Check if `AccessToken` is expired (with 60s buffer).
- [ ] If expired, use the `RefreshToken` to get a new `AccessToken` via `https://oauth2.googleapis.com/token`.

### 5. API Client
- [ ] Inject required headers (see `references/constants.md`).
- [ ] Transform requests to Gemini-style `contents` array format (even for Claude).
- [ ] Strip unsupported JSON schema fields (`const`, `$ref`, `$defs`).
- [ ] Strip "thinking" blocks from outgoing requests to avoid signature errors.

### 6. Multi-Account & Rate Limiting
- [ ] Support multiple Google accounts per user.
- [ ] Implement health-score-based rotation (see `logic_source/rotation.ts`).
- [ ] On 429 errors, automatically switch to the next healthy account.

## Troubleshooting Reference

| Error | Cause | Solution |
|-------|-------|----------|
| 403 `Permission Denied` | Invalid Project ID or API not enabled | Re-discover project via `loadCodeAssist` or use default |
| 429 `Rate Limited` | Quota exhausted | Rotate to another account |
| `Invalid signature` | Corrupted thinking block | Strip all thinking blocks before sending |
| `Unknown field: const` | JSON schema uses `const` | Convert `const` to `enum: [value]` |
| `tool_use without tool_result` | Interrupted execution | Inject synthetic `tool_result` for recovery |

See `references/original_docs/TROUBLESHOOTING.md` for more details.

## File Map

| File | Purpose |
|------|---------|
| `assets/templates/auth_handler.go` | Go template for OAuth handlers with PKCE and project discovery |
| `assets/templates/client.go` | Go template for API client with header injection |
| `references/constants.md` | All constants (credentials, endpoints, headers) |
| `references/api_spec.md` | Request/response format specification |
| `references/original_docs/` | Original documentation from the TypeScript repo |
| `references/logic_source/` | Original TypeScript source for algorithm reference |

## Key Algorithms to Port

1. **Request Transformation** (`logic_source/request.ts`, `logic_source/transform/`)
   - Convert messages to Gemini `contents` format.
   - Handle thinking config injection.
   - Clean JSON schemas.

2. **Thinking Block Handling** (`logic_source/thinking-recovery.ts`)
   - Strip ALL thinking blocks from outgoing requests.
   - Cache thinking signatures for tool-use injection.

3. **Account Rotation** (`logic_source/rotation.ts`)
   - Health score tracking.
   - LRU-based selection.
   - Automatic failover on rate limits.

4. **Token Refresh** (`logic_source/auth.ts`)
   - 60-second buffer before expiry.
   - Packed format: `refresh_token|project_id|managed_project_id`.
