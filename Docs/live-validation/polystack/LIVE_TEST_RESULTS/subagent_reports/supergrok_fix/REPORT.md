# SuperGrok / xai-oauth authentication fix report

**Date:** 2026-07-13  
**Repo:** `C:\CodeProjects\codeNERD`  
**Scope:** Robust SuperGrok OAuth auth — quarantine recovery, re-import, API-key fallback, clear status/errors

## Problems observed (live matrix)

| Symptom | Root cause |
|---------|------------|
| `xai-oauth: no credentials found` / `authentication required` | Empty store or Load failed without actionable recovery text |
| `invalid_grant: Refresh token has been revoked` while `auth status` still said engine=`xai-oauth` | Terminal refresh quarantined tokens; status didn't surface `needs_reauth` / `api_fallback` clearly |
| Probe told users `nerd auth grok` but tokens existed and stayed stuck | Quarantined local store blocked Load; no re-import preference over quarantine |
| `engine=xai-oauth` + broken refresh + present `xai_api_key` still hard-failed | Fallback existed partially; not gated by config default / loud WARNING path fully |

## Fixes implemented

### 1. Clear, actionable errors on revoked refresh

- `AuthRequiredError` appends `AuthRecoveryHelp(detail)`:
  - Run `nerd auth grok`
  - Delete `~/.nerd/xai_oauth.json` when quarantined/revoked
  - Or use `engine=api` + `xai_api_key`
- Terminal refresh (`invalid_grant` / revoked) still **quarantines** credentials on disk so retries don't spin forever.

### 2. On `invalid_grant`: quarantine + optional Grok CLI re-import

`TokenSource.refreshLocked`:

1. Marks credentials quarantined + persists reason  
2. If `ImportGrokAuth` (default true), tries `importGrokIfBetterLocked`  
3. If import yields a **different** refresh token, retries refresh once  
4. If import has a still-valid access token, uses it without refresh  
5. Else returns `AuthRequiredError` with recovery help  

### 3. Prefer successful re-import over stuck quarantined tokens

`TokenSource.Load()`:

- Non-quarantined local store → use as-is  
- Missing or quarantined → try Grok CLI import (`~/.grok/auth.json`)  
- Prefer import when refresh/access tokens differ, or local is quarantined  
- Same revoked refresh + expired access → do **not** re-import junk  

`PrepareForReauth()` (used by `nerd auth grok`):

- Deletes local credential file  
- Clears in-memory tokens so device login / fresh import can proceed  

### 4. Boot-time API key fallback (loud WARNING)

Config: `xai_oauth.fallback_to_api_key` (`*bool`, **default true** when nil).

`newSuperGrokClientOrAPIFallback` (`internal/perception/client_factory.go`):

1. Build OAuth client  
2. `Load()` credentials (includes re-import attempt)  
3. On hard auth failure + fallback enabled + `xai_api_key` / `XAI_API_KEY` present → return metered `XAIClient` with **WARNING** log  
4. Else keep OAuth client and warn to run `nerd auth grok`  

Disable: `"xai_oauth": { "fallback_to_api_key": false }`

### 5. `nerd auth status` real probe labels

Status taxonomy (`xaioauth.AuthStatus`):

| Status | Meaning |
|--------|---------|
| `ok` | SuperGrok OAuth ready |
| `needs_reauth` | Missing/revoked/quarantined; no usable API fallback |
| `api_fallback` | OAuth broken but API key present + fallback enabled |
| `degraded` | Transient probe failure (rate limit / network) |

`printSuperGrokAuthStatus` prints Status, Probe, Message, Quarantined, Import/Fallback flags, and next action.

### 6. Tests (no secrets)

Package: `internal/perception/xaioauth`

- `TestLoad_QuarantinedSameToken_FailsWithAuthRequired`
- `TestLoad_QuarantinedReimportsDifferentGrokToken`
- `TestPrepareForReauth_DeletesStore`
- `TestRefresh_InvalidGrant_QuarantinesAndClearError`
- `TestRefresh_InvalidGrant_ReimportsFreshGrokToken`
- `TestDeriveAuthStatus`
- `TestAuthRequiredError_IncludesRecoveryHelp`
- Plus existing refresh / store / import / chat tests

```
go test ./internal/perception/xaioauth/ -count=1   # PASS
go test ./internal/config/ -count=1                # PASS
go test ./internal/perception/ -count=1            # PASS
```

## Files changed

| Path | Change |
|------|--------|
| `internal/perception/xaioauth/token.go` | Load re-import-over-quarantine; refresh invalid_grant reimport; `PrepareForReauth`; `HasUsableCredentials` |
| `internal/perception/xaioauth/errors.go` | Actionable `AuthRequiredError` + `AuthRecoveryHelp` + `IsTerminalRefreshFailure` |
| `internal/perception/xaioauth/probe.go` | `AuthStatus`, `DeriveAuthStatus`, clearer probe messages |
| `internal/perception/xaioauth/token_quarantine_test.go` | Quarantine / reimport / invalid_grant tests |
| `internal/config/llm.go` | `FallbackToAPIKey` on `XAIOAuthConfig` |
| `internal/config/user_config.go` | Default `FallbackToAPIKey=true` |
| `internal/perception/client_factory.go` | Loud OAuth→API fallback; carry xai key for xai-oauth only |
| `cmd/nerd/cmd_auth.go` | Status `ok`/`needs_reauth`/`api_fallback`; reauth recovery already wired via `PrepareForReauth` |

## How users re-auth

### Preferred: SuperGrok OAuth

```powershell
# Option A: Grok CLI already logged in → import + probe
nerd auth grok

# Option B: if store is stuck quarantined, auth grok clears it and device-logins
nerd auth grok
# Follow browser device-code URL / user code
```

Manual clear if needed:

```powershell
Remove-Item $env:USERPROFILE\.nerd\xai_oauth.json -ErrorAction SilentlyContinue
# Optional: refresh Grok CLI store first
# grok login
nerd auth grok
```

Verify:

```powershell
nerd auth status
# Expect: Status: ok   Probe: ready
```

### Metered fallback (no SuperGrok session)

In `.nerd/config.json` (or user config path used by CLI):

```json
{
  "engine": "api",
  "provider": "xai",
  "xai_api_key": "<key>"
}
```

Or keep `engine=xai-oauth` with key present and default `fallback_to_api_key=true` — boot will log WARNING and use metered xAI until OAuth is fixed.

## Fallback behavior (summary)

```
engine=xai-oauth
  → Load local tokens
  → if quarantined/missing: try ~/.grok/auth.json import (if import_grok_auth)
  → if still hard-auth fail:
       if fallback_to_api_key (default true) && (xai_api_key || XAI_API_KEY):
           WARNING log → use metered XAIClient
       else:
           WARNING log → OAuth client; requests fail with AuthRequired + recovery help
```

Runtime refresh path:

```
AccessToken needs refresh
  → refresh_token grant
  → 4xx invalid_grant: quarantine on disk
  → try Grok CLI re-import once
  → success → continue; failure → AuthRequiredError + help text
```

## Secrets policy

- No secrets committed  
- Tests use synthetic tokens only (`dead-refresh`, `fresh-refresh`, mock HTTP servers)  
- Credential files remain `0600` under `~/.nerd/`  

## Residual notes

- `cmd/nerd` full package test compile may fail on unrelated tree issues (e.g. `internal/core/shards` missing symbols in this workspace snapshot); **xaioauth / perception / config tests are green**.  
- API fallback is **metered** — users should still re-auth SuperGrok for subscription usage.  
- Tier 403 (`ProbeTierForbidden`) still requires `engine=api` + key (subscription may not entitle OAuth inference).  
