# 04 — Architectural Principles: `internal/build`

> Binding design principles for `internal/build`.  
> Violations should be called out in review.  
> Last verified: **2026-07-13**

---

## P1 — Env factory only; never a launcher

`internal/build` returns `[]string` environments. It must not:

- Call `exec.Command`
- Own process lifecycle, timeouts, or stdout capture
- Implement sandbox policy

Launchers live in autopoiesis, tactile, tools, campaign, etc.

## P2 — Prefer filtered env over `os.Environ()`

Default to a **minimal essential** Go toolchain environment, then admit extras via:

1. Explicit build config `EnvVars`
2. Execution whitelist `AllowedEnvVars`

Do not regress to dumping the full parent environment into sandboxed compiles.

## P3 — Workspace-aware CGO honesty

If the workspace contains `sqlite_headers/`, default behavior should make CGO builds viable without tribal shell knowledge. Align with root `Agents.md` / `Claude.md` build recipe.

When sandbox policy requires no CGO, callers **overlay** with `MergeEnv(..., "CGO_ENABLED=0")` rather than forking a second factory.

## P4 — Config is optional but first-class

`userCfg` may be `nil` (tests, early boot, isolated tools). When non-nil, `Build` and `Execution` blocks must be honored. New call sites with access to loaded config **must not** habitually pass `nil` without justification.

## P5 — Separate detection root from build directory

Header / tag detection uses a **workspace root**. Compilation may use a temporary module directory as `cmd.Dir`. Do not conflate the two parameters.

## P6 — Overlay, don’t fork

Cross-cutting constraints (`CGO_ENABLED`, temporary `GOFLAGS`) apply through `MergeEnv` / `setEnvKey` on top of `GetBuildEnv*`, not through copy-pasted alternate constructors.

## P7 — Specialization APIs must earn their name

`GetBuildEnvForTest` and `GetBuildEnvForCompile` exist to encode **real** differences. Empty wrappers that only rename the same function are debt. Either differentiate or collapse.

## P8 — No silent dead config fields

If `GoFlags` or `CGOPackages` cannot affect behavior, do not present them as operational controls in user-facing config docs without noting “metadata only.” Prefer implementing consumers or deleting fields.

## P9 — Stay off the executive path

Do not introduce Mangle predicates, VirtualStore action names, or prompt atoms into this package. Permission and planning remain kernel/executive concerns.

## P10 — Observability at debug, secrets careful

Log **which keys** and **counts** via `logging.BuildDebug`. Avoid logging full secret-bearing values at info level. Whitelisted vars may be sensitive; treat debug as operator-only.

## P11 — Deterministic helpers for tests

Key helpers (`hasEnvKey`, `setEnvKey`, `deriveGOCACHE`, `detectCGOFlags`) must remain pure enough to unit-test with `t.Setenv` and temp dirs. Avoid hidden global caches.

## P12 — Document adoption, don’t invent it

When claiming “all components use GetBuildEnv,” verify with import graph. Prefer updating documentation over aspirational package comments.
