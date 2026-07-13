# 00 — Alignment & Vision Review: `internal/build`

> Last verified: **2026-07-13**  
> Evidence base: `internal/build/env.go`, callers in `internal/autopoiesis`, related `internal/config` / `internal/logging`

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
| **Separation of concerns** (env factory vs execution policy) | **5** | Pure env construction; no shell policy, no kernel rules |
| **Single source of truth (intent)** | **4** | Clear API and package comment mandate |
| **Single source of truth (adoption)** | **2** | Only autopoiesis imports; shell/tactile/manual bypass |
| **Honest config integration** | **3** | Reads `UserConfig.Build` + whitelist; prod callers pass `nil` |
| **codeNERD CGO / sqlite-vec honesty** | **4** | Auto-detects `sqlite_headers`, default `-tags=sqlite_vec` |
| **Constitutional safety (`permitted`)** | **N/A→3** | Not a policy layer; does filter env (good); not default-deny executive |
| **LLM / JIT prompt discipline** | **5** | No LLM surface; correctly non-prompt |
| **Wiring audit before “unused” claims** | **4** | Small API; callers greppable; `GoFlags`/`CGOPackages` partially dead |
| **Testability / determinism** | **5** | Heavy unit coverage; temp dirs; env isolation via `t.Setenv` |
| **Observability** | **3** | `BuildDebug` only; no metrics / structured audit of final env |
| **North-star “logic owns executive”** | **4** | Stays out of executive; actuators call it — correct layering |
| **Completeness of specialized APIs** | **2** | `GetBuildEnvForTest` empty specialization; GoFlags unused |

**Composite (subjective):** ~**3.5 / 5** — excellent narrow implementation, incomplete platform mandate.

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

1. Package comment oversells “all components” while adoption is two call sites, both with `nil` config.  
2. Dead / half-live fields: `GoFlags`, runtime use of `CGOPackages`, test-env specialization.  
3. Duplicate `BuildConfig` in `build` and `config` packages.  
4. `GetBuildEnv` can emit duplicate keys across merge stages.  
5. Autopoiesis passes arena/tmp as `workspaceRoot` — auto-detect of monorepo headers does not apply there.

---

## Alignment verdict

**Ship-worthy as a library.** **Not yet** the repo-wide unified build-environment gate described in its own package and config comments. Vision documents should treat full adoption as roadmap, not as current fact.

See [01-VISION.md](01-VISION.md) and [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).
