# 09 — Safety and Invariants: `nerd.md` Subsystem

> **Subsystem:** `internal/projectdoc` (`nerdmd.go`, `facts.go`) + write-gate in `internal/session/executor_tools.go`.
> Every claim cites an exact `file:line` as printed by `read_file` (line number + tab prefix). No estimate.

---

## 1. Strict-Parse Invariant — Unknown Keys Are Hard Errors

Frontmatter is **strict YAML**. An unknown key, bad schema, or malformed entry is a hard `error` naming the problem, never a silently dropped field. The invariant is stated in the package doc at `internal/projectdoc/nerdmd.go:2-22` and in the `Parse` comments at `internal/projectdoc/nerdmd.go:18-21` and `internal/projectdoc/nerdmd.go:178-179`.

* `internal/projectdoc/nerdmd.go:43` — `const SchemaVersion = "nerd/v1"` is the only accepted schema, pinned exactly (comment at `internal/projectdoc/nerdmd.go:38-42`).
* `internal/projectdoc/nerdmd.go:170-189` — `Parse` splits via `splitFrontmatter` (`internal/projectdoc/nerdmd.go:171`) then strict-decodes frontmatter.
* `internal/projectdoc/nerdmd.go:177-182` — `decoder := yaml.NewDecoder(bytes.NewReader(front))` / `decoder.KnownFields(true)` / `decoder.Decode(&spec)`; the comment at `internal/projectdoc/nerdmd.go:178-179` calls this "the whole point".
* `internal/projectdoc/nerdmd.go:185-187` — `spec.validate()` is called immediately after decode.
* `internal/projectdoc/nerdmd.go:236-274` — `validate()`:
  * `internal/projectdoc/nerdmd.go:237-238` — empty/missing `schema` → error naming `schema` and `internal/projectdoc/nerdmd.go:43` expected value.
  * `internal/projectdoc/nerdmd.go:240-244` — `s.Schema != SchemaVersion` → error naming both the supplied and expected version ("Refusing to half-apply").
  * `internal/projectdoc/nerdmd.go:247-249` — `forbid[i].match` empty → error (`"a rule that matches every path would deny every write"`).
  * `internal/projectdoc/nerdmd.go:251-255` — `forbid[i].reason` empty → error (`"a denial the agent cannot explain … invites a workaround"`).
  * `internal/projectdoc/nerdmd.go:258-264` — `conventions[i].id` / `conventions[i].rule` empty → error.
  * `internal/projectdoc/nerdmd.go:267-271` — `require[i]` empty → error.
* `internal/projectdoc/nerdmd.go:192` — `const frontmatterFence = "---"`.
* `internal/projectdoc/nerdmd.go:195-234` — `splitFrontmatter`:
  * `internal/projectdoc/nerdmd.go:196-200` — `bufio.NewScanner` + `scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)` — 4 MiB max token so a body long line (tables, command strings) cannot be truncated and mis-reported as a frontmatter error (comment at `internal/projectdoc/nerdmd.go:197-199`).
  * `internal/projectdoc/nerdmd.go:202-210` — first line must be `---` (`internal/projectdoc/nerdmd.go:205`), else error naming the fence and the got line via `truncate` at `internal/projectdoc/nerdmd.go:294-299`.
  * `internal/projectdoc/nerdmd.go:212-223` — collects until closing `---` (`internal/projectdoc/nerdmd.go:215-217`); `internal/projectdoc/nerdmd.go:222` — unclosed block → error.
  * `internal/projectdoc/nerdmd.go:225-233` — body lines collected after fence; `internal/projectdoc/nerdmd.go:229-230` — `scanner.Err()` checked so a body read error is not misreported.

**Invariant statement:** If `Parse` returns `(doc, nil)`, `doc.Spec` is exactly what the author wrote. Otherwise `Parse` returns an `error` that names the offending line/key/schema. No directive is silently dropped. `Load` wraps that error with the file path at `internal/projectdoc/nerdmd.go:159`.

---

## 2. Optional-File Absence Invariant — Absence Is Not Error

`nerd.md` is optional. The entire subsystem is safe when the file is absent.

* `internal/projectdoc/nerdmd.go:36` — `const FileName = "nerd.md"`.
* `internal/projectdoc/nerdmd.go:131-141` — `Find(workspace string) string` searches exactly two candidates:
  `filepath.Join(workspace, FileName)` at `internal/projectdoc/nerdmd.go:133` and
  `filepath.Join(workspace, ".nerd", FileName)` at `internal/projectdoc/nerdmd.go:134`,
  stat-tested at `internal/projectdoc/nerdmd.go:136`, returns `""` at `internal/projectdoc/nerdmd.go:140` when neither exists — not an error. No parent-directory walk.
* `internal/projectdoc/nerdmd.go:148-167` — `Load(workspace string) (*Document, error)`:
  * `internal/projectdoc/nerdmd.go:149-152` — `if path == "" { return nil, nil }`.
  * `internal/projectdoc/nerdmd.go:153-156` — `os.ReadFile` error wrapped at `internal/projectdoc/nerdmd.go:155` (unreadable file never degrades to "no directive" — comment at `internal/projectdoc/nerdmd.go:145-147`).
  * `internal/projectdoc/nerdmd.go:157-160` — `Parse(data)` error wrapped at `internal/projectdoc/nerdmd.go:159` as `"%s: %w", path`.
  * `internal/projectdoc/nerdmd.go:161-165` — `Path` relativized via `filepath.Rel` at `internal/projectdoc/nerdmd.go:161`.
  * Callers may pass `Load` result straight through without a nil check.
* `internal/projectdoc/facts.go:51-54` — `Facts()`: `if d == nil { return nil }` at `internal/projectdoc/facts.go:52-53`, so no facts are asserted when absent.
* `internal/projectdoc/facts.go:156-158` — `PromptSection()`: `if d == nil { return "" }` at `internal/projectdoc/facts.go:157-158`.
* `internal/projectdoc/nerdmd.go:46-56` — `Document{Path, Spec, Body}` carries both halves for callers that need either.

**Invariant statement:** Deleting `nerd.md` removes all `project_*` facts and all injected prompt prose, but cannot crash boot or session. Missing-file and present-but-invalid are disjoint: `(nil,nil)` vs `(*Document,error)`.

---

## 3. Frontmatter-vs-Body Separation Invariant

Only the YAML frontmatter becomes kernel facts. The Markdown body is prose and belongs in the prompt — never in the fact store.

* `internal/projectdoc/nerdmd.go:46-56` — `Document` holds `Spec Spec` (strict) at `internal/projectdoc/nerdmd.go:51` and `Body string` (verbatim, trimmed) at `internal/projectdoc/nerdmd.go:53-55`.
* `internal/projectdoc/nerdmd.go:58-92` — `Spec` enumerates every field that is projected: `Schema` at `internal/projectdoc/nerdmd.go:65`, `Project` at `internal/projectdoc/nerdmd.go:68`, `Language` at `internal/projectdoc/nerdmd.go:73`, `Commands` at `internal/projectdoc/nerdmd.go:78`, `Forbid` at `internal/projectdoc/nerdmd.go:83`, `Require` at `internal/projectdoc/nerdmd.go:88`, `Conventions` at `internal/projectdoc/nerdmd.go:91`.
* `internal/projectdoc/nerdmd.go:94-105` — `Commands{Build, Test, Lint, Run, Env}` field definitions.
* `internal/projectdoc/nerdmd.go:107-121` — `ForbidRule{Match, Reason}` with substring-not-glob comment at `internal/projectdoc/nerdmd.go:109-114`.
* `internal/projectdoc/facts.go:42-50` — `Facts()` doc comment at `internal/projectdoc/facts.go:42-50`: "Only the frontmatter is projected … asserting free text as a fact would invite policy to pattern-match natural language".
* `internal/projectdoc/facts.go:51-109` — `Facts()` projection:
  * `internal/projectdoc/facts.go:56-58` — always `project_doc(Path, Schema)` via `PredPresent` at `internal/projectdoc/facts.go:15`.
  * `internal/projectdoc/facts.go:60-62` — `project_name(Name)` via `PredName` at `internal/projectdoc/facts.go:18`.
  * `internal/projectdoc/facts.go:64-69` — `project_language(Lang)` via `PredLanguage` at `internal/projectdoc/facts.go:22`; `Lang` is a `types.MangleAtom` (comment at `internal/projectdoc/facts.go:65-67` — a quoted `"go"` would silently never unify), normalized by `normalizeAtom` at `internal/projectdoc/facts.go:129-148`.
  * `internal/projectdoc/facts.go:71-84` — `project_command(Kind, Command)` via `PredCommand` at `internal/projectdoc/facts.go:26` for `/build|/test|/lint|/run`.
  * `internal/projectdoc/facts.go:86-91` — `project_command_env(Name, Value)` via `PredCommandEnv` at `internal/projectdoc/facts.go:30`.
  * `internal/projectdoc/facts.go:93-98` — `project_forbidden_path(Match, Reason)` via `PredForbiddenPath` at `internal/projectdoc/facts.go:33` — the enforced one.
  * `internal/projectdoc/facts.go:100-102` — `project_requirement(Text)` via `PredRequirement` at `internal/projectdoc/facts.go:36`.
  * `internal/projectdoc/facts.go:104-106` — `project_convention(ID, Rule)` via `PredConvention` at `internal/projectdoc/facts.go:39`.
  * `Facts()` returns `nil` for a nil document at `internal/projectdoc/facts.go:52` and never touches `d.Body`.
* `internal/projectdoc/nerdmd.go:189` — `Body: strings.TrimSpace(body)` at `internal/projectdoc/nerdmd.go:189` preserves body verbatim except outer whitespace.
* `internal/projectdoc/facts.go:150-242` — `PromptSection()` is the *only* consumer of `Body` in this package:
  * `internal/projectdoc/facts.go:154-155` — comment: model cannot read the fact store, so frontmatter is restated in prose to avoid a wasted denial turn.
  * `internal/projectdoc/facts.go:200-212` — renders `### Write-protected paths (ENFORCED)` prose from `Forbid`.
  * `internal/projectdoc/facts.go:236-239` — appends trimmed `Body` verbatim.

**Invariant statement:** Enforcement reads `Facts()` (kernel). The model reads `PromptSection()` (prompt). Neither channel leaks into the other: body text never becomes a fact, and a prompt-injection failure cannot weaken enforcement because enforcement does not read the prompt.

---

## 4. Write Gate — What It Guards and How It Matches

The gate is deliberately narrow: only durable write-mutation tools, only substring match on slash-normalized case-insensitive paths. Reads are never gated.

* `internal/session/executor_tools.go:411-424` — `isWriteMutationTool(name string) bool`:
  * Normalization at `internal/session/executor_tools.go:412` — `strings.ToLower(strings.TrimSpace(name))`.
  * Registered VirtualStore write actions at `internal/session/executor_tools.go:413-417` — `write_file`, `edit_file`, `delete_file`, `edit_lines`, `insert_lines`, `delete_lines`, `edit_element`, `fs_write`.
  * Defensive aliases at `internal/session/executor_tools.go:418-419` — `apply_patch`, `str_replace`, `create_file`, `replace_in_file`, `multi_edit` (comment at `internal/session/executor_tools.go:418`).
  * `default: return false` at `internal/session/executor_tools.go:421-423` — any other tool is not a write-mutation.
  * Rationale comment at `internal/session/executor_tools.go:391-410` explains why a missing entry is a hole in a safety gate (observed live: `insert_lines` once missing, hollow-success reported even though a write landed) and why `TestIsWriteMutationTool_CoversEveryDurableWriteAction` pins the list to `internal/core/virtual_store_types.go`.
* `internal/session/executor_tools.go:463` — `var projectDocPathArgs = []string{"path","file_path","filepath","file","filename","target","dest","destination"}` — the gate checks every name a tool might use (comment at `internal/session/executor_tools.go:458-462`).
* `internal/session/executor_tools.go:466-475` — `projectDocTargetPath(args map[string]any) string` scans those keys at `internal/session/executor_tools.go:467`, returns first non-empty `string` at `internal/session/executor_tools.go:469-470`, else `""` at `internal/session/executor_tools.go:474`.
* `internal/session/executor_tools.go:281-292` — `Document.ForbidsPath(target string) (reason string, forbidden bool)` mirrors the same gate for local checks:
  * `internal/session/executor_tools.go:282-284` — nil/empty guard.
  * `internal/session/executor_tools.go:285` — `normalized := strings.ToLower(filepath.ToSlash(target))`.
  * `internal/session/executor_tools.go:287` — `strings.Contains(normalized, strings.ToLower(filepath.ToSlash(rule.Match)))` with case-insensitive slash normalization (comment at `internal/session/executor_tools.go:278-280`).
* `internal/session/executor_tools.go:487-524` — `projectForbidsWrite(call ToolCall) (string, bool)` — the authoritative gate:
  * `internal/session/executor_tools.go:488-490` — non-write-mutation → `("", false)`.
  * `internal/session/executor_tools.go:491-494` — `target := projectDocTargetPath(call.Args)`; empty target → `("", false)` at `internal/session/executor_tools.go:492-493`.
  * `internal/session/executor_tools.go:495-497` — `e.kernel == nil` → `("", false)` at `internal/session/executor_tools.go:496`.
  * `internal/session/executor_tools.go:499` — `facts, err := e.kernel.Query(projectdoc.PredForbiddenPath)` where `PredForbiddenPath = "project_forbidden_path"` is defined at `internal/projectdoc/facts.go:33`. Enforcement authority is the kernel, not `e.projectDoc` (comment at `internal/session/executor_tools.go:477-483` and `internal/session/executor_tools.go:426-432`).
  * `internal/session/executor_tools.go:510-521` — per-fact loop: skip facts with `len(fact.Args) < 2` at `internal/session/executor_tools.go:512-514`; skip empty match at `internal/session/executor_tools.go:515-517`; `strings.Contains(normalized, match)` at `internal/session/executor_tools.go:519` where both sides are `strings.ToLower(filepath.ToSlash(...))` at `internal/session/executor_tools.go:510` and `internal/session/executor_tools.go:515`; on hit returns reason at `internal/session/executor_tools.go:520`.
  * Substring, not glob, deliberately so Go and Mangle semantics are identical (comment at `internal/projectdoc/nerdmd.go:109-114`).

**Invariant statement:** A write whose target path `strings.Contains(slash-normalized lowercased target, slash-normalized lowercased match)` is denied before the tool runs. A read, a write with no path arg, or a non-mutation tool never denies. Capitalisation or `\` vs `/` cannot walk through the gate.

---

## 5. Ordering Against the Allowlist and the Constitutional Gate

Inside `executeToolCall`, `projectForbidsWrite` sits at a fixed position in a total order. The order is not incidental — it is commented at the call sites.

* `internal/session/executor_tools.go:669-712` — `executeToolCall` is the sole enforcement site that sequences all three pre-execution checks.
* `internal/session/executor_tools.go:673-675` — **1. Allowlist** — `if !e.isToolAllowed(call.Name, cfg)` → `tool %s not allowed by effective JIT config` (comment at `internal/session/executor_tools.go:670-672`: "Registry membership only proves that a handler exists; it does not grant the capability").
* `internal/session/executor_tools.go:677-682` — **2. Constitutional gate** — `if e.config.EnableSafetyGate { if !e.checkSafety(call) }` → `tool call blocked by safety gate` at `internal/session/executor_tools.go:680`.
* `internal/session/executor_tools.go:684-700` — **3. Project write protection** — `if reason, denied := e.projectForbidsWrite(call); denied` → `blocked by nerd.md: %s is write-protected (%s)` at `internal/session/executor_tools.go:698-699`. The block comment at `internal/session/executor_tools.go:684-694` states the ordering rationale verbatim: "It sits after checkSafety and before the Dreamer preflight on purpose: constitutional rules outrank project rules, and there is no reason to simulate the consequences of an action that is already denied."
  * Denial is logged at `internal/session/executor_tools.go:696-697` as `nerd.md BLOCKED %s on %s: %s`.
* `internal/session/executor_tools.go:702-712` — **4. Dreamer / executive preflight** — `if gate, ok := e.virtualStore.(InteractiveExecutiveGate); ok && gate != nil { blockErr := gate.PreflightDestructiveToolCall(...) }` at `internal/session/executor_tools.go:707-711` (comment at `internal/session/executor_tools.go:702-706`: "PRE-execution executive gate … before the tool mutates anything").
* `internal/session/executor_tools.go:714-723` onward — timeout + routing to `tools.Global()` or Ouroboros `core.ToolRegistry` only after all gates have passed.

**Invariant statement:** `isToolAllowed` ≺ `checkSafety` ≺ `projectForbidsWrite` ≺ `PreflightDestructiveToolCall` ≺ execution. Constitutional denials outrank project denials. Denied actions are never simulated by the Dreamer — there is no reason to simulate what is already refused.

---

## 6. Fail-Open-on-Kernel-Error Decision

When the kernel cannot be queried, the gate **fails open** (allows the write) and warns, rather than failing closed (denying every write).

* `internal/session/executor_tools.go:495-497` — `e.kernel == nil` → `("", false)` (`internal/session/executor_tools.go:496`) — a subagent or early-boot path with no kernel cannot enforce, so it degrades to no rules rather than to "deny all writes".
* `internal/session/executor_tools.go:499-507` — `e.kernel.Query(projectdoc.PredForbiddenPath)` error branch:
  * `internal/session/executor_tools.go:501-504` — comment: "Fail OPEN, loudly. A kernel query failure is not evidence that the path is protected, and turning every transient query error into a blocked write would make the agent unusable the moment the kernel hiccups."
  * `internal/session/executor_tools.go:505-507` — `logging.Get(logging.CategorySession).Warn("nerd.md write protection could not be evaluated for %s (%v); allowing the write", target, err)` and `return "", false` — allows the write and makes the degraded state visible.

**Invariant statement:** A transient kernel hiccup does not become a livelock where every write is blocked. Visibility is via `Warn`, not via a silent allow or a silent deny. The opposite choice (fail-closed) was deliberately rejected because it would make the agent unusable.

---

## 7. Kernel-as-Authority Invariant (Why `e.projectDoc` Is Not the Gate)

Write-protection authority is the Mangle fact store, not the `*Document` pointer cached on the executor.

* `internal/session/executor_tools.go:426-437` — `SetProjectDoc(doc *projectdoc.Document)` comment at `internal/session/executor_tools.go:426-432`: "Only the prose rendering is held here. Write protection is enforced by querying the kernel … so a subagent that never receives this pointer is still governed".
* `internal/session/executor_tools.go:433-437` — `SetProjectDoc` stores under `e.mu` at `internal/session/executor_tools.go:434-436`.
* `internal/session/executor_tools.go:445-456` — `withProjectInstructions` reads the same pointer at `internal/session/executor_tools.go:446-447` to append `PromptSection()` at `internal/session/executor_tools.go:450` — prose only. The frontmatter is restated in prose even though it is already in the kernel (comment at `internal/session/executor_tools.go:442-444`): "learning that a path is protected by being denied mid-edit costs a whole turn".
* `internal/session/executor_tools.go:477-483` — `projectForbidsWrite` doc comment: "The kernel is the authority, not a cached Go struct: nerd.md facts are asserted at boot like any other EDB, so policy, /query, and this gate all see exactly the same rules."
* `internal/session/executor_tools.go:499` — query is `e.kernel.Query(projectdoc.PredForbiddenPath)` at `internal/session/executor_tools.go:499`, not `e.projectDoc.Spec.Forbid`.
* `internal/projectdoc/facts.go:51-109` — every `Spec` field is projected into a distinct predicate; enforcement never pattern-matches `Body`.

**Invariant statement:** A missing or stale `e.projectDoc` weakens only prompt prose, never enforcement. A shared kernel with no per-executor pointer still enforces correctly. The prose path can be lost (e.g., in a subagent that never received `SetProjectDoc`) without losing the safety guarantee.

---

## 8. Supporting Invariants

### 8.1 Language atom normalization

* `internal/projectdoc/facts.go:64-69` — `project_language` is asserted as a `types.MangleAtom`, not a string, because atoms and strings are disjoint in Mangle and a quoted `"go"` would silently never unify (comment at `internal/projectdoc/facts.go:65-68`).
* `internal/projectdoc/facts.go:129-148` — `normalizeAtom` at `internal/projectdoc/facts.go:129`: `TrimSpace`, strip leading `/` at `internal/projectdoc/facts.go:130`, `ToLower` at `internal/projectdoc/facts.go:130`, drop non-name chars at `internal/projectdoc/facts.go:137-143` (only `a-z`, `0-9`, `_` survive; `- .` and space become `_` at `internal/projectdoc/facts.go:140-141`), return `""` at `internal/projectdoc/facts.go:144-145` when only `/` survived — caller emits no fact rather than an unparseable one.

### 8.2 Predicate constants

All predicates are constants at `internal/projectdoc/facts.go:12-40` so Go, Mangle schemas, and the gate query agree on spelling: `project_doc` at `internal/projectdoc/facts.go:15`, `project_name` at `internal/projectdoc/facts.go:18`, `project_language` at `internal/projectdoc/facts.go:22`, `project_command` at `internal/projectdoc/facts.go:26`, `project_command_env` at `internal/projectdoc/facts.go:30`, `project_forbidden_path` at `internal/projectdoc/facts.go:33`, `project_requirement` at `internal/projectdoc/facts.go:36`, `project_convention` at `internal/projectdoc/facts.go:39`.

### 8.3 `ForbidsPath` local mirror

* `internal/projectdoc/nerdmd.go:281-292` — `ForbidsPath` at `internal/projectdoc/nerdmd.go:281` is the non-kernel mirror (for tests/local checks). It uses the identical normalization as the gate (`strings.ToLower(filepath.ToSlash(...))` at `internal/projectdoc/nerdmd.go:285` and `internal/projectdoc/nerdmd.go:287`) and the same `strings.Contains` match.

---

## 9. Verification Matrix

| Claim | File | Exact line |
|-------|------|-----------|
| `FileName = "nerd.md"` | `internal/projectdoc/nerdmd.go` | `36` |
| `SchemaVersion = "nerd/v1"` (pinned) | `internal/projectdoc/nerdmd.go` | `43` |
| Package doc: strict frontmatter vs advisory body | `internal/projectdoc/nerdmd.go` | `2-22` |
| `Spec` is strict schema (adding field = schema bump) | `internal/projectdoc/nerdmd.go` | `58-62` (comment), `43` (version) |
| `Document{Path, Spec, Body}` | `internal/projectdoc/nerdmd.go` | `46-56` |
| `ForbidRule{Match, Reason}` — substring, not glob | `internal/projectdoc/nerdmd.go` | `107-121` (comment `109-114`) |
| `Find` searches 2 locations, `""` when absent | `internal/projectdoc/nerdmd.go` | `131-141` |
| `Load` returns `(nil,nil)` when absent | `internal/projectdoc/nerdmd.go` | `149-152` |
| `Load` wraps read/parse errors with path | `internal/projectdoc/nerdmd.go` | `155`, `159` |
| `Parse` uses `KnownFields(true)` | `internal/projectdoc/nerdmd.go` | `180` |
| `Parse` calls `validate()` | `internal/projectdoc/nerdmd.go` | `185` |
| `splitFrontmatter` 4 MiB buffer | `internal/projectdoc/nerdmd.go` | `200` |
| `splitFrontmatter` requires opening `---` | `internal/projectdoc/nerdmd.go` | `205` |
| `splitFrontmatter` requires closing `---` | `internal/projectdoc/nerdmd.go` | `221-223` |
| `splitFrontmatter` checks `scanner.Err()` | `internal/projectdoc/nerdmd.go` | `229-230` |
| `validate(): missing/empty schema` | `internal/projectdoc/nerdmd.go` | `237-238` |
| `validate(): unsupported schema` | `internal/projectdoc/nerdmd.go` | `240-244` |
| `validate(): forbid[].match empty` | `internal/projectdoc/nerdmd.go` | `247-249` |
| `validate(): forbid[].reason empty` | `internal/projectdoc/nerdmd.go` | `251-255` |
| `validate(): conventions / require empty` | `internal/projectdoc/nerdmd.go` | `258-271` |
| `ForbidsPath` normalization + `Contains` | `internal/projectdoc/nerdmd.go` | `285`, `287` |
| `PredForbiddenPath = "project_forbidden_path"` | `internal/projectdoc/facts.go` | `33` |
| `Facts()` nil-document → `nil` | `internal/projectdoc/facts.go` | `52-53` |
| `Facts()` projects `project_doc` | `internal/projectdoc/facts.go` | `56-58` |
| `Facts()` projects `project_forbidden_path` | `internal/projectdoc/facts.go` | `93-98` |
| `Facts()` comment: body not a fact | `internal/projectdoc/facts.go` | `42-50` |
| `normalizeAtom` definition | `internal/projectdoc/facts.go` | `129-148` |
| `PromptSection()` nil → `""` | `internal/projectdoc/facts.go` | `156-158` |
| `PromptSection()` renders `Body` verbatim (trimmed) | `internal/projectdoc/facts.go` | `236-239` |
| `isWriteMutationTool` list (case-insensitive) | `internal/session/executor_tools.go` | `411-424` |
| `projectDocPathArgs` — all path arg names | `internal/session/executor_tools.go` | `463` |
| `projectDocTargetPath` scans all keys | `internal/session/executor_tools.go` | `466-475` |
| `projectForbidsWrite`: non-write → allow | `internal/session/executor_tools.go` | `488-490` |
| `projectForbidsWrite`: empty target → allow | `internal/session/executor_tools.go` | `491-494` |
| `projectForbidsWrite`: `kernel == nil` → allow | `internal/session/executor_tools.go` | `495-497` |
| `projectForbidsWrite`: `kernel.Query(project_forbidden_path)` | `internal/session/executor_tools.go` | `499` |
| `projectForbidsWrite`: fail open with Warn | `internal/session/executor_tools.go` | `501-507` |
| `projectForbidsWrite`: `Contains` match (normalized) | `internal/session/executor_tools.go` | `510`, `515`, `519` |
| `executeToolCall`: allowlist (`isToolAllowed`) | `internal/session/executor_tools.go` | `673-675` |
| `executeToolCall`: constitutional gate (`checkSafety`) | `internal/session/executor_tools.go` | `678-682` |
| `executeToolCall`: project gate (`projectForbidsWrite`) + ordering comment | `internal/session/executor_tools.go` | `684-700` (comment `692-694`, gate `695`) |
| `executeToolCall`: Dreamer preflight | `internal/session/executor_tools.go` | `707-712` |

