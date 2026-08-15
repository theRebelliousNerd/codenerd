# 11 — Observability: `internal/build`

> Last verified: **2026-08-15**

---

## 1. Logging category

| Item | Value |
|------|-------|
| Category constant | `logging.CategoryBuild` |
| String | `"build"` |
| Defined in | `internal/logging/logger.go` |
| Convenience API | `Build`, `BuildDebug`, `BuildWarn`, `BuildError` in `logger_convenience.go` |

`internal/build` uses **`logging.BuildDebug` only**.

---

## 2. Emit sites (`env.go`)

| Message pattern | When |
|-----------------|------|
| `Building environment for workspace: %s` | Entry to `GetBuildEnv` |
| `Added whitelisted env: %s` | Each whitelist key admitted (key only) |
| `Added build config env: %s=%s` | Each Build.EnvVars pair (**includes value**) |
| `Auto-detected CGO_CFLAGS: %s` | Fallback detect path |
| `Final build environment has %d vars` | Exit `GetBuildEnv` |
| `Derived GOCACHE: %s` | When GOCACHE synthesized |
| `Loaded BuildConfig from user config` | userCfg.Build present |
| `Detected sqlite_headers at: %s` | Headers directory found |

---

## 3. What is not observed

| Missing | Impact |
|---------|--------|
| Metrics (counters/histograms) | No “build env constructions per minute” |
| Structured fields / slog attrs | String-format only |
| Glass-box / transparency events | Ops cannot see env in TUI glass box |
| Warning when GOCACHE could not be derived | Silent empty |
| Warning when headers missing but CGO likely needed | Silent |
| Redacted full env dump helper | Hard to debug subtle overrides |
| Caller-side success/fail of `go` | Belongs to autopoiesis logs (`AutopoiesisDebug`) |

---

## 4. Operator playbook

### Enable build debug

Set logging so category `build` is visible at debug (depends on global logging config / `.nerd` logging settings). Look under `.nerd/logs/` when file logging is enabled.

### Correlate with autopoiesis

Compile failures appear in autopoiesis logs with compiler stderr. Build category shows **whether CGO_CFLAGS / GOCACHE were prepared**, not whether compile succeeded.

### Manual verification without logs

```powershell
# In a small Go main or delve session conceptually:
# env := build.GetBuildEnv(cfg, `C:\CodeProjects\codeNERD`)
# inspect for CGO_CFLAGS and GOFLAGS=-tags=sqlite_vec when headers exist
```

Or unit-test pattern: temp dir + `sqlite_headers` + assert prefixes.

---

## 5. Privacy note

`Added build config env: %s=%s` logs **values**. Do not put secrets in `build.env_vars` if debug logs are shipped off-box. Prefer non-secret flags (`CGO_CFLAGS`, `GOFLAGS`) in that map.

Whitelist path logs **keys only** — safer.

---

## 6. Future observability (optional)

1. `BuildWarn` when `deriveGOCACHE` returns empty.  
2. Summary helper `SummarizeEnv(env []string) string` listing keys only.  
3. Glass-box event `build_env_prepared{var_count, has_cgo_cflags, has_gocache}` for compile actuators.  

None of these exist today.


## Redaction and summarization (2026-08-15)

`GetBuildEnv` used to log `Added build config env: %s=%s` at debug. `build.env_vars`
is a free-form operator map that routinely carries API keys, registry tokens and
proxy credentials, so that line wrote secrets to disk.

Two changes:

- **`redactEnvValue(key, value)`** — values are printed only for an allowlist of
  toolchain keys (`CGO_*`, `GO*`, `CC`, `CXX`, `PATH`, `HOME`, temp dirs, `CI`, …).
  Everything else prints `<redacted>` / `<empty>`. A key containing `TOKEN`,
  `SECRET`, `PASSWORD`, `_KEY`, `AUTH`, `CREDENTIAL`, `SESSION` or `COOKIE` is
  redacted even if it would otherwise be allowlisted.
- **`SummarizeEnv(env)`** — sorted, keys-only rendering, used for the final
  "environment has N vars" line and available to callers for env diffs.

Redaction is a **logging** concern only: the real values still reach the
subprocess (`TestGetBuildEnv_WhenSecretInConfigEnvVars_ShouldStillBePassedToSubprocess`).

`logging.BuildWarn` is now used (previously debug-only): when every GOCACHE
fallback is empty, the package says so by name instead of letting the subprocess
die with an opaque "GOCACHE is not defined".
