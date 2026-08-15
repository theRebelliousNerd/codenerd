# 00 — Alignment & Vision Review: `internal/build`

> Last verified: **2026-08-15**  
> Evidence base: `internal/build/env.go`, callers in `internal/autopoiesis` / `internal/session` / `internal/core`, related `internal/config` / `internal/logging`

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully aligned; living code matches north star for this slice |
| 4 | Strong alignment; minor gaps |
| 3 | Partial; intent clear, adoption or completeness incomplete |
| 2 | Weak; design aspiration mostly unmet |
| 1 | Misaligned or absent |

---

## Dimension scores

| Dimension | Score | Evidence |
|-----------|------:|----------|
| Dimension | 2026-07-13 | 2026-08-15 | Evidence for the current score |
|-----------|-----------:|-----------:|--------------------------------|
| **Separation of concerns** (env factory vs execution policy) | 5 | **5** | Pure env construction; no shell policy, no kernel rules |
| **Single source of truth (intent)** | 4 | **5** | Package comment now names the real importers and is held there by `TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented` |
| **Single source of truth (adoption)** | 2 | **4** | autopoiesis + session + core import it; every remaining `go` spawn carries a reasoned exemption enforced by `TestGoInvocations_…_ShouldUseBuildEnvOrBeExempt` |
| **Honest config integration** | 3 | **3** | Reads `UserConfig.Build` + whitelist; only `session.verifyBuild` passes a real config — the rest still pass `nil` |
| **codeNERD CGO / sqlite-vec honesty** | 4 | **5** | Auto-detects `sqlite_headers`, default `-tags=sqlite_vec`, and `DetectionRootFor` now finds monorepo headers from a nested module dir |
| **Constitutional safety (`permitted`)** | N/A→3 | **N/A→3** | Not a policy layer; does filter env (good); not default-deny executive |
| **LLM / JIT prompt discipline** | 5 | **5** | No LLM surface; correctly non-prompt |
| **Wiring audit before “unused” claims** | 4 | **5** | The audit is a test, not a grep someone remembers to run |
| **Testability / determinism** | 5 | **5** | Env slice is now order-deterministic (sorted config keys); integration tests exercise the real toolchain |
| **Observability** | 3 | **4** | `SummarizeEnv` keys-only logging, secret redaction, `BuildWarn` on underivable GOCACHE; still no metrics |
| **North-star “logic owns executive”** | 4 | **4** | Stays out of executive; actuators call it — correct layering |
| **Completeness of specialized APIs** | 2 | **4** | `GetBuildEnvForTest` has real specialization; `GoFlags` consumed by `AppendGoFlags`; `CGOPackages` still descriptive only |

**Composite (subjective):** ~**4.3 / 5** (was ~3.5) — the mandate is now enforced
rather than asserted. Held back from higher by production callers still passing
`nil` user config and by `CGOPackages` remaining documentation-only.

---

## North star injection

| North-star clause | How build participates |
|-------------------|------------------------|
| LLM = creative center; Mangle = executive | Build env is **neither**; it is effect plumbing under actuators |
| `permitted(...)` default deny | Does not evaluate permission; callers must already be authorized to compile |
| JIT prompt atoms | No prompt text here — correctly |
| Wiring audit before deletion | Do not remove env helpers because “only autopoiesis uses them” without checking compile paths |

---

## Strengths (keep)

1. **Focused responsibility** — one file, one job: `[]string` env for Go tools.  
2. **Platform-aware GOCACHE** — Windows `LOCALAPPDATA` first, Unix `HOME` later.  
3. **sqlite_headers convention** aligned with root build docs.  
4. **MergeEnv** lets callers force sandbox constraints (`CGO_ENABLED=0`) without forking the base factory.  
5. **Test density** relative to package size is high.

---

## Weaknesses (fix intentionally)

1. ~~Package comment oversells “all components”~~ — **fixed**; the comment lists the real importers and a test enforces it.  
2. ~~Dead / half-live fields: `GoFlags`, test-env specialization~~ — **fixed** (`AppendGoFlags`, real `GetBuildEnvForTest`). `CGOPackages` is still descriptive only, by design.  
3. ~~Duplicate `BuildConfig` in `build` and `config`~~ — **fixed**; `build.BuildConfig` is an alias for `config.BuildConfig`.  
4. ~~`GetBuildEnv` can emit duplicate keys across merge stages~~ — **fixed**; every stage uses `setEnvKey`.  
5. Autopoiesis passes arena/tmp as `workspaceRoot`. `DetectionRootFor` /
   `GetBuildEnvForModule` now make the fix a one-line call-site change, but the
   autopoiesis call sites still pass the temp dir and `nil` config.  
6. Production callers other than `session.verifyBuild` still pass `nil`
   `*config.UserConfig`, so operator `build.env_vars` stays latent for them.

---

## Alignment verdict

**Ship-worthy as a library, and now genuinely the repo's build-environment
gate** — not because the comment says so, but because
`TestGoInvocations_WhenSpawningGo_ShouldUseBuildEnvOrBeExempt` fails when a new
`go` invocation appears without either routing through this package or carrying
a written exemption. What remains is call-site quality (`nil` configs, temp-dir
detection roots), not coverage.

See [01-VISION.md](01-VISION.md) and [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).
