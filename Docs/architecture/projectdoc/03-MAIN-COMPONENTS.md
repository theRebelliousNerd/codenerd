# projectdoc — Main Components

> **Package:** `internal/projectdoc` — nerd.md project-instruction subsystem
> **Status:** `VERIFIED CURRENT` — 2026-05-13, HEAD (re-synthesized 2026-08-07)
> **Sources:** `internal/projectdoc/nerdmd.go`, `internal/projectdoc/facts.go`, `internal/projectdoc/nerdmd_test.go`, `internal/core/defaults/schemas_projectdoc.mg`, `internal/core/defaults/schemas_project.mg`, `internal/core/defaults/policy/projectdoc.mg`, `internal/session/executor_tools.go:406-445`
> **Corpus role:** This document completes the main-components slice of `Docs/architecture/projectdoc/` (companion to `02-CURRENT-STATE.md` and `IMPLEMENTED_SPEC.md`). Every behavioral claim cites `path#line` or `test#Name`. Claims beyond re-read files marked `ASSUMPTION`.

---

## 1. Thesis and Split

`nerd.md` splits along codeNERD's thesis (`internal/projectdoc/nerdmd.go:1-26`):

- **YAML frontmatter** (`---` fenced, strict `nerd/v1`) → kernel facts → policy-enforced. `forbid.match` becomes `project_forbidden_path/2` and is denied **before** tool execution.
- **Markdown body** (after second `---`, `TrimSpace`'d) → single project prompt atom → advisory only, never projected as facts (`internal/projectdoc/facts.go:47-52`).

File is **optional**: absence is `(nil, nil)` not an error (`internal/projectdoc/nerdmd.go:138-160`). Presence with any error is never degraded.

```mermaid
flowchart LR
  MD[nerd.md] --> FM[YAML frontmatter\nstrict nerd/v1]
  MD --> BODY[Markdown body\nTrimSpace verbatim]
  FM --> FACTS[Facts()\nproject_*]
  FACTS --> KERNEL[Mangle kernel overlay]
  KERNEL --> POLICY[policy/projectdoc.mg\nderives has_project_doc etc.]
  KERNEL --> GATE[executor_tools.go:406-445\nprojectForbidsWrite]
  BODY --> PROMPT[PromptSection()\nJIT prompt atom]
  GATE -. denies .-> TOOL[write-mutation tool]
```

---

## 2. Inventory

### 2.1 Package files (`internal/projectdoc/` — 3 Go files, ~900 lines, 0 `.mg` in-package)

| File | Approx Lines | Role | Evidence |
|---|---|---|---|
| `internal/projectdoc/nerdmd.go` | ~310 | Discovery (`Find`), loading (`Load`), parsing (`Parse`, `splitFrontmatter`), validation (`Spec.validate`), `ForbidsPath` helper, constants | `nerdmd.go:36 FileName`, `43 SchemaVersion`, `131-140 Find`, `145-160 Load`, `163-181 Parse`, `182-220 splitFrontmatter`, `232-272 validate`, `281-293 ForbidsPath` |
| `internal/projectdoc/facts.go` | ~250 | Predicate constants, `Facts()` projection, `CommandCount()`, `normalizeAtom()`, `PromptSection()` | `facts.go:10-38` constants, `54-131 Facts()`, `133-156 normalizeAtom`, `158-248 PromptSection` |
| `internal/projectdoc/nerdmd_test.go` | ~350 | 15+ tests: strictness, schema pin, forbid invariants, path normalisation, fact→atom, Find/Load | `TestParse_RequiresFrontmatter`, `TestParse_UnknownKeyIsAHardError`, `TestParse_SchemaVersionIsPinned`, `TestFind_PrefersWorkspaceRootOverNerdDir`, `TestFacts`, `TestPromptSection_StatesThatProtectionIsEnforced` |

No `.mg` inside package — Mangle surfaces live in `internal/core/defaults/` intentionally. Ownership split prevents parser package from depending on kernel wiring.

### 2.2 External Mangle surfaces

| File | Role | Predicates Declared |
|---|---|---|
| `internal/core/defaults/schemas_projectdoc.mg` | EDB/IDB declarations for projectdoc | `project_doc/2:18`, `project_name/1:21`, `project_command/2:37`, `project_command_env/2:42`, `project_forbidden_path/2:49`, `project_requirement/1:53`, `project_convention/2:56`, `has_project_doc/0:63`, `project_write_protected/0:68`, `project_has_command/1:71` |
| `internal/core/defaults/schemas_project.mg` | Decl for `project_language/1` shared with `nerd init` scan | `project_language/1` (`/name` bound) — not redeclared in `schemas_projectdoc.mg` to avoid duplicate Decl panic (`schemas_projectdoc.mg:23-31`) |
| `internal/core/defaults/policy/projectdoc.mg` | Derived predicates | `has_project_doc :- project_doc` `:7`, `project_has_command` `:13`, dormant `project_write_denied`/`coder_block_write` bridge `:32-34` — not yet consumed by executor |
| `internal/session/executor_tools.go:406-445` | **Live write-protection gate** `projectForbidsWrite` | Queries `project_forbidden_path/2` facts; denial text `blocked by nerd.md: <path> is write-protected (<reason>)` `:529` |

---

## 3. Component 1 — Discovery and Loading

### 3.1 `Find(workspace string) string` — `nerdmd.go:131-140`

Ordered first-hit search:

1. `<workspace>/nerd.md`
2. `<workspace>/.nerd/nerd.md`
3. `""` when absent — optional file, not an error.

Implementation: `os.Stat(candidate); err==nil && !info.IsDir()` per candidate — directory masquerading as file is ignored. Proven by `TestFind_PrefersWorkspaceRootOverNerdDir` (root wins when both exist; `.nerd/` is fallback).

```go
func Find(workspace string) string // nerdmd.go:131
```

Caller contract: empty return means proceed with `nil` document. No error, no log, no side effect.

### 3.2 `Load(workspace string) (*Document, error)` — `nerdmd.go:145-160`

```go
func Load(workspace string) (*Document, error)
```

Steps:

- `Find` → `""` ⇒ `(nil,nil)` — fast path.
- On hit: `os.ReadFile(path)` → `Parse(data)`; errors wrapped as `read <path>: …` or `<path>: …` so boot can surface which file failed (`147-151`).
- On success: `doc.Path = filepath.ToSlash(filepath.Rel(workspace, path))` with fallback to absolute slash-normalised on `Rel` error (`153-158`).
- Never degrades invalid/unreadable to "no directive" — caller must handle error.

Tests: `TestLoad_AbsentFileIsNotAnError` (`nil,nil`), `TestLoad_MalformedFileIsAnError` (`nerd/v9` rejected, wrapped with path).

### 3.3 `Document` shape — `nerdmd.go:48-56`

```go
type Document struct {
    Path string // slash-normalised relative or absolute
    Spec Spec   // strict frontmatter
    Body string // verbatim Markdown after frontmatter, TrimSpace'd, advisory
}
```

`Body` stored trimmed but otherwise verbatim; never fact-projected. `Path` is set only by `Load`, not by `Parse` (Parse has no workspace context).

---

## 4. Component 2 — Parsing (`Parse` + `splitFrontmatter`)

### 4.1 `Parse(data []byte) (*Document, error)` — `nerdmd.go:163-181`

1. `splitFrontmatter(data)` → `(front, body, err)` — fence extraction.
2. Strict YAML: `yaml.NewDecoder(bytes.NewReader(front))` with `KnownFields(true)` — unknown keys are **hard error** (`168-175`). Rationale at `170-172`: ignored directive worse than no directive.
3. `spec.validate()` (`177`) — semantic checks after syntactic success.
4. Return `&Document{Spec: spec, Body: strings.TrimSpace(body)}` (`181`).

Unknown key error shape: `invalid frontmatter: yaml: unmarshal errors: line N: field <key> not found in type projectdoc.Spec` — proven `TestParse_UnknownKeyIsAHardError`.

### 4.2 `splitFrontmatter` — `nerdmd.go:182-220`

| Concern | Detail | Line |
|---|---|---|
| Scanner buffer | `make([]byte,0,64*1024)` grown to `4*1024*1024` | `188-189` |
| Empty file | `file is empty; expected a "---" frontmatter block` | `196` |
| First line | Must be `---` trimmed else `first line must be "---" to open the frontmatter block (got …); nerd.md requires machine-readable frontmatter, unlike CLAUDE.md` | `199-203` |
| Closing fence | Second `---` trimmed closes; missing ⇒ `frontmatter block opened with "---" was never closed` | `214` |
| Body | Everything after closing fence joined with `"\n"`; `scanner.Err()` as `read: …` | `222-224` |
| Truncation safety | 4 MiB prevents body tables/commands truncating at default 64 KiB and misreporting as frontmatter error | `188-189` comment |

Validated by `TestParse_RequiresFrontmatter` (no frontmatter, unterminated, empty all rejected).

---

## 5. Component 3 — `nerd/v1` Frontmatter Schema (`nerdmd.go:61-127`)

All tags `yaml:"…,omitempty"` except `schema`. Adding any field is a schema change that must bump `SchemaVersion` (`60-62`).

| YAML key | Go field | Type | Required | Validation | Semantics |
|---|---|---|---|---|---|
| `schema` | `Schema` | `string` | **Yes** | `TrimSpace==""` ⇒ `frontmatter is missing the required "schema"` (`238`); `!= "nerd/v1"` ⇒ `unsupported schema` (`240-245`) | Pinned. Bump `SchemaVersion` for any new field |
| `project` | `Project` | `string` | No | trimmed empty ignored | Human name → `project_name/1` + prompt |
| `language` | `Language` | `string` | No | `normalizeAtom` → `""` ignored | Seeds JIT `/lang` atom (`facts.go:63-68`) |
| `commands` | `Commands` | `Commands` | No | — | See §5.1 |
| `forbid` | `Forbid` | `[]ForbidRule` | No | per-entry `match`+`reason` required (`247-255`) | **ENFORCED** → `project_forbidden_path/2` §5.2 |
| `require` | `Require` | `[]string` | No | `TrimSpace==""` ⇒ `require[i] is empty` (`265-268`) | → `project_requirement/1`, no Go gate yet |
| `conventions` | `Conventions` | `[]Convention` | No | `id`/`rule` required (`258-263`) | → `project_convention/2`, no Go gate yet |

Unknown key anywhere → hard error via `KnownFields(true)`.

Constants: `FileName="nerd.md"` (`36`), `SchemaVersion="nerd/v1"` (`43`) — pinned not range-checked so newer documents fail loudly rather than half-understood (`38-43`).

### 5.1 `Commands` — `nerdmd.go:95-106`

```go
type Commands struct {
    Build string            `yaml:"build,omitempty"`
    Test  string            `yaml:"test,omitempty"`
    Lint  string            `yaml:"lint,omitempty"`
    Run   string            `yaml:"run,omitempty"`
    Env   map[string]string `yaml:"env,omitempty"`
}
```

Projection (`facts.go:70-93`): non-empty trimmed `Build/Test/Lint/Run` → `project_command("/build" etc., Command)` with `Kind` as `types.MangleAtom`; `Env` entries with empty trimmed name skipped → `project_command_env/2` per entry.

Comment at `75-77`: without these the agent guesses, and a guess that compiles on the maintainer's machine is the most expensive kind of wrong. `Env` exists because `CGO_CFLAGS`-style prerequisites are invisible in the command string (`100-103`).

`CommandCount() int` counts non-empty `Build/Test/Lint/Run` — used for prompt gating and tests.

### 5.2 `ForbidRule` — `nerdmd.go:108-122`

```go
type ForbidRule struct { Match string `yaml:"match"`; Reason string `yaml:"reason"` }
```

- `match`: **substring** on slash-normalised case-insensitive path. Deliberately substring so Go and Mangle agree (`109-114`, `schemas_projectdoc.mg:40-44`).
- `reason`: required, interpolated into denial `blocked by nerd.md: <path> is write-protected (<reason>)` (`executor_tools.go:529`).

Matching must agree in two places (same `ToLower(ToSlash(…))` + `Contains`):

- Helper `Document.ForbidsPath(target)` (`281-293`) — `Contains(ToLower(ToSlash(normalized)), ToLower(ToSlash(match)))`, nil/empty-target safe.
- Live gate `Executor.projectForbidsWrite` (`executor_tools.go:406-445`) — same logic over kernel facts.

Neither glob nor regex — substring is deliberate to avoid engine disagreement across layers ("a safety gate that sometimes opens").

### 5.3 `Convention` — `nerdmd.go:124-127`

```go
type Convention struct { ID string `yaml:"id"`; Rule string `yaml:"rule"` }
```

Becomes `project_convention(ID, Rule)` (`facts.go:122-124`). Validated `id`/`rule` trimmed non-empty.

---

## 6. Component 4 — Fact Projection and Atom Normalisation

### 6.1 `Facts() []types.Fact` — `facts.go:54-131`

Contract:

- `nil` document → `nil` (pass-through for `Load` without nil check) (`55-57`).
- Always `project_doc(Path, Schema)` (`59-61`) — one fact per loaded doc, `Path` is the slash-relative `Document.Path`.
- Conditionally: `project_name` (`59-61`, `TrimSpace(Project)!=""`), `project_language` as `MangleAtom` (`63-68`, via `normalizeAtom`), `project_command ×0-4` (`70-84`), `project_command_env ×N` (`86-93`, empty-name skipped), `project_forbidden_path ×N` (`95-101`, verbatim), `project_requirement ×N` (`103-105`), `project_convention ×N` (`107-124`).
- **Never** emits `Body` (`47-52` comment: asserting free text as fact would invite policy to pattern-match natural language, which Mangle guardrails forbid).

All predicates `Args` are `string` except `project_language` and `project_command.Kind` which are `types.MangleAtom` (atoms, not strings) — disjoint in Mangle (`facts.go:65-67`).

### 6.2 `normalizeAtom(raw string) string` — `facts.go:133-156`

Used only for `Language` → `project_language/1`:

1. `TrimSpace`, strip single leading `/`, `TrimSpace` again, `ToLower`.
2. Empty → `""` (no fact).
3. Build `"/"+filtered` where `a-z0-9_` pass, `"- ."` → `"_"`, all else dropped.
4. Result `"/"` → `""` (no fact — nothing survived).

Examples: `"Go"`/`"/go"`/`"go"` → `"/go"`; `"python-3.11"` → `"/python_3_11"`; `"C++"` → `"/c"`; `"  "` → `""`. Load-bearing: atoms (`/go`) and strings (`"go"`) are disjoint — quoted string would silently never unify (`facts.go:65-67`).

### 6.3 `CommandCount() int`

Counts non-empty `Build/Test/Lint/Run` after `TrimSpace`. Used to gate `### Canonical commands` section in prompt and in tests.

---

## 7. Component 5 — Prompt Rendering

`PromptSection() string` — `facts.go:158-248`:

- `nil` → `""` (no prompt added).
- Header `## Project Instructions (<Path>)\n\n`.
- Sections when present, in order:
  1. `**Project**: <name>` if `Project` trimmed non-empty.
  2. `### Canonical commands` if any of `Build/Test/Lint/Run` non-empty — preamble `Use these exactly. Do not infer a build or test command.` + bullets `- `build`: `<cmd>`` per non-empty, then if `Env` non-empty: `Required environment for those commands:` + bullets `- `NAME=VALUE``.
  3. `### Write-protected paths (ENFORCED)` if `Forbid` non-empty — preamble `These are denied by the kernel before the tool runs, not by your judgement…` + bullets `- any path containing `Match` — Reason` — proven `TestPromptSection_StatesThatProtectionIsEnforced`.
  4. `### Required steps` if `Require` non-empty — bullets per entry.
  5. `### Conventions` if `Conventions` non-empty — bullets `- **ID**: Rule`.
  6. Verbatim trimmed `Body` + `"\n"` if non-empty — advisory prose appended last.

Body advisory non-projection is intentional — frontmatter enforcement should not require prompt surprise; prompt restates enforcement so model learns forbid before wasting a turn on a denied tool call.

Integration point: JIT prompt compiler (`internal/prompt/compiler.go` / `internal/prompt/atoms/`) injects `PromptSection` output as a project atom; executor gate remains kernel's enforcement. **ASSUMPTION:** exact call site not re-read this turn; marked as JIT injection pending verification — see `OPEN-QUESTIONS.md`.

---

## 8. Component 6 — Mangle Predicate Catalogue and Gates

Declared in `internal/core/defaults/schemas_projectdoc.mg` unless noted. No `/number` or float bounds — intentional per `mangle_scale.go` (`schemas_projectdoc.mg:9-14`).

| Predicate | Arity | Bound | Decl | Producer | Consumer | Notes |
|---|---|---|---|---|---|---|
| `project_doc(Path, Schema)` | 2 | `[/string,/string]` | `schemas_projectdoc.mg:18` | `facts.go:55-57` | `policy/projectdoc.mg:7` (`has_project_doc`), diagnostics, `/query` | Absence = no nerd.md |
| `project_name(Name)` | 1 | `[/string]` | `:21` | `facts.go:59-61` | Prompt only | Trimmed non-empty |
| `project_language(Lang)` | 1 | `[/name]` | `schemas_project.mg` (not redeclared here) | `facts.go:63-68` `MangleAtom` | JIT `/lang` dimension, `current_context(/lang,…)` | Duplicate Decl would panic at boot; shared with `nerd init` scan (`schemas_projectdoc.mg:23-31`) |
| `project_command(Kind, Command)` | 2 | `[/name,/string]` | `:37` | `facts.go:70-84` | `policy/projectdoc.mg:13` (`project_has_command`) | Kind `/build`/`/test`/`/lint`/`/run` as atom |
| `project_command_env(Name, Value)` | 2 | `[/string,/string]` | `:42` | `facts.go:86-93` | Prompt only | Empty-name skipped |
| `project_forbidden_path(Match, Reason)` | 2 | `[/string,/string]` | `:49` | `facts.go:95-101` | `executor_tools.go:422` (live), `policy:32-34` (dormant) | **Enforced** — substring |
| `project_requirement(Text)` | 1 | `[/string]` | `:53` | `facts.go:103-105` | Policy-available | No Go gate yet |
| `project_convention(ID, Rule)` | 2 | `[/string,/string]` | `:56` | `facts.go:107-124` | Policy-available | No Go gate yet |
| `has_project_doc()` | 0 | `[]` | `:63` | Derived only | Reporting / branching | `policy/projectdoc.mg:7` |
| `project_write_protected()` | 0 | `[]` | `:68` | Derived only | Cheap check before enumerating | `policy/projectdoc.mg:18` |
| `project_has_command(Kind)` | 1 | `[/name]` | `:71` | Derived only | Tool selection | `policy/projectdoc.mg:13` |

Derived but **dormant**: `project_write_denied/2` and `coder_block_write` bridge at `policy/projectdoc.mg:32-34` — exists but not consumed by executor. Live gate still queries facts directly (`executor_tools.go:406-445`).

**Live gate ordering** (`executor_tools.go`): `projectForbidsWrite` runs **before** tool execution, inside `executeToolCall` permission check. On match returns `blocked by nerd.md: <path> is write-protected (<reason>)` (`:529`), costs turn, no mutation. Verified via `TestForbidsPath*`; Mangle-side `project_write_denied` consumption is `planned:` — needs `ToLower(ToSlash(…))` equivalence proof.

### 8.1 Evidence and Uncertainty Note

- **Verified:** `nerdmd.go:1-293`, `facts.go:1-250`, `schemas_projectdoc.mg:18-71`, `policy/projectdoc.mg:7,13,32-34`, `executor_tools.go:406-445,529`, all `nerdmd_test.go#Test*` cited.
- **ASSUMPTION pending re-read:** JIT wiring exact call site in `internal/prompt/compiler.go`; whether `PromptSection` vs body alone becomes the atom; multi-workspace `Find` vs `Load` invocation frequency; dormant Mangle `contains` equivalence to Go `Contains`.

Validate locally:

```bash
go test ./internal/projectdoc -run TestParse -count=1 -v
go vet ./internal/projectdoc
python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py
```

---

## 9. Cross-Cutting Invariants

- Strict vs advisory split load-bearing — never pattern-match Markdown body as facts (`facts.go:47-52`).
- Pinned `nerd/v1` (not range-checked) — newer documents fail loudly rather than half-apply (`nerdmd.go:38-43`).
- Substring `forbid.match` (not glob) so Go and Mangle agree; case-insensitive via `ToLower` on both sides — capitalization bypass is not a gate.
- Atoms (`/go`) and strings (`"go"`) disjoint — `MangleAtom` required for `project_language` and `project_command.Kind` (`facts.go:65-67`).
- 4 MiB scanner buffer prevents long-line truncation misreported as frontmatter error (`nerdmd.go:188-189`).
- Duplicate `Decl` at same arity is kernel boot abort — `project_language/1` shared declaration intentionally not duplicated.
