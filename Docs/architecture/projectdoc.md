# Project Instructions Subsystem (`nerd.md` / `internal/projectdoc`) — Architecture Corpus

> **Status:** Draft corpus synthesized from code-read evidence 2026-08-07. Exploration budget exhausted before full Docs/Spec and policy/schema files could be read — gaps flagged inline as **UNCERTAINTY**.
> **Sources actually read:**
> - `nerd.md` (workspace root, schema `nerd/v1`)
> - `internal/projectdoc/nerdmd.go` — `Find`, `Load`, `Parse`, `splitFrontmatter`, `validate`, `ForbidsPath`
> - `internal/projectdoc/facts.go` — `Facts`, `CommandCount`, `normalizeAtom`, `PromptSection`, predicate constants
>
> **Sources NOT read this pass (inferred from comments/imports only):**
> - `internal/core/defaults/schemas_projectdoc.mg` (Decls)
> - `internal/core/defaults/policy/projectdoc.mg` (enforcement)
> - `internal/prompt/atoms/*`, `internal/prompt/compiler.go`, `internal/session/executor.go`, `internal/core/kernel_*.go`
> - `Docs/Spec/internal/projectdoc/*` (README, api-contract, etc. — listed but not opened)
> - `internal/projectdoc/nerdmd_test.go`

---

## 1. Intent & Thesis

`nerd.md` is the codeNERD analogue of `CLAUDE.md`/`AGENTS.md` with a deliberate split (per `nerdmd.go:11-28` and `nerd.md` body):

| Half | Form | Semantics | Projection |
|------|------|-----------|------------|
| **YAML frontmatter** | Strict schema, `---` fenced | **Machine-readable, kernel-enforced**. Unknown key / bad version / malformed entry = hard parse error naming the line. Never silently dropped. | Kernel facts (`project_*`) → policy derives `permitted` / denials |
| **Markdown body** | Free prose, trimmed verbatim | **Advisory**, like `CLAUDE.md` | Prompt atom (not a fact) — see §7 |

> Quote from `nerdmd.go:23-27`: *“A directive the user believes is in force but which the parser dropped is worse than no directive at all.”* Strictness is the point.

System thesis (from `nerd.md`): *The model is the creative center; the Mangle kernel is the executive. Logic determines reality; the model merely describes it. Every action must derive `permitted(...)`; default is deny.*

---

## 2. Where Things Live

| Concern | Path | Evidence |
|---------|------|----------|
| Canonical file | `nerd.md` at workspace root, fallback `.nerd/nerd.md` | `nerdmd.go:Find()` lines 88-100 |
| Parser & validation | `internal/projectdoc/nerdmd.go` | Read |
| Fact projection & prompt rendering | `internal/projectdoc/facts.go` | Read |
| Kernel fact Decls | `internal/core/defaults/schemas_projectdoc.mg` | Cited in `facts.go:5-6`, **not read — UNCERTAINTY** |
| Policy enforcement | `internal/core/defaults/policy/projectdoc.mg` | Cited in `facts.go:5-6`, **not read** |
| Prompt atoms dir | `internal/prompt/atoms/` | `nerd.md` table |
| JIT compiler (atom selection, `/lang` dimension) | `internal/prompt/compiler.go` | `nerd.md` table + comment on `Spec.Language` |
| Session execution / tool gate | `internal/session/executor.go` + `internal/core/kernel_*.go` | `nerd.md` table + `facts.go` comment on forbid |
| Spec corpus (should mirror this doc) | `Docs/Spec/internal/projectdoc/` (README, api-contract, configuration, current-state, data-flow, dependencies, design-decisions, error-taxonomy, failure-modes, gap-analysis, glossary, north-star, observability, performance-profile, safety-model, test-strategy, todos, wiring) | Glob listed ~18 files, **not read** |
| Example document | `nerd.md` itself (project `codeNERD`, language `go`, build/test/lint/env, 2 forbid, 3 require, 4 conventions) | Read |

---

## 3. Spec Schema (`Spec` struct, `nerdmd.go:45-78`)

```go
type Spec struct {
  Schema      string            `yaml:"schema"`               // required, must == "nerd/v1"
  Project     string            `yaml:"project,omitempty"`
  Language    string            `yaml:"language,omitempty"`
  Commands    Commands          `yaml:"commands,omitempty"`
  Forbid      []ForbidRule      `yaml:"forbid,omitempty"`
  Require     []string          `yaml:"require,omitempty"`
  Conventions []Convention      `yaml:"conventions,omitempty"`
}
type Commands struct {
  Build string            `yaml:"build,omitempty"`
  Test  string            `yaml:"test,omitempty"`
  Lint  string            `yaml:"lint,omitempty"`
  Run   string            `yaml:"run,omitempty"`
  Env   map[string]string `yaml:"env,omitempty"` // e.g. CGO_CFLAGS
}
type ForbidRule struct {
  Match  string `yaml:"match"`  // substring, slash-normalized, case-insensitive
  Reason string `yaml:"reason"` // required, shown on denial
}
type Convention struct {
  ID   string `yaml:"id"`
  Rule string `yaml:"rule"`
}
```

**Schema versioning:** `const SchemaVersion = "nerd/v1"` — pinned, not range-checked. A newer document fails loudly with message naming both versions (`validate()`). Adding any field is a breaking change requiring a bump — older binaries reject unknown keys (see §4).

Current `nerd.md` exercises all fields:
```yaml
schema: nerd/v1
project: codeNERD
language: go
commands:
  build: go build -o nerd.exe ./cmd/nerd
  test: go test ./...
  lint: go vet ./...
  env: { CGO_CFLAGS: -IC:/CodeProjects/codeNERD/sqlite_headers }
forbid:
  - {match: .nerd/config.json, reason: "Live user-owned runtime config..."}
  - {match: .nerd/mangle/learned, reason: "Persistent kernel overlay..."}
require:
  - "Run `go test ./...` before declaring work complete."
  - "All Mangle predicates need a Decl before use, and no predicate may be declared twice at the same arity..."
  - "New LLM-facing behaviour becomes a prompt atom under internal/prompt/atoms/..."
conventions:
  - {id: conventional-commits, rule: "Commit subjects use ..."}
  - {id: mangle-atoms-are-lowercase, rule: "Mangle variables are UPPERCASE..."}
  - {id: numbers-are-int64, rule: "Every numeric Mangle slot is /number..."}
  - {id: audit-before-delete, rule: "Look for wiring gaps before deleting..."}
```

---

## 4. Parse & Validation Pipeline

### 4.1 Discovery — `Find(workspace string) string`

Search order (strict):
1. `<workspace>/nerd.md`
2. `<workspace>/.nerd/nerd.md`
3. `""` (absent = not an error; optional file) — `nerdmd.go:94-102`

**UNCERTAINTY:** Whether discovery is invoked per-turn, at session boot, or both; and how it interacts with multi-workspace or `Find` vs `Load` callers — not read.

### 4.2 Loading — `Load(workspace string) (*Document, error)`

- Calls `Find`; if `""` returns `(nil, nil)`.
- `os.ReadFile` → `Parse`; on error wraps with `fmt.Errorf("%s: %w", path, err)`.
- On success sets `doc.Path` to `filepath.Rel` slash-normalized relative path (or absolute slash-normalized fallback).
- **Invariant:** An unreadable/invalid directive never degrades to “no directive” — caller must handle error.

### 4.3 Splitting — `splitFrontmatter(data []byte)`

- Uses `bufio.Scanner` with `Buffer(0, 4*1024*1024)` (4 MiB) — comment: bodies embed long tables/commands that would truncate at default 64 KiB.
- First line must be `---` (trimmed); else error naming got value: ``first line must be "---" to open...``
- Collects lines until closing `---`; if never closed → error.
- Remainder is body (joined with `\n`). `scanner.Err` surfaced.

### 4.4 Decoding — `Parse(data []byte)`

```go
decoder := yaml.NewDecoder(bytes.NewReader(front))
decoder.KnownFields(true) // THE whole point — unknown key = hard fail
decoder.Decode(&spec)
spec.validate()
return &Document{Spec: spec, Body: strings.TrimSpace(body)}
```

- `KnownFields(true)` ensures author learns immediately that a directive is not honoured.
- Body is `strings.TrimSpace` preserved verbatim (advisory only).

### 4.5 Validation — `Spec.validate() error`

- `schema` trimmed empty → `frontmatter is missing required "schema"; expected "nerd/v1"`
- `schema != SchemaVersion` → `unsupported schema "X"; this build speaks "nerd/v1". Refusing to half-apply...`
- Each `forbid[i]`: `match` trimmed empty → `forbid[i] has empty "match"; a rule that matches every path would deny every write`; `reason` trimmed empty → explicit message with match value.
- Each `conventions[i]`: `id`/`rule` trimmed empty → errors naming index and id.
- Each `require[i]`: trimmed empty → `require[i] is empty`.

---

## 5. Fact Projection — `facts.go:Facts() []types.Fact`

Only frontmatter is projected — comment: *“asserting free text as a fact would invite policy to pattern-match natural language, which Mangle guardrails explicitly forbid.”* Returns `nil` for `nil` document (Load passthrough without nil check).

| Predicate | Args | When | Notes |
|-----------|------|------|-------|
| `project_doc` | `(Path, Schema)` | Always (if doc non-nil) | `PredPresent`; `Path` is slash-relative from `Load` |
| `project_name` | `(Name)` | `strings.TrimSpace(Project) != ""` | |
| `project_language` | `(Lang)` | `normalizeAtom(Language) != ""` | `Lang` is `types.MangleAtom("/go")` etc., **not** string — atoms vs strings are disjoint in Mangle; quoted `"go"` would never unify (`facts.go:38-42`) |
| `project_command` | `(Kind, Command)` | For each non-empty of build/test/lint/run | `Kind` is `types.MangleAtom("/build")` etc. |
| `project_command_env` | `(Name, Value)` | For each `Env` entry with non-empty trimmed name | |
| `project_forbidden_path` | `(Match, Reason)` | Per `Forbid` | **ENFORCED** — executor refuses write-mutation before tool runs |
| `project_requirement` | `(Text)` | Per `Require` | Surfaced to model + available to policy |
| `project_convention` | `(ID, Rule)` | Per `Convention` | |

**Decls:** Asserted to live in `internal/core/defaults/schemas_projectdoc.mg` (not read). **UNCERTAINTY:** Exact arities/types, whether `project_doc` is declared as `(string, string)` vs atom, and duplicate-Decl guard interaction (nerd.md `require` notes duplicate Decl at same arity takes kernel down at boot).

**Helper `normalizeAtom(raw string) string`:**
- Trim, strip leading `/`, lower-case, then rebuild: keep `a-z0-9_`, map `-`, ` `, `.` → `_`, drop all else.
- If result == `"/"` or empty → `""` (emit no fact rather than unparseable).
- Called only for `Language`.

**Helper `CommandCount() int`:** Counts non-empty build/test/lint/run.

---

## 6. Enforcement — `ForbidsPath` & Kernel Gate

### 6.1 In-process check — `Document.ForbidsPath(target string) (reason string, forbidden bool)`

- Nil/empty target → false.
- `normalized := strings.ToLower(filepath.ToSlash(target))`
- For each rule: `strings.Contains(normalized, strings.ToLower(filepath.ToSlash(rule.Match)))` → match → return `rule.Reason, true`.
- **Semantics:** Substring, not glob — deliberate so Go and Mangle agree (comment in `ForbidRule`). Case-insensitive — “a different capitalization walks through is not a gate.”

### 6.2 Kernel enforcement (inferred, **UNCERTAINTY — policy not read**)

- `project_forbidden_path` facts asserted at boot.
- Policy in `internal/core/defaults/policy/projectdoc.mg` derives denials.
- `internal/session/executor.go` checks before any write-mutation tool runs; per `nerd.md` table: *“Attempting one costs a turn and changes nothing.”*
- Same `Match` string is used both in Go (`ForbidsPath`) and in Mangle (`contains`-style) — must stay identical or gate “sometimes opens.”
- `project_language`, `project_command*`, `project_requirement`, `project_convention` are advisory/informational to model and/or available to policy but not hard gates (except possibly conventions via lint policy — unknown).

---

## 7. Prompt Injection — `PromptSection() string`

Model cannot read fact store directly, so frontmatter is restated in prose to avoid “surprise denial” wasting a turn. Enforcement remains kernel’s.

Rendering order (when doc non-nil, `strings.Builder`):

1. `## Project Instructions (<Path>)\n\n`
2. `**Project**: <name>` if present
3. `### Canonical commands` if any of build/test/lint/run present:
   - Preamble: *“Use these exactly. Do not infer a build or test command.”*
   - Bullet `- `build/test/lint/run`: `<cmd>`` for each non-empty
   - If `Env` non-empty: `Required environment for those commands:` + bullets `- `NAME=VALUE``
4. `### Write-protected paths (ENFORCED)` if forbid non-empty:
   - Preamble: *“These are denied by the kernel before the tool runs, not by your judgement...”*
   - Bullet `- any path containing `Match` — Reason`
5. `### Required steps` if require non-empty — bullets
6. `### Conventions` if conventions non-empty — bullets `- **ID**: Rule`
7. Body (trimmed) if non-empty, appended verbatim

Returns `""` for nil doc.

**Integration point UNCERTAINTY:** Where `PromptSection` is called — presumably `internal/prompt/compiler.go` JIT compilation, or a prompt atom under `internal/prompt/atoms/` per `nerd.md` require rule *“New LLM-facing behaviour becomes a prompt atom”*. Whether body alone or full `PromptSection` becomes the atom is not confirmed (facts.go says body belongs in prompt, not fact store; nerd.md says body becomes a project prompt atom).

---

## 8. Data Flow (Observed → Hypothesized)

```
workspace/nerd.md ─┐
                   ├─ Find → Load → Parse (splitFrontmatter → yaml KnownFields → validate)
                   │                ├─ error → surface to user (never silent)
                   │                └─ *Document{Path, Spec, Body}
                   │
                   ├─ Facts() ──→ []types.Fact ──→ kernel overlay (schemas_projectdoc.mg Decls)
                   │                              └─ policy/projectdoc.mg derives permitted / denials
                   │                                                   └─ executor gate on write tools
                   │
                   └─ PromptSection() ──→ prompt atom / JIT context (/lang, commands, forbid, require, conventions, body)
```

**Cross-cutting invariants from `nerd.md` conventions:**
- Numbers are `int64` only (`types.PercentFromRatio` scaling) — `Language`/other atoms must not introduce float facts.
- Mangle vars `UPPERCASE`, atoms `/lowercase` — disjoint, never unify (explains `types.MangleAtom` usage).
- Conventional commits, audit-before-delete (wiring gaps), comprehensive testing — apply to any change here.

---

## 9. Failure Modes & Safety Model (from code comments)

| Failure | Detection | Consequence |
|---------|-----------|-------------|
| Unknown YAML key | `KnownFields(true)` | Hard parse error — author learns directive not honoured |
| Wrong schema version | `validate` | Loud failure naming have/want, refuses half-apply |
| Empty forbid match | `validate` | Fail — would deny every write |
| Empty forbid reason | `validate` | Fail — denial without explanation invites workaround |
| Missing closing `---` | `splitFrontmatter` | `frontmatter block opened with "---" was never closed` |
| Body line >64 KiB without enlarged buffer | Would truncate | Mitigated by 4 MiB scanner buffer |
| Float fact from language/ratio | Kernel fixpoint abort | Must use `PercentFromRatio` / `normalizeAtom` int/atom only |
| Duplicate predicate Decl at same arity | Kernel boot abort | Per `nerd.md` require — whole kernel down, not just rule |
| Case-variant bypass of forbid | — | Mitigated by `ToLower` + `ToSlash` on both sides |

---

## 10. What the Spec Corpus Should Contain (Gap Analysis)

`Docs/Spec/internal/projectdoc/` currently lists 18 placeholder/template files (per glob 2026-08-07). To complete the corpus, each should be filled from the evidence above — **do not invent beyond evidence; mark gaps:**

- **README.md** — One-paragraph thesis + table of §2, link to `nerd.md` example.
- **api-contract.md** — `Find`, `Load`, `Parse`, `Facts`, `PromptSection`, `ForbidsPath`, `CommandCount`, `normalizeAtom` signatures, `Document`/`Spec` types, error strings verbatim.
- **configuration.md** — Schema `nerd/v1` pinned, all frontmatter keys, Env map, forbid substring semantics.
- **current-state.md** — What is implemented (parser, validation, facts, prompt, ForbidsPath) vs what is wired to kernel/prompt (UNCERTAINTY).
- **data-flow.md** — Diagram §8 with file paths.
- **dependencies.md** — `gopkg.in/yaml.v3`, `internal/types`, `internal/core/defaults/schemas_projectdoc.mg`, `policy/projectdoc.mg`, `internal/prompt/compiler.go`.
- **design-decisions.md** — Strict vs advisory split, substring not glob, KnownFields, 4 MiB buffer, nil-doc nil-facts passthrough, case-insensitive match.
- **error-taxonomy.md** — All `validate`/`splitFrontmatter`/`Parse` error strings.
- **failure-modes.md** — Table §9 + kernel boot abort modes.
- **gap-analysis.md** — List UNCERTAINTYs below.
- **glossary.md** — `project_doc`, `project_forbidden_path`, etc., `MangleAtom`, `KnownFields`, `PromptSection`.
- **north-star.md** — “Zero silent directive drops; every forbid is a denied tool call, not a suggestion.”
- **observability.md** — How parse errors surface (path-prefixed), how denials surface (reason shown), logging? (UNCERTAINTY)
- **performance-profile.md** — Scanner buffer, yaml decode cost, fact count = 1 + ... (bounded small).
- **safety-model.md** — `project_forbidden_path` kernel gate, why substring+ToLower+ToSlash, .nerd/config.json and .nerd/mangle/learned incidents.
- **test-strategy.md** — Mirror `nerdmd_test.go` (not read) — hypothesized: valid/invalid frontmatter, unknown keys, schema mismatch, forbid validation, ForbidsPath case/slash, normalizeAtom, PromptSection rendering, Load missing file.
- **todos.md** — Wire `PromptSection` to prompt atoms, assert Decls, add policy tests, audit wiring gaps.
- **wiring.md** — Call graph: who calls `Load`/`Facts`/`PromptSection`; where facts enter kernel; where prompt section enters JIT.

---

## 11. Open Questions & Uncertainty Log (Must be resolved by reading the unread files)

1. **Schema Decls** — Exact `Decl` forms in `schemas_projectdoc.mg` (arity, types `/string` vs `/atom` vs `/number`) and whether `project_doc` path is string or atom.
2. **Policy enforcement** — Does `policy/projectdoc.mg` use `project_forbidden_path` to derive `deny_edit`/`permitted`? Exact rule shape, and whether `project_requirement`/`project_convention` have policy teeth.
3. **Prompt wiring** — Is `PromptSection()` wrapped as `internal/prompt/atoms/projectdoc.md` atom, or injected directly via `compiler.go`? How does `Language` seed `current_context(/lang, ...)`?
4. **Load call sites** — Boot vs per-turn, handling of `.nerd/nerd.md` precedence, interaction with `internal/session/executor.go`.
5. **Tests** — `nerdmd_test.go` coverage and any missing edge tests (empty file, BOM, CRLF, symlinks).
6. **Docs/Spec templates** — Whether those 18 files are intended to be hand-written or generated from code annotations.
7. **Numbers-as-int64** — Any numeric frontmatter planned (not currently) and how `PercentFromRatio` would apply.

> **Confidence:** 0.92 for §§3-7 (direct code read); 0.55 for §§6.2, 8, 11 wiring/policy claims (comments only). Recommend re-running with read budget to open `schemas_projectdoc.mg`, `policy/projectdoc.mg`, `compiler.go`, `executor.go`, and one `Docs/Spec/internal/projectdoc/*.md` to confirm.

---

## 12. Minimal Next Investigative Steps (if budget restored)

1. `read_file internal/core/defaults/schemas_projectdoc.mg` + `policy/projectdoc.mg`
2. `read_file internal/prompt/compiler.go` (search `PromptSection`/`projectdoc`) + `read_file internal/session/executor.go` (search `ForbidsPath`/`project_forbidden_path`)
3. `read_file internal/projectdoc/nerdmd_test.go` + one `Docs/Spec/internal/projectdoc/README.md` to align corpus template
4. `glob internal/core/defaults/**/*` to confirm all `project_*` Decls and no duplicates

---

*Generated under constrained budget — file written as deliverable per instruction to note uncertainty inside artifact rather than defer.*
