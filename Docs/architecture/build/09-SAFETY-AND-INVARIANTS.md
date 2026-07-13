# 09 — Safety and Invariants: `internal/build`

> Last verified: **2026-07-13**

---

## 1. Role in the safety architecture

| Layer | Responsibility |
|-------|----------------|
| Mangle `permitted(...)` | Whether an action may run at all |
| VirtualStore / shards | Dispatch and tool selection |
| tactile / shell policy | Binary allowlists, sandboxes, timeouts |
| **`internal/build`** | **What environment a Go subprocess inherits** |

This package is a **data hygiene** control, not an authorization control. It cannot deny a compile; it can avoid leaking env and can make CGO/cache behavior predictable.

---

## 2. Safety properties (positive)

### 2.1 Filtered environment by default

Does not start from full `os.Environ()`. Starts from an essential allowlist of Go/OS variables. Reduces accidental leakage of:

- Cloud credentials in uncommon env vars  
- User-specific secrets not on the essential/whitelist lists  
- Irrelevant ambient config that makes builds non-reproducible  

### 2.2 Explicit whitelist expansion

Additional process vars only enter when:

- Named in `Execution.AllowedEnvVars` and non-empty in the process, or  
- Explicitly set in `Build.EnvVars` config  

### 2.3 Caller-enforced CGO disable for tools

Autopoiesis forces `CGO_ENABLED=0` via `MergeEnv` for generated tool compile and thunderdome arena. That limits:

- Linking against unexpected system libraries  
- CGO-based escape surface in sandboxed tool builds  
- Non-portable binaries  

### 2.4 No network in this package

Filesystem `Stat` only for local header dirs. No HTTP, no downloads.

---

## 3. Safety limitations (honest)

| Limitation | Risk |
|------------|------|
| Whitelist values may still be secrets | Debug logs print key names; values logged for build config (`key=val` in BuildDebug for config env) |
| Duplicate keys | Unclear which value wins if both base and append present — depends on Go’s last-wins env behavior |
| `nil` config skips whitelist logic | Callers may miss intentional env; also skips maliciously large whitelist if config were attacker-controlled |
| Detected `-I` paths from workspace | If workspace is untrusted, header path injection is by design (build the untrusted tree) |
| Does not sandbox the `go` binary itself | OS-level confinement is tactile/OS concern |
| `GoFlags` not applied | Cannot rely on config flags for hardening argv |

---

## 4. Invariants

### I1 — Prefix-true key membership

`hasEnvKey` only matches `KEY=`, never prefix-of-key (`FOO` ∉ match for `FOOBAR=1`). Tested.

### I2 — MergeEnv purity of base

`MergeEnv` must not mutate the input base slice. Tested.

### I3 — Malformed merge entries ignored

Entries without `=` are skipped. Tested.

### I4 — User CGO_CFLAGS wins over auto sqlite path

If config sets `CGO_CFLAGS`, `loadBuildConfig` does not overwrite with sqlite_headers path. Tested.

### I5 — Process GOFLAGS blocks default sqlite_vec tag

If process `GOFLAGS` is set, auto `-tags=sqlite_vec` is not injected into cfg.EnvVars. Tested.

### I6 — DefaultBuildConfig maps non-nil

Empty but non-nil `EnvVars`. Tested.

### I7 — No package-global mutable env cache

Each call recomputes. Supports concurrent callers.

---

## 5. Concurrency

Safe for concurrent `GetBuildEnv*` as pure-ish functions. Callers must not share and mutate the same returned slice across goroutines without coordination.

---

## 6. Mangle / constitutional surface

**None.** No `Decl`, no `permitted` derivation, no default-deny logic inside this package.

If a future design asserted build facts into the kernel (e.g. `build_env_ready(workspace)`), that would live in a different package; this corpus would need an extra Mangle surface doc.

---

## 7. Threat-oriented scenarios

| Scenario | Outcome |
|----------|---------|
| Agent compiles untrusted generated Go in thunderdome | CGO off; filtered env; still runs host `go` — residual trust in toolchain |
| Config supplies `CGO_CFLAGS` pointing at attacker headers | Trusted config store issue; build will honor it |
| Empty env machine (no HOME/LOCALAPPDATA/TEMP) | GOCACHE may be empty; compile fails loudly in caller |
| Operator expects secret `API_KEY` in go test env | Not present unless whitelisted or in Build.EnvVars |

---

## 8. Review checklist for changes

- [ ] Does the change expand essential vars carefully?  
- [ ] Are new logs redacting secrets where needed?  
- [ ] Still no full `os.Environ()` default?  
- [ ] Tests for hasEnvKey prefix behavior still pass?  
- [ ] Autopoiesis CGO=0 overlays still documented?  
