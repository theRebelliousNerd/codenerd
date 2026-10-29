# 06 — Public API and Types: `internal/projectdoc` Exported Surface

> **Corpus type:** Verified — complete exported surface of `internal/projectdoc` (`nerd.md` subsystem). This document is a line-anchored inventory; every exported symbol cites its definition site as `verified: <file:line>`. Claims beyond `internal/projectdoc/nerdmd.go` and `internal/projectdoc/facts.go` are marked `ASSUMPTION`.
> **Package:** `projectdoc` (`verified: internal/projectdoc/nerdmd.go:22`, `verified: internal/projectdoc/facts.go:1`)
> **Source files read:** `internal/projectdoc/nerdmd.go:1-299`, `internal/projectdoc/facts.go:1-256` (`verified: internal/projectdoc/nerdmd.go:1-299`, `verified: internal/projectdoc/facts.go:1-256`)
> **Last synthesized:** 2026-05-14 (re-read 2026-05-14). Validated against `internal/projectdoc/nerdmd.go` and `internal/projectdoc/facts.go` where cited as `verified:`; claims beyond those files marked `ASSUMPTION`.
> **Uncertainty note:** This document covers only exported symbols in the two source files above. Behaviour, callers, kernel wiring, and Mangle declarations outside `internal/projectdoc/nerdmd.go` / `internal/projectdoc/facts.go` are `ASSUMPTION` unless explicitly re-read.

## 1. Inventory summary

`internal/projectdoc` exports exactly 10 constants, 5 types, 19 struct fields, and 7 functions/methods across its two source files. No other exported symbols exist in those files (`verified: internal/projectdoc/nerdmd.go:1-299`, `verified: internal/projectdoc/facts.go:1-256`).

| Category | Count | File |
|---|---|---|
| Exported constants | 2 + 8 = 10 | `nerdmd.go:2`, `facts.go:8` — see §2 |
| Exported types | 5 | `nerdmd.go:5` — see §3 |
| Exported struct fields | 19 | `nerdmd.go:19` — see §3 |
| Exported functions / methods | 7 | `nerdmd.go:3`, `facts.go:3` — see §4 |

Unexported helpers are not part of the public surface and are listed only for exclusion in §5 (`verified: internal/projectdoc/nerdmd.go:192`, `verified: internal/projectdoc/nerdmd.go:195`, `verified: internal/projectdoc/nerdmd.go:236`, `verified: internal/projectdoc/nerdmd.go:294`, `verified: internal/projectdoc/facts.go:129`).

## 2. Exported constants

All constants are untyped strings (`verified: internal/projectdoc/nerdmd.go:36`, `verified: internal/projectdoc/nerdmd.go:43`, `verified: internal/projectdoc/facts.go:12-39`).

### 2.1 `nerdmd.go` — file identity and schema pin

| Symbol | Value | Definition site | Comment / semantics |
|---|---|---|---|
| `FileName` | `"nerd.md"` | `verified: internal/projectdoc/nerdmd.go:36` | `// FileName is the canonical project instruction file.` (`verified: internal/projectdoc/nerdmd.go:35`) |
| `SchemaVersion` | `"nerd/v1"` | `verified: internal/projectdoc/nerdmd.go:43` | `// SchemaVersion is the only frontmatter schema this build accepts.` Pinned, not range-checked, so newer documents fail loudly rather than half-applied (`verified: internal/projectdoc/nerdmd.go:38-43`) |

```go
const FileName = "nerd.md"      // verified: internal/projectdoc/nerdmd.go:36
const SchemaVersion = "nerd/v1" // verified: internal/projectdoc/nerdmd.go:43
```

### 2.2 `facts.go` — Mangle predicate names

Declared in a single const block (`verified: internal/projectdoc/facts.go:12`). Each constant is a predicate name string. Comments name the predicate arity/shape (`verified: internal/projectdoc/facts.go:13-39`).

| Symbol | Value | Definition site | Predicate shape (from comment) |
|---|---|---|---|
| `PredPresent` | `"project_doc"` | `verified: internal/projectdoc/facts.go:15` | `project_doc(Path, Schema)` (`verified: internal/projectdoc/facts.go:14`) |
| `PredName` | `"project_name"` | `verified: internal/projectdoc/facts.go:18` | `project_name(Name)` (`verified: internal/projectdoc/facts.go:17`) |
| `PredLanguage` | `"project_language"` | `verified: internal/projectdoc/facts.go:22` | `project_language(Lang)` where Lang is a Mangle atom `/go`, `/python` (`verified: internal/projectdoc/facts.go:20-21`) |
| `PredCommand` | `"project_command"` | `verified: internal/projectdoc/facts.go:26` | `project_command(Kind, Command)` where Kind is `/build`, `/test`, `/lint`, `/run` (`verified: internal/projectdoc/facts.go:24-25`) |
| `PredCommandEnv` | `"project_command_env"` | `verified: internal/projectdoc/facts.go:30` | `project_command_env(Name, Value)` (`verified: internal/projectdoc/facts.go:28-29`) |
| `PredForbiddenPath` | `"project_forbidden_path"` | `verified: internal/projectdoc/facts.go:33` | `project_forbidden_path(Match, Reason)` — the ENFORCED one (`verified: internal/projectdoc/facts.go:32`) |
| `PredRequirement` | `"project_requirement"` | `verified: internal/projectdoc/facts.go:36` | `project_requirement(Text)` (`verified: internal/projectdoc/facts.go:35`) |
| `PredConvention` | `"project_convention"` | `verified: internal/projectdoc/facts.go:39` | `project_convention(ID, Rule)` (`verified: internal/projectdoc/facts.go:38`) |

```go
const (
    PredPresent       = "project_doc"            // verified: internal/projectdoc/facts.go:15
    PredName          = "project_name"           // verified: internal/projectdoc/facts.go:18
    PredLanguage      = "project_language"       // verified: internal/projectdoc/facts.go:22
    PredCommand       = "project_command"        // verified: internal/projectdoc/facts.go:26
    PredCommandEnv    = "project_command_env"    // verified: internal/projectdoc/facts.go:30
    PredForbiddenPath = "project_forbidden_path" // verified: internal/projectdoc/facts.go:33
    PredRequirement   = "project_requirement"    // verified: internal/projectdoc/facts.go:36
    PredConvention    = "project_convention"     // verified: internal/projectdoc/facts.go:39
)
```

Mangle declaration site for these predicates is `internal/core/defaults/schemas_projectdoc.mg` (`ASSUMPTION` — not re-read this turn; see `01-VISION.md` and `02-CURRENT-STATE.md` for that mapping).

## 3. Exported types and struct fields

### 3.1 `Document` — a parsed `nerd.md` (`verified: internal/projectdoc/nerdmd.go:45-56`)

```go
// Document is a parsed nerd.md. // verified: internal/projectdoc/nerdmd.go:45
type Document struct {            // verified: internal/projectdoc/nerdmd.go:46
    Path string // verified: internal/projectdoc/nerdmd.go:48
    Spec Spec   // verified: internal/projectdoc/nerdmd.go:51
    Body string // verified: internal/projectdoc/nerdmd.go:55
}
```

| Field | Type | Definition site | YAML tag | Comment / semantics |
|---|---|---|---|---|
| `Path` | `string` | `verified: internal/projectdoc/nerdmd.go:48` | n/a | File this was read from, relative to workspace when known; set by `Load` via `filepath.Rel` + `ToSlash` (`verified: internal/projectdoc/nerdmd.go:47-48`) |
| `Spec` | `Spec` | `verified: internal/projectdoc/nerdmd.go:51` | n/a | Strict machine-readable frontmatter (`verified: internal/projectdoc/nerdmd.go:50-51`) |
| `Body` | `string` | `verified: internal/projectdoc/nerdmd.go:55` | n/a | Markdown prose after frontmatter, verbatim and trimmed; advisory only (`verified: internal/projectdoc/nerdmd.go:53-55`) |

### 3.2 `Spec` — strict frontmatter schema (`verified: internal/projectdoc/nerdmd.go:58-92`)

```go
// Spec is the strict frontmatter schema.                                    // verified: internal/projectdoc/nerdmd.go:58
// Every field is optional except Schema. Adding a field here is a schema     // verified: internal/projectdoc/nerdmd.go:60
// change: bump SchemaVersion, because older binaries reject unknown keys by   // verified: internal/projectdoc/nerdmd.go:61
// design.                                                                    // verified: internal/projectdoc/nerdmd.go:62
type Spec struct { // verified: internal/projectdoc/nerdmd.go:63
    Schema      string       `yaml:"schema"`               // verified: internal/projectdoc/nerdmd.go:65
    Project     string       `yaml:"project,omitempty"`    // verified: internal/projectdoc/nerdmd.go:68
    Language    string       `yaml:"language,omitempty"`   // verified: internal/projectdoc/nerdmd.go:73
    Commands    Commands     `yaml:"commands,omitempty"`   // verified: internal/projectdoc/nerdmd.go:78
    Forbid      []ForbidRule `yaml:"forbid,omitempty"`     // verified: internal/projectdoc/nerdmd.go:83
    Require     []string     `yaml:"require,omitempty"`    // verified: internal/projectdoc/nerdmd.go:88
    Conventions []Convention `yaml:"conventions,omitempty"`// verified: internal/projectdoc/nerdmd.go:91
}
```

| Field | Type | YAML tag | Definition site | Required | Semantics |
|---|---|---|---|---|---|
| `Schema` | `string` | `yaml:"schema"` | `verified: internal/projectdoc/nerdmd.go:65` | **Yes** | Pins frontmatter contract; must equal `SchemaVersion` (`verified: internal/projectdoc/nerdmd.go:64`) |
| `Project` | `string` | `yaml:"project,omitempty"` | `verified: internal/projectdoc/nerdmd.go:68` | No | Human-readable project name, surfaced in prompts (`verified: internal/projectdoc/nerdmd.go:67`) |
| `Language` | `string` | `yaml:"language,omitempty"` | `verified: internal/projectdoc/nerdmd.go:73` | No | Primary language tag e.g. `"go"`, `"python"`; seeds JIT `/lang` dimension (`verified: internal/projectdoc/nerdmd.go:70-72`) |
| `Commands` | `Commands` | `yaml:"commands,omitempty"` | `verified: internal/projectdoc/nerdmd.go:78` | No | Canonical build/test/lint/run invocations; without these the agent guesses (`verified: internal/projectdoc/nerdmd.go:75-77`) |
| `Forbid` | `[]ForbidRule` | `yaml:"forbid,omitempty"` | `verified: internal/projectdoc/nerdmd.go:83` | No | Paths the agent must not write to; ENFORCED → `project_forbidden_path` facts, executor refuses before tool runs (`verified: internal/projectdoc/nerdmd.go:80-82`) |
| `Require` | `[]string` | `yaml:"require,omitempty"` | `verified: internal/projectdoc/nerdmd.go:88` | No | Non-negotiable steps; surfaced to model and as `project_requirement` facts (`verified: internal/projectdoc/nerdmd.go:85-87`) |
| `Conventions` | `[]Convention` | `yaml:"conventions,omitempty"` | `verified: internal/projectdoc/nerdmd.go:91` | No | Named, checkable project rules (`verified: internal/projectdoc/nerdmd.go:90`) |

Unknown YAML keys are a hard error via `decoder.KnownFields(true)` (`verified: internal/projectdoc/nerdmd.go:180`).

### 3.3 `Commands` — canonical shell invocations (`verified: internal/projectdoc/nerdmd.go:94-105`)

```go
// Commands holds the project's canonical shell invocations. // verified: internal/projectdoc/nerdmd.go:94
type Commands struct { // verified: internal/projectdoc/nerdmd.go:95
    Build string            `yaml:"build,omitempty"` // verified: internal/projectdoc/nerdmd.go:96
    Test  string            `yaml:"test,omitempty"`  // verified: internal/projectdoc/nerdmd.go:97
    Lint  string            `yaml:"lint,omitempty"`  // verified: internal/projectdoc/nerdmd.go:98
    Run   string            `yaml:"run,omitempty"`   // verified: internal/projectdoc/nerdmd.go:99
    Env   map[string]string `yaml:"env,omitempty"`   // verified: internal/projectdoc/nerdmd.go:104
}
```

| Field | Type | YAML tag | Definition site | Semantics |
|---|---|---|---|---|
| `Build` | `string` | `yaml:"build,omitempty"` | `verified: internal/projectdoc/nerdmd.go:96` | Canonical build invocation |
| `Test` | `string` | `yaml:"test,omitempty"` | `verified: internal/projectdoc/nerdmd.go:97` | Canonical test invocation |
| `Lint` | `string` | `yaml:"lint,omitempty"` | `verified: internal/projectdoc/nerdmd.go:98` | Canonical lint invocation |
| `Run` | `string` | `yaml:"run,omitempty"` | `verified: internal/projectdoc/nerdmd.go:99` | Canonical run invocation |
| `Env` | `map[string]string` | `yaml:"env,omitempty"` | `verified: internal/projectdoc/nerdmd.go:104` | Environment variables required for the commands above; exists because `CGO_CFLAGS`-style prerequisites are invisible in the command string (`verified: internal/projectdoc/nerdmd.go:100-103`) |

### 3.4 `ForbidRule` — denied write target (`verified: internal/projectdoc/nerdmd.go:107-121`)

```go
// ForbidRule denies writes to paths containing Match. // verified: internal/projectdoc/nerdmd.go:107
type ForbidRule struct { // verified: internal/projectdoc/nerdmd.go:108
    Match  string `yaml:"match"`  // verified: internal/projectdoc/nerdmd.go:115
    Reason string `yaml:"reason"` // verified: internal/projectdoc/nerdmd.go:120
}
```

| Field | Type | YAML tag | Definition site | Required | Semantics |
|---|---|---|---|---|---|
| `Match` | `string` | `yaml:"match"` | `verified: internal/projectdoc/nerdmd.go:115` | **Yes** | Substring matched against slash-normalised target path; deliberately substring, not glob, so Go and Mangle agree (`verified: internal/projectdoc/nerdmd.go:109-114`) |
| `Reason` | `string` | `yaml:"reason"` | `verified: internal/projectdoc/nerdmd.go:120` | **Yes** | Shown to model and user when write is denied; required because unexplained denial reads as malfunction (`verified: internal/projectdoc/nerdmd.go:117-119`) |

### 3.5 `Convention` — named project rule (`verified: internal/projectdoc/nerdmd.go:123-127`)

```go
// Convention is a named project rule. // verified: internal/projectdoc/nerdmd.go:123
type Convention struct { // verified: internal/projectdoc/nerdmd.go:124
    ID   string `yaml:"id"`   // verified: internal/projectdoc/nerdmd.go:125
    Rule string `yaml:"rule"` // verified: internal/projectdoc/nerdmd.go:126
}
```

| Field | Type | YAML tag | Definition site | Required |
|---|---|---|---|---|
| `ID` | `string` | `yaml:"id"` | `verified: internal/projectdoc/nerdmd.go:125` | **Yes** |
| `Rule` | `string` | `yaml:"rule"` | `verified: internal/projectdoc/nerdmd.go:126` | **Yes** |

## 4. Exported functions and methods

Signatures are reproduced verbatim; each line number is the `func` keyword line.

### 4.1 `nerdmd.go` — discovery, loading, parsing, enforcement helper

| Signature | Definition site | Behaviour |
|---|---|---|
| `func Find(workspace string) string` | `verified: internal/projectdoc/nerdmd.go:131` | Locates `nerd.md` searching `<workspace>/nerd.md` then `<workspace>/.nerd/nerd.md`; returns `""` when absent (not an error; `nerd.md` is optional). Uses `os.Stat` + `!IsDir` (`verified: internal/projectdoc/nerdmd.go:129-141`) |
| `func Load(workspace string) (*Document, error)` | `verified: internal/projectdoc/nerdmd.go:148` | Reads and parses `nerd.md` from a workspace. Returns `(nil, nil)` when file does not exist; returns error only when file exists and is invalid — unreadable directive must never degrade to "no directive" (`verified: internal/projectdoc/nerdmd.go:143-167`). On success sets `doc.Path` to slash-normalised `filepath.Rel(workspace, path)` with absolute fallback (`verified: internal/projectdoc/nerdmd.go:161-165`) |
| `func Parse(data []byte) (*Document, error)` | `verified: internal/projectdoc/nerdmd.go:170` | Splits frontmatter from body and strictly decodes frontmatter (`verified: internal/projectdoc/nerdmd.go:169`). Uses `splitFrontmatter` then `yaml.NewDecoder` with `KnownFields(true)` then `spec.validate()` then `&Document{Spec: spec, Body: strings.TrimSpace(body)}` (`verified: internal/projectdoc/nerdmd.go:171-189`) |
| `func (d *Document) ForbidsPath(target string) (reason string, forbidden bool)` | `verified: internal/projectdoc/nerdmd.go:281` | Reports whether any forbid rule covers `target`, returning the reason. Matching is slash-normalised and case-insensitive via `strings.Contains(strings.ToLower(filepath.ToSlash(...)))` because a case-sensitive gate is not a gate (`verified: internal/projectdoc/nerdmd.go:276-292`). Nil/empty-target safe (`verified: internal/projectdoc/nerdmd.go:282-283`) |

Full verbatim signatures for copy-paste:

```go
func Find(workspace string) string                                         // verified: internal/projectdoc/nerdmd.go:131
func Load(workspace string) (*Document, error)                              // verified: internal/projectdoc/nerdmd.go:148
func Parse(data []byte) (*Document, error)                                 // verified: internal/projectdoc/nerdmd.go:170
func (d *Document) ForbidsPath(target string) (reason string, forbidden bool) // verified: internal/projectdoc/nerdmd.go:281
```

### 4.2 `facts.go` — fact projection, prompt rendering, helpers

| Signature | Definition site | Behaviour |
|---|---|---|
| `func (d *Document) Facts() []types.Fact` | `verified: internal/projectdoc/facts.go:51` | Projects frontmatter into kernel facts only; Markdown body is never projected — asserting free text as a fact would invite natural-language pattern matching (`verified: internal/projectdoc/facts.go:42-50`). Returns `nil` for nil document so `Load` result passes through without nil check (`verified: internal/projectdoc/facts.go:52-54`). Always emits `project_doc(Path, Schema)` (`verified: internal/projectdoc/facts.go:56-58`) then conditional `project_name`, `project_language` (as `types.MangleAtom` via `normalizeAtom`), `project_command` ×0-4, `project_command_env` ×N, `project_forbidden_path` ×N, `project_requirement` ×N, `project_convention` ×N (`verified: internal/projectdoc/facts.go:60-107`) |
| `func (d *Document) CommandCount() int` | `verified: internal/projectdoc/facts.go:112` | Reports how many canonical commands the document declares; nil-safe returns 0 (`verified: internal/projectdoc/facts.go:112-124`). Counts non-empty `Build`/`Test`/`Lint`/`Run` after `TrimSpace` (`verified: internal/projectdoc/facts.go:117-122`) |
| `func (d *Document) PromptSection() string` | `verified: internal/projectdoc/facts.go:156` | Renders document for prompt injection — restates frontmatter in prose alongside body so model is not surprised by subsequent hard denial (`verified: internal/projectdoc/facts.go:158-169`). Returns `""` for nil (`verified: internal/projectdoc/facts.go:157`). Emits header `## Project Instructions (Path)`, then conditional sections for project name, canonical commands (+ env), write-protected paths (ENFORCED), required steps, conventions, then verbatim trimmed body (`verified: internal/projectdoc/facts.go:161-248`) |

Full verbatim signatures:

```go
func (d *Document) Facts() []types.Fact  // verified: internal/projectdoc/facts.go:51
func (d *Document) CommandCount() int    // verified: internal/projectdoc/facts.go:112
func (d *Document) PromptSection() string // verified: internal/projectdoc/facts.go:156
```

## 5. Explicitly not exported (excluded from public surface)

These appear in the source but are unexported and therefore not part of the caller-facing API (`verified: internal/projectdoc/nerdmd.go:192`, `verified: internal/projectdoc/nerdmd.go:195`, `verified: internal/projectdoc/nerdmd.go:236`, `verified: internal/projectdoc/nerdmd.go:294`, `verified: internal/projectdoc/facts.go:129`). Documented here to prevent misuse of assumed exports.

| Symbol | Kind | Definition site | Note |
|---|---|---|---|
| `frontmatterFence` | `const string = "---"` | `verified: internal/projectdoc/nerdmd.go:192` | Fence literal for frontmatter block |
| `splitFrontmatter` | `func splitFrontmatter(data []byte) (front []byte, body string, err error)` | `verified: internal/projectdoc/nerdmd.go:195` | Extracts leading `---`-fenced YAML block; scanner buffer grown to 4 MiB (`verified: internal/projectdoc/nerdmd.go:196-199`) |
| `(s *Spec) validate` | `func (s *Spec) validate() error` | `verified: internal/projectdoc/nerdmd.go:236` | Semantic validation after strict YAML decode; called by `Parse` (`verified: internal/projectdoc/nerdmd.go:185`) |
| `truncate` | `func truncate(s string, n int) string` | `verified: internal/projectdoc/nerdmd.go:294` | Helper for error messages |
| `normalizeAtom` | `func normalizeAtom(raw string) string` | `verified: internal/projectdoc/facts.go:129` | Converts `Language` tag to Mangle atom `/name`; lowercase, slash-prefixed, non-name chars dropped, `"- ."` → `"_"` (`verified: internal/projectdoc/facts.go:129-152`) |

No other file in `internal/projectdoc/` contributes exported symbols (`ASSUMPTION` — `internal/projectdoc/nerdmd_test.go` not read; test file exports are not part of the runtime surface).

## 6. Verification notes and open assumptions

* Every table row in §2-§4 cites a `verified: <file:line>` anchored to the `func`/`type`/`const` keyword line as found by `grep -n` and `cat -n` this turn (`verified: internal/projectdoc/nerdmd.go:36`, `verified: internal/projectdoc/nerdmd.go:43`, `verified: internal/projectdoc/facts.go:12`, etc.).
* `PromptSection()` injection point into `internal/prompt/compiler.go` and consumption of `project_forbidden_path` / `project_write_denied` in `internal/session/executor_tools.go` and `internal/core/defaults/policy/projectdoc.mg` are `ASSUMPTION` — those files were not re-read this turn; see `01-VISION.md` `ASSUMPTION` notes and `02-CURRENT-STATE.md` §7-§8 for previously verified wiring.
* `types.Fact` shape (`Predicate string`, `Args []any`) and `types.MangleAtom` distinction are `ASSUMPTION` — defined in `internal/types` not re-read; their use as `types.MangleAtom(kind)` for `project_language` / `project_command` is `verified: internal/projectdoc/facts.go:68`, `verified: internal/projectdoc/facts.go:82`.
* Exhaustiveness claim ("no other exported symbols") is `verified: internal/projectdoc/nerdmd.go:1-299` and `verified: internal/projectdoc/facts.go:1-256` by full-file read; extending to other packages or future schema bumps is `ASSUMPTION`.

## 7. Risks if surface drifts

* Adding a field to `Spec` without bumping `SchemaVersion` silently breaks `KnownFields(true)` strictness thesis (`verified: internal/projectdoc/nerdmd.go:60-62`, `verified: internal/projectdoc/nerdmd.go:43`, `verified: internal/projectdoc/nerdmd.go:180`). Older binary would reject unknown key; newer field would be lost.
* Changing `ForbidRule.Match` from substring to glob/regex without updating both `ForbidsPath` (`verified: internal/projectdoc/nerdmd.go:281-292`) and Mangle predicate semantics creates a gate that sometimes opens (`ASSUMPTION` for Mangle side; see `01-VISION.md` §3).
* Representing `PredLanguage` / `PredCommand` Kind as `string` instead of `types.MangleAtom` would silently never unify in Mangle because atoms and strings are disjoint (`verified: internal/projectdoc/facts.go:65-67`).
