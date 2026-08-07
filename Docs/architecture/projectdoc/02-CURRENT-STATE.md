# nerd.md Project-Instruction Subsystem — Current State

> **Scope:** This document is a precise, citation-backed spec of the `nerd.md` subsystem as it exists on `main` today. It covers the `nerd/v1` frontmatter schema field-by-field, every Mangle predicate emitted (and derived), the exact position of the write-protection gate in the tool-call path, and what is declared but not yet wired. Every non-obvious claim cites `file:line`.

> **Status source-of-truth files (read 2026-05-13):**
> - `internal/projectdoc/nerdmd.go` — loader, parser, validator, file discovery, `ForbidsPath`
> - `internal/projectdoc/facts.go` — predicate constants, `Facts()`, `PromptSection()`, atom normalisation
> - `internal/core/defaults/schemas_projectdoc.mg` — EDB/IDB predicate declarations
> - `internal/core/defaults/policy/projectdoc.mg` — derived predicates and the dormant `project_write_denied` / `coder_block_write` bridge
> - `internal/session/executor_tools.go` — `projectForbidsWrite` gate and its ordering inside `executeToolCall`

---

## 1. What `nerd.md` is

`nerd.md` is the per-project instruction file, intentionally split along the codeNERD thesis ("model is creative center, Mangle kernel is executive") — `internal/projectdoc/nerdmd.go:1-26`:

- **YAML frontmatter** (`---`-fenced, strict schema) → kernel facts → policy-enforced. A forbidden path becomes a **denied tool call**, not prompt advice.
- **Markdown body** (everything after the second `---`) → a single project prompt atom, **advisory only**. Never projected as facts (`internal/projectdoc/facts.go:47-52`).

Comparison made explicit in the source: `CLAUDE.md` prose is appended to the prompt; `nerd.md` frontmatter is not advice (`internal/projectdoc/nerdmd.go:9-17`).

The file is **optional**. Absence is `(*Document, nil) == (nil, nil)` — not an error (`internal/projectdoc/nerdmd.go:138-160`).

---

## 2. Discovery, loading, and parsing

### 2.1 Canonical filename and schema version

| Constant | Value | Location |
|---|---|---|
| `FileName` | `"nerd.md"` | `internal/projectdoc/nerdmd.go:36` |
| `SchemaVersion` | `"nerd/v1"` | `internal/projectdoc/nerdmd.go:43` |

`SchemaVersion` is **pinned, not range-checked** on purpose; a document written for a newer binary fails loudly rather than being half-understood (`internal/projectdoc/nerdmd.go:38-43`).

### 2.2 Search order

`Find(workspace)` checks in order and returns the first hit (`internal/projectdoc/nerdmd.go:131-140`):

1. `<workspace>/nerd.md`
2. `<workspace>/.nerd/nerd.md`

Returns `""` when absent. `os.Stat` + `!IsDir` guards against a directory masquerading as the file.

### 2.3 Load

```go
func Load(workspace string) (*Document, error) // internal/projectdoc/nerdmd.go:145
```

- Returns `(nil, nil)` if `Find` returns `""`.
- On hit, `os.ReadFile(path)` then `Parse(data)`.
- Wraps errors as `read <path>: …` or `<path>: …` so the caller can surface which file failed.
- On success, `doc.Path` is set to `filepath.Rel(workspace, path)` slash-normalised, falling back to the absolute slash-normalised path (`internal/projectdoc/nerdmd.go:153-158`).

Unreadable or invalid when present **never degrades to "no directive"** — it returns an error (`internal/projectdoc/nerdmd.go:147-151`).

### 2.4 Parse and frontmatter extraction

```go
func Parse(data []byte) (*Document, error) // internal/projectdoc/nerdmd.go:163
```

1. `splitFrontmatter(data)` — extracts the leading `---`-fenced YAML block (`internal/projectdoc/nerdmd.go:182-220`).
   - First line must be exactly `---` (trimmed); otherwise `first line must be "---" to open the frontmatter block (got …); nerd.md requires machine-readable frontmatter, unlike CLAUDE.md` (`internal/projectdoc/nerdmd.go:199-203`).
   - Second `---` (trimmed) closes the block; missing close → `frontmatter block opened with "---" was never closed` (`internal/projectdoc/nerdmd.go:214`).
   - Body is everything after the closing fence, joined with `"\n"`.
   - Scanner buffer is grown to `4 MiB` (default 64 KiB) because bodies embed long lines such as tables (`internal/projectdoc/nerdmd.go:188-189`).
   - Empty file → `file is empty; expected a "---" frontmatter block` (`internal/projectdoc/nerdmd.go:196`).
2. Strict YAML decode with `decoder.KnownFields(true)` — **unknown keys are a hard error**, not silently dropped, because a directive the author believes is in force but the parser ignores is worse than no directive (`internal/projectdoc/nerdmd.go:168-175`).
3. `spec.validate()` (`internal/projectdoc/nerdmd.go:177`).
4. Returns `&Document{Spec: spec, Body: strings.TrimSpace(body)}` (`internal/projectdoc/nerdmd.go:181`).

### 2.5 Document shape

```go
type Document struct {              // internal/projectdoc/nerdmd.go:48
    Path string                     // relative slash-normalised path to the file
    Spec Spec                       // strict machine-readable frontmatter
    Body string                     // verbatim Markdown after frontmatter, trimmed; advisory
}
```

`Body` is stored trimmed but otherwise verbatim; it is never fact-projected.

---

## 3. `nerd/v1` frontmatter schema — field by field

Type definitions at `internal/projectdoc/nerdmd.go:61-127`. YAML tags are shown; `omitempty` everywhere except `schema`.

### 3.1 Top-level `Spec`

| YAML key | Go field | Go type | Required | YAML tag | Location | Semantics |
|---|---|---|---|---|---|---|
| `schema` | `Schema` | `string` | **Yes** | `yaml:"schema"` | `nerdmd.go:65` | Must equal `SchemaVersion` (`"nerd/v1"`). Pins the contract. Validated `validate:238-245`. Adding any field is a schema change that must bump `SchemaVersion` (`nerdmd.go:60-62`). |
| `project` | `Project` | `string` | No | `yaml:"project,omitempty"` | `nerdmd.go:68` | Human-readable project name. Surfaced in prompts only; also emitted as `project_name/1`. Empty/whitespace trimmed and ignored. |
| `language` | `Language` | `string` | No | `yaml:"language,omitempty"` | `nerdmd.go:73` | Primary language tag e.g. `"go"`, `"python"`. Seeds JIT compilation context `/lang` dimension so language-specific atoms are selected without opening a file. Normalised to a Mangle atom (see §5.2). Empty/whitespace ignored. |
| `commands` | `Commands` | `Commands` | No | `yaml:"commands,omitempty"` | `nerdmd.go:78` | Canonical build/test/lint/run invocations. See §3.2. |
| `forbid` | `Forbid` | `[]ForbidRule` | No | `yaml:"forbid,omitempty"` | `nerdmd.go:83` | Write-protection rules. **ENFORCED** — become `project_forbidden_path/2` and are checked before any write-mutation tool runs. See §3.3. |
| `require` | `Require` | `[]string` | No | `yaml:"require,omitempty"` | `nerdmd.go:88` | Non-negotiable steps (e.g. `"run go test ./... before handoff"`). Surfaced to the model in prose and available to policy as `project_requirement/1`. No enforcement gate today; see §9. |
| `conventions` | `Conventions` | `[]Convention` | No | `yaml:"conventions,omitempty"` | `nerdmd.go:92` | Named, checkable project rules. Surfaced to the model and as `project_convention/2`. No enforcement gate today; see §9. |

Unknown top-level keys → hard parse error via `KnownFields(true)` (`nerdmd.go:171-174`).

### 3.2 `Commands`

```go
type Commands struct {              // internal/projectdoc/nerdmd.go:95
    Build string            `yaml:"build,omitempty"` // nerdmd.go:96
    Test  string            `yaml:"test,omitempty"`  // nerdmd.go:97
    Lint  string            `yaml:"lint,omitempty"`  // nerdmd.go:98
    Run   string            `yaml:"run,omitempty"`   // nerdmd.go:99
    Env   map[string]string `yaml:"env,omitempty"`   // nerdmd.go:103
}
```

| Field | Type | Required | Purpose |
|---|---|---|---|
| `build` | `string` | No | Canonical build invocation. |
| `test` | `string` | No | Canonical test invocation. |
| `lint` | `string` | No | Canonical lint invocation. |
| `run` | `string` | No | Canonical run invocation. |
| `env` | `map[string]string` | No | Environment variables that must be set for the commands above. Exists because `CGO_CFLAGS`-style prerequisites are invisible in the command string and their absence fails far from the cause (`nerdmd.go:100-103`). Empty map key entries are ignored when projecting facts (`facts.go:107-114`). |

Comment at `nerdmd.go:75-77`: without these the agent guesses, and a guess that compiles on the maintainer's machine is the most expensive kind of wrong.

### 3.3 `ForbidRule`

```go
type ForbidRule struct {            // internal/projectdoc/nerdmd.go:108
    Match  string `yaml:"match"`    // nerdmd.go:115
    Reason string `yaml:"reason"`   // nerdmd.go:121
}
```

| Field | Type | Required | Validation | Location | Semantics |
|---|---|---|---|---|---|
| `match` | `string` | **Yes** (per entry) | `TrimSpace != ""` else `forbid[i] has an empty "match"; a rule that matches every path would deny every write` | `nerdmd.go:247-249` | **Substring** (not glob) matched against the slash-normalised, case-insensitive target path. Deliberately substring so Go and Mangle agree on meaning; a glob engine that disagrees across layers is a safety gate that sometimes opens (`nerdmd.go:109-114` and `schemas_projectdoc.mg:40-44`). Stored verbatim as `Args[0]` of `project_forbidden_path/2`. |
| `reason` | `string` | **Yes** | `TrimSpace != ""` else `forbid[i] (match "…") has no "reason"; a denial the agent cannot explain reads as a malfunction and invites a workaround` | `nerdmd.go:250-255` | Shown to the model and user when the write is denied. Required. Becomes `Args[1]` of `project_forbidden_path/2` and is interpolated into the denial error `blocked by nerd.md: <path> is write-protected (<reason>)` (`executor_tools.go:529`). |

Matching is implemented in two places that must agree:

- **Go helper** `Document.ForbidsPath(target)` (`nerdmd.go:281-293`): `strings.Contains(strings.ToLower(filepath.ToSlash(normalized)), strings.ToLower(filepath.ToSlash(rule.Match)))`, nil/empty-target safe.
- **Kernel gate** `Executor.projectForbidsWrite` (`executor_tools.go:406-445`): same `ToLower(ToSlash(…))` + `Contains` loop over `project_forbidden_path` facts.

Neither is the "live" gate today — `ForbidsPath` is a helper; the live gate is `projectForbidsWrite` querying the kernel (see §7).

### 3.4 `Convention`

```go
type Convention struct {            // internal/projectdoc/nerdmd.go:124
    ID   string `yaml:"id"`         // nerdmd.go:125
    Rule string `yaml:"rule"`       // nerdmd.go:126
}
```

| Field | Type | Required | Validation | Location |
|---|---|---|---|---|
| `id` | `string` | **Yes** | `TrimSpace != ""` else `conventions[i] has an empty "id"` | `nerdmd.go:258-259` |
| `rule` | `string` | **Yes** | `TrimSpace != ""` else `conventions[i] (id "…") has an empty "rule"` | `nerdmd.go:260-263` |

Becomes `project_convention(ID, Rule)` (`facts.go:122-124`).

### 3.5 Validation summary (`Spec.validate`, `nerdmd.go:232-272`)

| Check | Error |
|---|---|
| `Schema` missing/whitespace | `frontmatter is missing the required "schema" key; expected "nerd/v1"` (`nerdmd.go:238`) |
| `Schema != "nerd/v1"` | `unsupported schema "…" ; this build speaks "nerd/v1". Refusing to half-apply…` (`nerdmd.go:240-245`) |
| `forbid[i].match` empty | `forbid[i] has an empty "match"` (`nerdmd.go:248`) |
| `forbid[i].reason` empty | `forbid[i] (match "…") has no "reason"` (`nerdmd.go:251`) |
| `conventions[i].id` empty | `conventions[i] has an empty "id"` (`nerdmd.go:259`) |
| `conventions[i].rule` empty | `conventions[i] (id "…") has an empty "rule"` (`nerdmd.go:261`) |
| `require[i]` empty/whitespace | `require[i] is empty` (`nerdmd.go:265-268`) |
| Unknown YAML key anywhere | `invalid frontmatter: yaml: unmarshal errors: line N: field <key> not found in type projectdoc.Spec` (via `KnownFields(true)`, `nerdmd.go:172-174`) |

On any validation error `Parse` returns `nil, err` and `Load` wraps it as `<path>: …`.

---

## 4. Mangle predicate catalogue

### 4.1 Declarations — `internal/core/defaults/schemas_projectdoc.mg`

Every predicate below is `Decl`'d in `schemas_projectdoc.mg` and produced/consumed as noted. Bounds are all `/string` or `/name`; no `/number` or float bounds appear — intentional per `mangle_scale.go` discussion (`schemas_projectdoc.mg:9-14`).

| Predicate | Arity | Bound | Decl line | Producer (Go) | Consumer(s) | Notes |
|---|---|---|---|---|---|---|
| `project_doc(Path, Schema)` | 2 | `[/string, /string]` | `schemas_projectdoc.mg:18` | `facts.go:55-57` — one fact per loaded doc: `Args: [Path, Schema]` | `policy/projectdoc.mg:7` (`has_project_doc`), diagnostics, `/query` | Absence means no `nerd.md`. Path is the slash-normalised `Document.Path`. |
| `project_name(Name)` | 1 | `[/string]` | `schemas_projectdoc.mg:21` | `facts.go:59-61` — only if `TrimSpace(Project) != ""` | Prompt rendering; no policy gate | |
| `project_language(Lang)` | 1 | `[/name]` | **Not declared here** — declared in `internal/core/defaults/schemas_project.mg` (`schemas_projectdoc.mg:23-31`) | `facts.go:63-68` — only if `normalizeAtom(Language) != ""`; emitted as `types.MangleAtom(lang)` (atom, not string) | JIT language dimension, `current_context(/lang, …)` | Deliberately not redeclared; duplicate `Decl` is a hard analysis error that takes the kernel down at boot. `nerd.md` writes to the same predicate that `nerd init`'s codebase scan populates so language queries need not know the source. |
| `project_command(Kind, Command)` | 2 | `[/name, /string]` | `schemas_projectdoc.mg:37` | `facts.go:70-84` — iterated over `map{"/build": Build, "/test": Test, "/lint": Lint, "/run": Run}`; empty commands skipped | `policy/projectdoc.mg:13` (`project_has_command`) | `Kind` is `/build`, `/test`, `/lint`, or `/run` as an atom (`types.MangleAtom(kind)`). |
| `project_command_env(Name, Value)` | 2 | `[/string, /string]` | `schemas_projectdoc.mg:42` | `facts.go:86-93` — one per `Commands.Env` entry; empty-name entries skipped | Prompt rendering only | |
| `project_forbidden_path(Match, Reason)` | 2 | `[/string, /string]` | `schemas_projectdoc.mg:49` | `facts.go:95-101` — one per `Forbid` entry, verbatim | `executor_tools.go:422` (live gate), `policy/projectdoc.mg:32-34` (dormant derived rule) | **The enforced predicate.** Match is a substring; see §3.3. |
| `project_requirement(Text)` | 1 | `[/string]` | `schemas_projectdoc.mg:53` | `facts.go:103-105` — one per `Require` entry | Policy-available; no Go gate queries it | No enforcement today. |
| `project_convention(ID, Rule)` | 2 | `[/string, /string]` | `schemas_projectdoc.mg:56` | `facts.go:107-109` — one per `Convention` entry (`facts.go:122-124`) | Policy-available; no Go gate queries it | No enforcement today. |
| `has_project_doc()` | 0 | `[]` | `schemas_projectdoc.mg:63` | Derived only | Reporting / branching on "instructions exist without binding Path" | |
| `project_write_protected()` | 0 | `[]` | `schemas_projectdoc.mg:68` | Derived only | Cheap "is anything protected?" check before enumerating rules | |
| `project_has_command(Kind)` | 1 | `[/name]` | `schemas_projectdoc.mg:71` | Derived only | | |

### 4.2 Atom normalisation for `project_language`

`normalizeAtom(raw)` at `facts.go:133-156`:

1. `TrimSpace`, strip leading `/`, `TrimSpace` again, `ToLower`.
2. If empty → `""` (no fact).
3. Build `"/" + filtered runes` where `a-z`, `0-9`, `_` pass through, `"- ."` map to `"_"`, everything else dropped.
4. If result is `"/"` (nothing survived) → `""` (no fact).

So `"Go"`, `"/go"`, `"go"` all become `"/go"`; `"C++"` becomes `"/c"`; `"python-3.11"` → `"/python_3_11"`. This matters because Mangle atoms (`/go`) and strings (`"go"`) are disjoint — a quoted string would silently never unify (`facts.go:65-67`).

### 4.3 `Facts()` projection function

```go
func (d *Document) Facts() []types.Fact // internal/projectdoc/facts.go:54
```

- `nil` document → `nil` (so `Load` result can be passed straight through without a nil check, `facts.go:55-57`).
- Always emits `project_doc(Path, Schema)` (`facts.go:59-61`).
- Conditionally emits `project_name`, `project_language`, `project_command{×0-4}`, `project_command_env{×N}`, `project_forbidden_path{×N}`, `project_requirement{×N}`, `project_convention{×N}` as above.
- **Never emits the Markdown body** — asserting free text as a fact would invite policy to pattern-match natural language, which guardrails forbid (`facts.go:47-52`).
- `CommandCount()` helper at `facts.go:115-128` counts non-empty Build/Test/Lint/Run; used by diagnostics only.

### 4.4 Derived predicates — `internal/core/defaults/policy/projectdoc.mg`

```mangle
has_project_doc() :- project_doc(_, _).                    // policy/projectdoc.mg:7
project_write_protected() :- project_forbidden_path(_, _). // policy/projectdoc.mg:10
project_has_command(Kind) :- project_command(Kind, _).      // policy/projectdoc.mg:13
```

These are thin markers so callers can branch cheaply.

The **write-protection derived rules** at `policy/projectdoc.mg:22-39`:

```mangle
Decl project_write_denied(Path, Reason) bound [/string, /string]. // policy/projectdoc.mg:31
project_write_denied(Path, Reason) :-
    pending_edit(Path, _),
    project_forbidden_path(Match, Reason),
    path_contains(Path, Match).                    // policy/projectdoc.mg:32-35

coder_block_write(Path, Reason) :-
    project_write_denied(Path, Reason).            // policy/projectdoc.mg:38-39
```

Comment at `policy/projectdoc.mg:16-27` states explicitly: the Go gate queries `project_forbidden_path` directly **before** any write tool runs, so denial does not depend on `project_write_denied` being reachable. A safety gate that silently stops firing because an upstream rule stopped deriving is worse than no gate. The `pending_edit`-based rule exists so that once `pending_edit` is wired, protection turns on across the transaction path for free.

---

## 5. Tool-call gate — ordering and mechanics

### 5.1 Where the gate sits

Inside `Executor.executeToolCall(ctx, call, cfg)` at `internal/session/executor_tools.go:485-575`, immediately before any tool handler runs.

Exact ordering after the current source (lines ~493-560, see §5.3 table):

```
isToolAllowed  →  checkSafety  →  projectForbidsWrite  →  PreflightDestructiveToolCall  →  handler
     (JIT)          (Constitutional)    (nerd.md)              (Dreamer/VirtualStore)
```

Comment at `executor_tools.go:508-521` is the design rationale in one paragraph:

> This is the line that makes `nerd.md`'s frontmatter different in kind from `CLAUDE.md`. A "never touch `config.json`" written in prose is a request the model complies with most of the time; a `project_forbidden_path` fact is checked here, before the tool runs, and no amount of model conviction gets past it. It sits after `checkSafety` and before the Dreamer preflight on purpose: constitutional rules outrank project rules, and there is no reason to simulate the consequences of an action that is already denied.

### 5.2 `projectForbidsWrite` — implementation

```go
func (e *Executor) projectForbidsWrite(call ToolCall) (string, bool) // executor_tools.go:420
```

Steps (`executor_tools.go:420-445`):

1. Guard: if `!isWriteMutationTool(call.Name)` → `("", false)` — reading a protected file is allowed and often necessary (`executor_tools.go:421-423`).
2. Extract target path via `projectDocTargetPath(call.Args)` (`executor_tools.go:424-427`, impl at `executor_tools.go:391-402`). Checks **eight** arg names in order — `path`, `file_path`, `filepath`, `file`, `filename`, `target`, `dest`, `destination` (`executor_tools.go:391`) — because tools disagree on the name and a gate that only fires for one convention has holes.
3. If target is `""` or `e.kernel == nil` → `("", false)` (`executor_tools.go:424-431`).
4. `facts, err := e.kernel.Query(projectdoc.PredForbiddenPath)` where `PredForbiddenPath == "project_forbidden_path"` (`facts.go:27`, `executor_tools.go:433`).
   - On query error → **fail open, loudly**: `Warn("nerd.md write protection could not be evaluated …; allowing the write")` and return `("", false)` (`executor_tools.go:434-441`). Rationale: a transient kernel hiccup must not make the agent unusable; the warning makes degraded state visible.
5. Normalise `target` as `ToLower(ToSlash(target))` (`executor_tools.go:443`).
6. Iterate facts: `match := ToLower(ToSlash(ExtractString(fact.Args[0])))`; skip empty or short facts (`executor_tools.go:444-449`); if `Contains(normalized, match)` → return `(ExtractString(fact.Args[1]), true)` (`executor_tools.go:450-451`).
7. Otherwise `("", false)`.

Subtypes handled identically to `ForbidsPath`; the two implementations must stay in sync (see §3.3). Both are case-insensitive and slash-normalised.

### 5.3 Gate table — step-by-step with line refs

| Step | Predicate / check | Code | Location | Denial error | Fail mode |
|---|---|---|---|---|---|
| 1 | JIT allowlist | `if !e.isToolAllowed(call.Name, cfg)` | `executor_tools.go:493` | `tool <name> not allowed by effective JIT config` | Fail **closed** — `isToolAllowed` returns `false` when `cfg == nil` or `AllowedTools` empty (`executor_tools.go:578-585`) |
| 2 | Constitutional Gate | `if e.config.EnableSafetyGate { if !e.checkSafety(call) }` | `executor_tools.go:499-502` | `tool call blocked by safety gate: <name>` | Fail closed |
| **3** | **nerd.md write protection** | `if reason, denied := e.projectForbidsWrite(call); denied` | `executor_tools.go:520-531` | `blocked by nerd.md: <path> is write-protected (<reason>)` | **Fail open on kernel query error** (see §5.2 step 4) |
| 4 | Dreamer preflight | `gate.PreflightDestructiveToolCall(ctx, call.ID, call.Name, call.Args)` behind `InteractiveExecutiveGate` type-assert | `executor_tools.go:539-545` | `tool call blocked by executive gate: …` | Skipped gracefully when `virtualStore` doesn't implement the interface |
| 5 | Modular handler | `tools.Global().Execute` | `executor_tools.go:556-575` | handler error | — |
| 6 | Ouroboros handler | `ouroborosReg.ExecuteRegisteredTool` | `executor_tools.go:588-603` | handler error | — |
| 7 | Post-validation | `gate.ValidateInteractiveToolResult` | `executor_tools.go:565-571` | `post-action validation failed: …` (surfaced so the model can retry) | Skipped when no gate |

Notes:

- `isWriteMutationTool` at `executor_tools.go:320-331`: `write_file`, `edit_file`, `delete_file`, `apply_patch`, `str_replace`, `create_file`, `replace_in_file`, `multi_edit` (case-insensitive, trimmed). This is the same predicate that guards `projectForbidsWrite` step 1, gates `SuccessfulWriteTools` counting (`executor_tools.go:447-449` in `forceFinalAnswer` path and `executor_tools.go:559-561`), and drives `writeOnlyToolDefinitions` for budget-exhausted final calls (`executor_tools.go:307-318`).
- `isToolAllowed` at `executor_tools.go:578-585`: `slices.Contains(cfg.AllowedTools, toolName)`; missing/empty config fails closed.
- After denial at step 3 the denial is `Warn`-logged at `executor_tools.go:522-524` with tool, path, and reason.

### 5.4 Kernel as authority vs cached struct

`SetProjectDoc` (`executor_tools.go:339-347`) stores only the **prose rendering** for prompt injection. Write protection is enforced by querying the kernel, not a cached `*Document` struct, so a subagent that never receives the pointer is still governed — comment at `executor_tools.go:339-346` calls out that a safety gate that depends on a field being wired at every construction site is a gate that is off wherever someone forgot.

### 5.5 Prompt surface (advisory, not enforcement)

`withProjectInstructions(systemPrompt)` at `executor_tools.go:352-368`:

- Reads `e.projectDoc` under `RLock`.
- Calls `doc.PromptSection()` (`facts.go:163-244`).
- When non-empty, appends as `systemPrompt + "\n\n" + section` and logs `Injected <path> instructions into system prompt (<N> chars)`.
- The section restates `project`, canonical commands, `forbid` (explicitly labelled "ENFORCED — denied by the kernel before the tool runs", `facts.go:194-196`), `require`, `conventions`, and then the verbatim body.

The rendered prose exists so the model learns about protection **before** being denied mid-edit and wasting a turn; enforcement remains the kernel's (`executor_tools.go:355-357` and `facts.go:197-200`).

`PromptSection` rendering order (`facts.go:163-244`): header `## Project Instructions (<Path>)` → `**Project**` → `### Canonical commands` (build/test/lint/run + env) → `### Write-protected paths (ENFORCED)` → `### Required steps` → `### Conventions` → verbatim body.

---

## 6. Body, prompt injection, and JIT context

- Body is not a fact, not validated, not structured. It is injected verbatim via `PromptSection` (`facts.go:228-232`).
- `language` additionally seeds the JIT compilation context's `/lang` dimension so language-specific atoms are selected without waiting for a file open (`nerdmd.go:70-72`).
- `commands` are restated in `PromptSection` as `` - `<kind>`: `<command>` `` with `Use these exactly. Do not infer…` (`facts.go:186-188`).
- `forbid` is restated with `- any path containing \`<match>\` — <reason>` plus `These are denied by the kernel before the tool runs, not by your judgement. Attempting one costs a turn and changes nothing.` (`facts.go:192-201`).

---

## 7. What is NOT yet wired

This is the most important section for planning. The codebase is explicit about several dormant paths.

### 7.1 `pending_edit` / transaction-path protection is dormant

`policy/projectdoc.mg:16-35` states it plainly:

- `pending_edit(Path, _)` has **no Go producer today**; "the whole `coder_safety.mg` block family is dormant on that account, so this rule derives nothing yet and is NOT what enforces `nerd.md`" (`policy/projectdoc.mg:28-29`).
- `project_write_denied/2` and its bridge `coder_block_write/2` therefore **derive nothing in production today**. The live enforcement is the imperative gate in `executor_tools.go:520`, not the Mangle derivation.
- The Mangle bridge is written now so that wiring `pending_edit` later turns protection on across the CodeDOM transaction path for free, without rediscovering this file.

**Implication:** Any safety reasoning that queries `project_write_denied` or relies on `coder_block_write` from `coder_safety.mg` to block a CodeDOM edit is currently a no-op. Audit callers of `coder_safe_to_write/1` (or similar) against this.

### 7.2 `require` and `conventions` have no enforcement gate

- `project_requirement/1` and `project_convention/2` are emitted (`facts.go:103-109`) and rendered in the prompt (`facts.go:205-225`), but **no Go code queries them and no policy rule derives an enforcement action** from them. They are informational. A `require` such as "run `go test ./...` before handoff" is not checked by `checkHollowSuccess` or any pre-handoff hook; that work is tracked as future wiring elsewhere.

### 7.3 `commands` and `command_env` have no execution gate

- `project_command/2` and `project_command_env/2` are facts only. Nothing currently forces the agent to use `project_command(/test, …)` instead of an inferred command, nor injects `project_command_env` into tool invocations. The prompt says "Use these exactly. Do not infer" (`facts.go:187`), but the enforcement is still the model.

### 7.4 `has_project_doc`, `project_write_protected`, `project_has_command` are informational markers

- They let policy/prompt branch cheaply (`schemas_projectdoc.mg:58-71`), but nothing currently changes agent capabilities based on them except prompt rendering.

### 7.5 `Document.ForbidsPath` is not the live gate

- `ForbidsPath` (`nerdmd.go:281-293`) is a pure helper with identical matching semantics. Nothing in the current tool-call path calls it; `projectForbidsWrite` queries the kernel instead. Keep the two in sync if either changes, or the spec and the gate will diverge.

### 7.6 No read gate; only write-mutation tools are gated

- The gate checks `isWriteMutationTool` first (`executor_tools.go:421`). Reading, grepping, or listing a forbidden path is intentional — the agent often needs to read it to understand it.

### 7.7 Fail-open on kernel query failure

- On `kernel.Query("project_forbidden_path")` error, the gate **allows** the write and logs a warning (`executor_tools.go:433-441`). This is deliberate (transient failure must not make the agent unusable), but means a degraded kernel silently disables `nerd.md` protection until the query recovers. Monitoring should alert on the `Warn` line `nerd.md write protection could not be evaluated`.

### 7.8 No `MangleAtom` vs string confusion guard beyond `facts.go`

- `project_language` must be an atom; everything else is strings. A new field added with the wrong bound type will silently fail to unify rather than error. `schemas_projectdoc.mg:30` and `facts.go:65` are the only places that discuss this; keep `internal/types/mangle_scale.go`'s `/float64` prohibition in mind if numeric bounds are ever added (`schemas_projectdoc.mg:9-14`).

---

## 8. Cross-references and invariants

| Invariant | Enforced by | Why it matters |
|---|---|---|
| Frontmatter is strict; unknown keys error | `KnownFields(true)` in `Parse` (`nerdmd.go:171`) | Prevents silent-drop of a directive the user believes is in force |
| `schema` must be exactly `nerd/v1` | `validate` (`nerdmd.go:240`) | Prevents half-applying a future contract |
| `forbid` match is substring, case-insensitive, slash-normalised | `ForbidsPath` (`nerdmd.go:287`) and `projectForbidsWrite` (`executor_tools.go:443-450`) | Both layers must agree or the gate has holes |
| `forbid[].reason` required | `validate` (`nerdmd.go:250`) | Denial without explanation looks like malfunction and invites workaround |
| Kernel is the authority for write protection | `projectForbidsWrite` queries `project_forbidden_path` facts (`executor_tools.go:433`) | Avoids a parallel in-memory copy diverging from what policy sees |
| Constitutional > project > Dreamer preflight | Ordering in `executeToolCall` (`executor_tools.go:493-545`) | Constitutional rules outrank project rules; no reason to simulate an already-denied action |

---

## 9. Known test and example coverage (non-exhaustive)

- `internal/session/executor_projectdoc_test.go` — executor + `nerd.md` interaction tests (load, prompt injection, write protection).
- `internal/projectdoc/` — unit tests for `Parse`, `validate`, `Facts`, `PromptSection`, `ForbidsPath` (if present; check package).
- `internal/core/defaults/policy/projectdoc.mg:31-39` contains `Decl project_write_denied` — covered by policy tests that assert it currently derives nothing without `pending_edit`.

---

## 10. Change checklist — what bumping this subsystem requires

- **New frontmatter field** → add to `Spec` (`nerdmd.go:63`), add YAML tag, bump `SchemaVersion` (`nerdmd.go:43`), add `Pred…` constant + `Facts()` emission (`facts.go`), add `Decl` in `schemas_projectdoc.mg`, add `PromptSection` rendering if user-visible, add `validate` check, update this doc.
- **New derived policy** → add to `policy/projectdoc.mg` (never redeclare a `Decl` that already exists in `schemas_projectdoc.mg`).
- **Changing `forbid` semantics** (e.g. glob instead of substring) → update **both** `ForbidsPath` and `projectForbidsWrite` and add a migration note; the two must remain identical.
- **Wiring `pending_edit`** → verify `project_write_denied` and `coder_block_write` start deriving and decide whether the imperative `projectForbidsWrite` gate can be retired or must stay as defence-in-depth.

---

*Generated from source at the line refs above. If any citation has drifted, the source file wins — update this document and note the commit.*
