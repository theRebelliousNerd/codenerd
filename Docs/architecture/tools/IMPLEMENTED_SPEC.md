# tools — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — **code-grounded**  
> Mode: 1:1 with `internal/tools/`  
> Implementation: **25** non-test `.go`, **21** test files, **0** local `.mg`  
> External Mangle surface: `internal/mangle/intent_routing.mg` + `internal/core/defaults/schemas_tools.mg`  
> Package README: `internal/tools/README.md` (architecture version 2.0.0, JIT-driven)

---

## 1. Overview

`internal/tools` is the **modular tool registry and built-in tool library** for the JIT Clean Loop. Tools are standalone Go handlers with JSON-schema-ish argument contracts. Any agent can invoke them once ConfigFactory / Mangle routing has allowed the name.

### Architectural slogan for this package

```
Intent → ConfigFactory (AllowedTools[]) → Registry.Get / Execute → Tool.Execute(ctx, args)
```

Tools are **not** the executive. The Mangle kernel + VirtualStore + session safety gates decide *whether* a call may run. This package decides *how* a named tool runs and what it returns as a string payload for the LLM.

### Package layout

```
internal/tools/
├── types.go              # Tool, ToolSchema, ToolCategory, ToolResult
├── registry.go           # Thread-safe Registry + global singleton
├── errors.go             # Sentinel errors
├── README.md
├── core/                 # Filesystem: read/write/edit/delete/list + glob/grep
├── shell/                # run_command, bash, build, tests, git_*
├── codedom/              # Semantic-ish element listing + line edits + test impact
└── research/             # context7, web_*, browser_*, research_cache_*, helpers
```

### Dual-registry reality (critical)

There are **two** registration targets at boot:

| Registry | Owner | Consumers |
|----------|-------|-----------|
| `VirtualStore.modularTools` | `internal/core` | RouteAction / research handlers, VS APIs |
| `tools.Global()` process singleton | `internal/tools` | `session.Executor.executeToolCall` |

`VirtualStore.HydrateModularTools()` (`internal/core/virtual_store_tools.go`) registers **all four** subpackages into **both**. Session interactive coding uses `tools.Global()` first, then falls back to Ouroboros compiled binaries.

Separately, `core.ToolRegistry` (Ouroboros / compiled tools under `.nerd/tools/.compiled/`) is **not** this package — it is a sibling execution path in VirtualStore/session.

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Root types + schema | **Implemented** | `types.go` |
| Thread-safe Registry | **Implemented** | RWMutex, category index, arg validation |
| Global singleton | **Implemented** | `Global()`, package-level Register/Execute |
| core filesystem tools | **Implemented** | workspace path containment |
| core search (glob/grep) | **Implemented** | **partial** workspace guard on search paths |
| shell execution | **Implemented** | shellquote parse; Windows bash discovery |
| git tools | **Implemented** | diff/log/operation wrappers over run_command |
| codedom elements | **Implemented** | regex multi-language, not full AST |
| codedom line edits | **Implemented** | no workspace guard (path absolute trust) |
| test impact tools | **Implemented** | requires `RegisterTestImpactProvider` |
| research context7 | **Implemented** | llms.txt + common docs; optional API key |
| research web_search | **Implemented** | DuckDuckGo HTML scrape |
| research web_fetch | **Implemented** | HTML→markdown, size limits |
| browser tools | **Implemented** | via `internal/browser.SessionManager` |
| research cache tools | **Implemented** | in-memory TTL cache |
| GroundingHelper | **Implemented** | Gemini Google Search / URL Context |
| ThinkingHelper | **Implemented** | Gemini thinking metadata capture |
| Local `.mg` in package | **N/A** | routing lives in mangle corpus |
| Intent FilterByIntent | **Partial** | soft map; empty intent → All() |
| Review/Attack category tools | **Absent** | categories exist; no tools register them |
| Overall living package | **~85–90%** | production-wired, known gaps documented |

---

## 3. Source inventory

### 3.1 Counts

| Kind | Count (approx) |
|------|---------------:|
| Non-test `.go` | 25 |
| Test `.go` | 21 |
| Local `.mg` | 0 |
| Subpackages | 4 (`core`, `shell`, `codedom`, `research`) |

### 3.2 Largest non-test files

| Path | Lines (≈) | Role |
|------|----------:|------|
| `shell/execute.go` | 772 | Command/bash/build/test/git tool bodies |
| `core/file_ops.go` | 465 | read/write/edit/delete/list |
| `codedom/run_impacted_tests.go` | 433 | Impact analysis tools + provider DI |
| `core/search.go` | 362 | glob, grep, search_code |
| `research/browser.go` | 355 | browser_* tools over SessionManager |
| `registry.go` | 341 | Registry + FilterByIntent + global |
| `research/grounding.go` | 334 | Gemini grounding helper |
| `research/cache.go` | 303 | ResearchCache + cache tools |
| `research/context7.go` | 297 | context7_fetch / llms.txt |
| `codedom/lines.go` | 279 | edit/insert/delete lines |
| `research/web_fetch.go` | 263 | Fetch + HTML→md |
| `research/web_search.go` | 246 | DuckDuckGo search |
| `codedom/elements.go` | 239 | get_elements / get_element |
| `research/thinking.go` | 211 | ThinkingHelper |
| `types.go` | 134 | Core types |
| `core/workspace_guard.go` | 102 | Path containment |
| `errors.go` | ~27 | Sentinels |
| `*/register.go` | 29–44 each | RegisterAll hooks |

### 3.3 Complete built-in tool catalog

#### `core` — CategoryCode (unless noted)

| Name | Priority | Required args | Safety notes |
|------|---------:|---------------|--------------|
| `read_file` | 90 | `path` | Workspace root containment |
| `write_file` | 80 | `path`, `content` | Containment; create_dirs default true |
| `edit_file` | 85 | `path`, `old_text`, `new_text` | Containment; replace_all optional |
| `delete_file` | 50 | `path` | Containment; refuses directories |
| `list_files` | 85 | `path` | Containment; recursive skips out-of-tree |
| `glob` | 85 | `pattern` | **No** `resolveWorkspacePath` on base_path |
| `grep` | 85 | `pattern` | **No** workspace guard; skips `.` dirs/node_modules/vendor |
| `search_code` | 85 | (same as grep) | Alias of GrepTool with renamed metadata |

#### `shell`

| Name | Category | Priority | Required | Notes |
|------|----------|---------:|----------|-------|
| `run_command` | `/code` | 70 | `command` | shellquote.Split; timeout default 60s; output cap 50k |
| `bash` | `/code` | 70 | `script` | Windows: Git Bash / PATH; else falls back to run_command |
| `run_build` | `/code` | 75 | — | Auto-detect go/cargo/npm/make/gradle/mvn/cmake/python |
| `run_tests` | `/test` | 75 | — | Auto-detect + pattern append |
| `git_diff` | `/code` | 70 | — | Wraps `git diff` |
| `git_log` | `/code` | 70 | — | Wraps `git log` |
| `git_operation` | `/code` | 70 | `operation` | status/add/commit/checkout/branch/push/pull/fetch/stash/reset |

#### `codedom`

| Name | Category | Priority | Required | Notes |
|------|----------|---------:|----------|-------|
| `get_elements` | `/code` | 80 | `path` | Regex per language ext |
| `get_element` | `/code` | 80 | `path`, `name` | Filter extracted elements |
| `edit_lines` | `/code` | 80 | path + line range + content | Direct os.Read/WriteFile |
| `insert_lines` | `/code` | 80 | path, after_line, content | |
| `delete_lines` | `/code` | 75 | path, start_line, end_line | |
| `run_impacted_tests` | `/test` | 60 | — | Needs TestImpactProvider; may query `plan_edit` |
| `get_impacted_tests` | `/test` | 55 | — | Query only; optional coverage gaps |

#### `research` — CategoryResearch

| Name | Priority | Required | Notes |
|------|---------:|----------|-------|
| `context7_fetch` | 80 | `topic` | Optional `repo`, `max_docs`; config API key |
| `web_search` | 75 | `query` | DuckDuckGo HTML; max_results cap 30 |
| `web_fetch` | 70 | `url` | 2MB read limit; max_length default 50k |
| `browser_navigate` | 60 | `url` | Shared SessionManager |
| `browser_extract` | 55 | `session_id` | CSS selector default body |
| `browser_screenshot` | (mid) | `session_id` | base64 image payload |
| `browser_click` | (mid) | session + selector | |
| `browser_type` | (mid) | session + selector + text | |
| `browser_close` | (mid) | `session_id` | |
| `research_cache_get` | (high in Mangle prio) | `key` | miss → error |
| `research_cache_set` | 40 | `key`, `value` | optional source |
| `research_cache_clear` | 30 | — | |
| `research_cache_stats` | 30 | — | |

**Registered tool count after full hydrate:** ~35 tools (8 core + 7 shell + 7 codedom + 13 research).

---

## 4. Deep dive — Registry

### 4.1 Types (`types.go`)

- **`ToolCategory`**: `/research`, `/code`, `/test`, `/review`, `/attack`, `/general`.
- **`Tool`**: Name, Description, Category, Execute, Schema, Priority (default 50), RequiresContext.
- **`ToolSchema`**: Required []string + Properties map (type/description/default/enum/items).
- **`ExecuteFunc`**: `func(ctx context.Context, args map[string]any) (string, error)`.
- **`ToolResult`**: ToolName, Result, Error, DurationMs; `IsSuccess()`.

### 4.2 Registry behaviors (`registry.go`)

| Method | Behavior |
|--------|----------|
| `Register` | Validate; reject duplicates; default Priority 50; index by category; ToolsDebug log |
| `MustRegister` | Panic on failure (static init style) |
| `Get` / `Has` / `All` / `Names` / `Count` | RLock lookups; Names sorted |
| `GetByCategory` | Copy + stable sort by Priority desc, Name asc |
| `GetMultiple` | Skip missing silently |
| `Execute` / `ExecuteTool` | Nil ctx → Background; validateArgs; time duration; return ToolResult + err |
| `validateArgs` | Required keys; best-effort JSON-schema types; extra keys allowed |
| `FilterByIntent` | Maps intent verb → category; unknown/empty → **All()** |

### 4.3 Intent → category map (`intentToCategory`)

| Intent verbs | Category |
|--------------|----------|
| `/research`, `/explore`, `/learn`, `/document` | `/research` |
| `/fix`, `/implement`, `/refactor`, `/create`, `/edit` | `/code` |
| `/test`, `/cover`, `/verify` | `/test` |
| `/review`, `/audit`, `/check` | `/review` |
| `/attack`, `/break`, `/nemesis` | `/attack` |
| `/general` | `/general` |
| other / empty | fall back to All() |

**Honest note:** Runtime allowlisting in production is **primarily** ConfigFactory `AllowedTools` + Mangle `modular_tool_allowed` + session safety, not `FilterByIntent` alone. FilterByIntent is a soft selector helper.

### 4.4 Errors (`errors.go`)

`ErrToolNotFound`, `ErrToolNameEmpty`, `ErrToolExecuteNil`, `ErrToolAlreadyRegistered`, `ErrMissingRequiredArg`, `ErrInvalidArgType`, `ErrToolNil`.

---

## 5. Deep dive — core (filesystem)

### 5.1 Workspace containment (`workspace_guard.go`)

- Root: `CODENERD_WORKSPACE_ROOT` env if set, else `os.Getwd()`.
- `resolveWorkspacePath`: Abs + EvalSymlinks; non-existent write paths resolve nearest existing parent; reject `..` escapes with `ErrPathOutsideWorkspace`.
- TODO in source: thread workspace root via registry context instead of process env.

**Applied to:** read/write/edit/delete/list in `file_ops.go`.  
**Not applied to:** glob/grep/search_code in `search.go` (path/base_path used raw).

### 5.2 File ops behaviors

- **read_file**: optional start_line/end_line with float64 coercion (`coerceInt`).
- **write_file**: content must be string (rejects non-string); MkdirAll when create_dirs.
- **edit_file**: first-only or replace_all; fails if old_text missing.
- **delete_file**: Stat; refuse IsDir.
- **list_files**: recursive Walk skips hidden dirs by default; containment re-check on each path.

Logging: `logging.VirtualStore` / `VirtualStoreDebug` (not CategoryTools) for file ops.

---

## 6. Deep dive — shell

### 6.1 run_command

1. Parse with `github.com/kballard/go-shellquote` (injection-resistant vs shell -c).
2. `context.WithTimeout` before `exec.CommandContext` (bound deadline).
3. Optional working_dir + env map (string values only).
4. Merge stdout + `--- stderr ---` + stderr.
5. Truncate output > 50_000 chars.
6. Timeout → explicit error; non-zero exit returns error with output body.

Test hooks: `execCommandContext`, `execLookPath` package vars.

### 6.2 bash

- Non-Windows: `bash` with script on stdin.
- Windows: search Git Bash paths + PATH; else degrade to `executeRunCommand` on the script string.

### 6.3 Auto-detect build/test

File markers → commands (go.mod → `go build ./...` / `go test ./...`, etc.). Pattern injection for go/pytest/npm/cargo.

### 6.4 Git tools

Compose argv → string → `executeRunCommand`. `git_operation` whitelists operations; commit requires message; unsupported ops error.

**Gap:** `working_dir` for shell tools is **not** forced through workspace guard — cwd can be any path the OS allows.

---

## 7. Deep dive — codedom

### 7.1 Element extraction

Regex maps for go/py/js-ts/java-kt-scala/rs/c-cpp + generic fallback. `EndLine` is currently start line only (comment in code: needs block tracking). Explicit note: full AST is VirtualStore/world territory; this is lightweight.

### 7.2 Line tools

Read entire file, slice lines, rewrite. Accept int and float64 for line numbers. **No workspace containment** — absolute paths work if process can open them.

### 7.3 Test impact DI

Interfaces break import cycles with `world` and `core`:

- `TestDependencyAnalyzer` — Build, GetImpactedTests, GetImpactedTestPackages, GetCoverageGaps
- `KernelQuerier` — Query facts
- `TestImpactProvider` — GetKernel, GetProjectRoot, NewTestDependencyAnalyzer
- `RegisterTestImpactProvider` sets package global

Without provider: `run_impacted_tests` / `get_impacted_tests` error `"test impact provider not initialized"`.

Empty `edited_refs` → query kernel `plan_edit` facts. Dry-run mode lists tests without `go test`.

---

## 8. Deep dive — research

### 8.1 context7_fetch

- Optional `config.AutoDetectContext7APIKey()`.
- Infer GitHub repo from known topic map (react, rod, mangle, bubbletea, …).
- Fetch llms.txt / common docs; combine markdown; graceful empty message.

### 8.2 web_search / web_fetch

- Search: DuckDuckGo HTML endpoint, browser-like UA, 30s timeout, 1MB body, HTML parse.
- Fetch: 60s timeout, 2MB body, plain/md pass-through, HTML→markdown with link option, length cap.

### 8.3 browser_* 

Shared `browser.SessionManager` (once): Start, CreateSession/Navigate, Page for extract/click/type/screenshot. Logs via `logging.Browser` / `BrowserDebug`.

### 8.4 ResearchCache

Default: 1000 entries, 30m TTL, oldest eviction. Tools expose get/set/clear/stats. Mangle prioritizes `research_cache_get` at priority 90.

### 8.5 GroundingHelper / ThinkingHelper

**Not registry tools.** Library helpers for init/campaigns/autopoiesis when LLM client implements:

- `types.GroundingController` / `GroundingProvider` (Google Search, URL Context ≤20 URLs)
- `types.ThinkingProvider` / `ThoughtSignatureProvider` (thought summary, signature, token counts)

Used from `internal/init`, `internal/campaign`, `internal/autopoiesis` imports of `tools/research`.

---

## 9. Integration map (fact-flow)

```
user input
  → perception → user_intent(…)
  → ConfigFactory / JIT → EffectiveAgentRuntimeConfig.AllowedTools
  → prompt JIT exposes tool schemas to model
  → LLM tool_calls
  → session.Executor.runToolLoop
       ├─ isToolAllowed(AllowedTools)   # empty list = allow all
       ├─ checkSafety → pending_action → query permitted  # if EnableSafetyGate
       ├─ InteractiveExecutiveGate.PreflightDestructiveToolCall
       ├─ tools.Global().Execute  OR  Ouroboros ExecuteRegisteredTool
       └─ ValidateInteractiveToolResult (post)
  → tool result string → ToolResultsProvider multi-turn (or piggyback single pass)
  → articulation / next turn
```

### Hydration path

```
system boot (internal/system/factory.go)
  → virtualStore.HydrateModularTools()
       → core.RegisterAll + shell + codedom + research
       → both VS.modularTools and tools.Global()
```

### Mangle modular routing

- Decls: `schemas_tools.mg` — `modular_tool_allowed/2`, `modular_tool_priority/2`, `tool_execution/3`, …
- Rules: `intent_routing.mg` Section 4.5 — per-tool allow by intent category (read tools for all intents; write/shell for /code|/test; research tools for /research|/learn|/document|/verify).

Session `isToolAllowed` currently uses **config list**, not a live kernel query of `modular_tool_allowed`. ConfigFactory is expected to materialize AllowedTools from policy/JIT atoms; audit wiring if names diverge (e.g. git_* tools absent from intent_routing snippet).

### Consumers (import evidence)

| Package | Use |
|---------|-----|
| `internal/core` | modularTools field, HydrateModularTools, research routes |
| `internal/session` | tools.Global() execute path |
| `internal/system` | boot hydrate |
| `internal/init` | research Grounding/Context7 knowledge |
| `internal/campaign` | research helpers in decomposer/replan |
| `internal/autopoiesis` | research grounding |
| `internal/world` | codedom interfaces for test dependency |
| `tests/e2e/*` | AllowedTools / modular registry boundaries |

---

## 10. Concurrency & process state

| Global | Package | Risk |
|--------|---------|------|
| `globalRegistry` | tools | Process-wide; dual-hydrate idempotent via Has() skip |
| `globalTestProvider` | codedom | Must register once at boot |
| `defaultCache` | research | In-memory only; not multi-process |
| `browserMgr` | research | Shared SessionManager; Start once |
| Registry.mu | tools | RWMutex correct for concurrent Execute |

---

## 11. Observability surface

| Logger | Where |
|--------|-------|
| `logging.ToolsDebug` | Register + Execute in registry.go |
| `logging.VirtualStore` / Debug | core file ops, shell tools |
| `logging.Researcher` / Debug / Warn | research web/context7/cache/grounding |
| `logging.Browser` / Debug | browser tools |
| Session category | executeToolCall success/fail (outside package) |

No Prometheus metrics in-package. DurationMs on ToolResult is the local timing hook.

---

## 12. Testing inventory (package-local)

| Area | Representative tests |
|------|----------------------|
| Registry | register/dup/validate/execute/types/filter/global/boundary nil ctx |
| core | RegisterAll; workspace resolve; glob/grep success & edges |
| shell | mocked exec; coerceInt; detect build/test; git defs; integration suite |
| codedom | lines edit/insert/delete; elements; impact parse helpers |
| research | context7/fetch/search coverage + tool tests |

Command:

```powershell
go test ./internal/tools/...
```

E2E coverage of allowlists and safety lives under `tests/e2e/` (tool_safety_fallback, SessionExecutor_VirtualStore_Kernel, piggyback, session_clean_loop).

---

## 13. Gaps pointer (see also 03-GAP-ANALYSIS)

1. **Search path containment** — glob/grep not using workspace_guard.
2. **codedom path containment** — same.
3. **shell working_dir** — unconstrained.
4. **git_* / research_cache_clear/stats** missing from `modular_tool_allowed` rules (partial Mangle catalog).
5. **FilterByIntent vs Mangle** — two overlapping models; FilterByIntent falls open.
6. **CategoryReview / CategoryAttack** — enums without tools.
7. **doc.go drift** — codedom doc lists open_file/edit_element/etc. not registered.
8. **isToolAllowed empty = allow all** — intentional fallback; dangerous if ConfigFactory fails open.
9. **Piggyback tool loop** — single pass (session limitation, affects tool UX).
10. **Workspace root via env** — process-global TODO in workspace_guard.go.

---

## 14. Non-goals of this corpus revision

- Implementing fixes or wiring changes
- Documenting Ouroboros compiler internals (belongs to core/autopoiesis)
- Full HTML parser algorithm for DuckDuckGo edge cases
- Vectryx / external product vocabulary

---

## 15. Verify commands

```powershell
go test ./internal/tools/...
go test ./internal/session/ -count=1 -run Tool
# full tree when feasible:
go test ./...
```

Build with sqlite-vec flags from root `Agents.md` only if linking the full binary for e2e.

---

## 16. Document map

| Doc | Role |
|-----|------|
| [README.md](README.md) | Index + verify |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star scores |
| [01-VISION.md](01-VISION.md) | Target architecture |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Living inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components + flows |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported surface |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Imports |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot + session |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logs |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failures |
| [TODO.md](TODO.md) | Backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Open design Qs |
| [_progress.md](_progress.md) | Rebuild log |
