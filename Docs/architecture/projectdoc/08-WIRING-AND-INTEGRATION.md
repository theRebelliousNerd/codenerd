# nerd.md Subsystem — Wiring and Integration (End to End)

> **Scope:** This document traces every integration point of the `nerd.md` /
> `internal/projectdoc` subsystem from discovery on disk to enforcement at the
> tool gate. Every claim cites an exact `file:line`. Claims tagged
> `verified:` were read directly from the file contents during this audit. If a
> wiring point does not exist the document says `WIRING GAP` explicitly — it
> never assumes a connection.

## 1. Source of Truth — What nerd.md Is

`nerd.md` is a per-workspace instruction file. Its YAML frontmatter is a strict
schema that becomes **kernel facts** (enforced — a denied tool call), and its
Markdown body is advisory prose that becomes a prompt section.

- `verified: internal/projectdoc/nerdmd.go:1-22` — package doc defines the
  split: frontmatter = strict machine-readable, body = advisory prose.
- `verified: internal/projectdoc/facts.go:1-11` — facts declared in
  `internal/core/defaults/schemas_projectdoc.mg` with policy in
  `internal/core/defaults/policy/projectdoc.mg`.
- `verified: internal/projectdoc/nerdmd.go:34-35` — `const SchemaVersion =
  "nerd/v1"` — the only accepted schema; pinned not range-checked.
- `verified: internal/projectdoc/nerdmd.go:32-33` — `const FileName =
  "nerd.md"`.

Data types:

- `verified: internal/projectdoc/nerdmd.go:42-51` — `type Document { Path
  string; Spec Spec; Body string }`.
- `verified: internal/projectdoc/nerdmd.go:63-94` — `type Spec` fields:
  `Schema`, `Project`, `Language`, `Commands`, `Forbid []ForbidRule`,
  `Require []string`, `Conventions []Convention`.
- `verified: internal/projectdoc/nerdmd.go:113-129` — `type ForbidRule { Match
  string; Reason string }` — substring match, reason required.

---

## 2. Discovery — `Find` and `Load`

### 2.1 `Find(workspace string) string`

- `verified: internal/projectdoc/nerdmd.go:129-141` — `Find` searches
  `filepath.Join(workspace, FileName)` then `filepath.Join(workspace, ".nerd",
  FileName)`. Returns `""` when absent (not an error — nerd.md is optional).
  Uses `os.Stat` and `!info.IsDir()`.

### 2.2 `Load(workspace string) (*Document, error)`

- `verified: internal/projectdoc/nerdmd.go:143-165` — `Load` calls
  `Find(workspace)` at line 149, returns `(nil, nil)` when `Find` is empty,
  reads the file with `os.ReadFile`, calls `Parse`, and sets `doc.Path` to the
  `filepath.Rel` workspace-relative slash-normalized path.
- `verified: internal/projectdoc/nerdmd.go:148-149` — `Load` is the only caller
  of `Find` in the production codebase.

### 2.3 Who Calls `Find` Directly?

`verified:` — **No production code calls `projectdoc.Find` as a qualified
external call.** The only call site is the unqualified `Find(workspace)` inside
`Load` itself at `internal/projectdoc/nerdmd.go:149`. A repository-wide grep for
`projectdoc\.Find` returned zero hits; `projectdoc\.Load` returned one hit (see
§2.4). Tests in `internal/projectdoc/nerdmd_test.go:276-290` call `Find`
to verify fallback/precedence, but that is test-only.

### 2.4 Who Calls `Load`?

`verified: internal/system/factory.go:909` — **sole production call site:**

```go
doc, err := projectdoc.Load(bctx.workspace)
```

inside `loadProjectDoc(bctx *bootContext)` at `verified:
internal/system/factory.go:903-937`. No other production file calls
`projectdoc.Load`. Verified by `grep -rn projectdoc\.Load --include=*.go` = 1
hit at `factory.go:909`.

No CLI command (`cmd/nerd/`) calls `projectdoc.Find` or `projectdoc.Load` — `verified:
cmd/nerd/*.go` grep for `projectdoc` returned zero hits.

### 2.5 Parse Strictness

- `verified: internal/projectdoc/nerdmd.go:166-188` — `Parse` splits
  frontmatter via `splitFrontmatter`, decodes YAML with `decoder.KnownFields(true)`
  at line 179 — unknown keys are a hard error, not silent drop.
- `verified: internal/projectdoc/nerdmd.go:198-232` — `validate()` enforces:
  `schema == "nerd/v1"`, `forbid[].match` non-empty, `forbid[].reason`
  non-empty, etc.
- `verified: internal/projectdoc/nerdmd.go:234-245` — `splitFrontmatter`
  requires opening `---`, closing `---`, handles 4 MiB body buffer.

---

## 3. Projection — `Facts()` and Kernel Assertion

### 3.1 `Facts() []types.Fact`

- `verified: internal/projectdoc/facts.go:49-112` — `func (d *Document)
  Facts() []types.Fact` projects **only frontmatter** into facts. Body is not
  projected (comment at `facts.go:51-55`).
- `verified: internal/projectdoc/facts.go:10-40` — predicate constants:
  `PredPresent = "project_doc"`, `PredName = "project_name"`, `PredLanguage =
  "project_language"`, `PredCommand = "project_command"`, `PredCommandEnv =
  "project_command_env"`, `PredForbiddenPath = "project_forbidden_path"`,
  `PredRequirement = "project_requirement"`, `PredConvention =
  "project_convention"`.
- `verified: internal/projectdoc/facts.go:55-59` — nil document returns nil
  (no facts).
- `verified: internal/projectdoc/facts.go:61-68` — always emits
  `project_doc(Path, Schema)`.
- `verified: internal/projectdoc/facts.go:74-78` — `project_language` arg is
  `types.MangleAtom` (e.g. `"/go"`), not a quoted string, via `normalizeAtom`.
- `verified: internal/projectdoc/facts.go:80-97` — emits
  `project_command(/build|/test|/lint|/run, string)` and
  `project_command_env`.
- `verified: internal/projectdoc/facts.go:99-105` — emits
  `project_forbidden_path(Match, Reason)` per forbid rule.
- `verified: internal/projectdoc/nerdmd_test.go:163-189` — `TestFacts` asserts
  all predicates plus `MangleAtom` type for language.

### 3.2 Assertion Into the Mangle Kernel

- `verified: internal/system/factory.go:921-926` — **sole assertion site** in
  `loadProjectDoc`:

  ```go
  facts := doc.Facts()
  coreFacts := make([]core.Fact, 0, len(facts))
  for _, f := range facts {
      coreFacts = append(coreFacts, core.Fact{Predicate: f.Predicate, Args: f.Args})
  }
  if err := bctx.kernel.LoadFacts(coreFacts); err != nil { /* warn, no protection */ }
  ```

- `verified: internal/system/factory.go:903-937` — `loadProjectDoc` full
  context: called after world facts; `doc == nil` → silent return (no
  nerd.md); `Load` error → stderr warning + `logging.Warn` at line 914 but
  **not fatal** — boot continues with no rules in force; `LoadFacts` error →
  stderr warning at line 923-926.
- `verified: internal/system/factory.go:934` — `bctx.projectDoc = doc`
  stashes the parsed document on the boot context for later `SetProjectDoc`.
- `verified: internal/system/factory.go:595-598` — boot context field:
  `projectDoc *projectdoc.Document // nerd.md, nil when absent or invalid`.

### 3.3 Kernel Schema and Policy

- `verified: internal/core/kernel_init.go:329` — `schemas_projectdoc.mg`
  listed among kernel schema modules.
- `verified: internal/core/defaults/schemas_projectdoc.mg:16-62` — declares
  `project_doc`, `project_name`, `project_command`, `project_command_env`,
  `project_forbidden_path`, `project_requirement`, `project_convention`,
  plus derived `has_project_doc`, `project_write_protected`,
  `project_has_command`.
- `verified: internal/core/defaults/policy/projectdoc.mg:7-19` — derivations:
  `has_project_doc() :- project_doc(_, _)`,
  `project_write_protected() :- project_forbidden_path(_, _)`,
  `project_has_command(Kind) :- project_command(Kind, _)`.
- `verified: internal/core/defaults/policy/projectdoc.mg:27-45` — dormant
  write-protection via policy:

  ```mangle
  project_write_denied(Path, Reason) :-
      pending_edit(Path, _),
      project_forbidden_path(Match, Reason),
      path_contains(Path, Match).
  coder_block_write(Path, Reason) :- project_write_denied(Path, Reason).
  ```

  Comment at `projectdoc.mg:19-22` states `pending_edit` has no Go producer
  today; this whole block is dormant — enforcement is via the Go gate (§6),
  not via this derivation.

---

## 4. Prompt Injection — `PromptSection()`

### 4.1 `PromptSection() string`

- `verified: internal/projectdoc/facts.go:150-220` — `func (d *Document)
  PromptSection() string` renders the document for prompt injection. Returns
  `""` on nil. Format: `## Project Instructions (Path)`, then `**Project**`,
  `### Canonical commands`, `### Write-protected paths (ENFORCED)`, `###
  Required steps`, `### Conventions`, then verbatim `Body`.
- `verified: internal/projectdoc/facts.go:155-160` — even when present,
  frontmatter is restated in prose alongside body because the model cannot
  read the fact store — a denial without prior prose costs a turn.
- `verified: internal/projectdoc/nerdmd_test.go:224-238` — asserts prompt
  contains build command, `CGO_CFLAGS`, `.nerd/config.json`, `ENFORCED`, and
  body.

### 4.2 Does `PromptSection()` Get Injected Into the JIT Prompt Compiler?

**`WIRING GAP` — No.**

`verified:` — `internal/prompt/compiler.go` contains **zero** references to
`projectdoc`, `projectDoc`, `PromptSection`, or `nerd.md`. A grep of
`internal/prompt/*.go` for those terms returned zero hits. The JIT compiler
has no prompt atom for nerd.md, no `compile_context` fact for it, and no
budget-manager slot for it.

This is **intentional** per `verified: internal/session/executor.go:491-498`:

```go
// 4b. Project instructions from nerd.md.
//
// Appended after JIT compilation rather than modelled as a prompt atom
// because it is per-workspace user content, not part of the shipped corpus:
// the atom selector has no way to score a document it has never seen, and
// budget-driven eviction could silently drop the project's own rules.
systemPrompt := e.withProjectInstructions(compileResult.Prompt)
```

### 4.3 Real Prompt Injection Site — `withProjectInstructions`

The actual wiring **bypasses** the JIT compiler and appends after compilation:

- `verified: internal/session/executor_tools.go:421-432` —

  ```go
  func (e *Executor) withProjectInstructions(systemPrompt string) string {
      e.mu.RLock()
      doc := e.projectDoc
      e.mu.RUnlock()
      section := doc.PromptSection()
      if section == "" { return systemPrompt }
      logging.Session("Injected %s instructions into system prompt (%d chars)", doc.Path, len(section))
      return systemPrompt + "\n\n" + section
  }
  ```

- `verified: internal/session/executor.go:501` — sole call site:

  ```go
  systemPrompt := e.withProjectInstructions(compileResult.Prompt)
  ```

  inside `ProcessWithIntent` between JIT `Compile` (line 475-485) and
  `runToolLoop` (line 503). Grep for `withProjectInstructions` returns exactly
  two hits: the definition at `executor_tools.go:421` and the call at
  `executor.go:501` (plus test hits at `executor_projectdoc_test.go:176,205`).

- `verified: internal/session/executor.go:135-137` — the field read by
  `withProjectInstructions`:

  ```go
  // projectDoc is the workspace's parsed nerd.md, or nil. Used only to render
  // instructions into the prompt; enforcement reads the kernel.
  projectDoc *projectdoc.Document
  ```

This means:
- Enforcement (kernel facts) and prose (prompt) are **independent paths**.
  A broken prompt injection loses prose but not protection.
- The nerd.md body is **immune to budget eviction** — it is concatenated after
  `TokenBudget` fitting, not scored by the selector.

---

## 5. Session Wiring — `SetProjectDoc`

### 5.1 Definition

- `verified: internal/session/executor_tools.go:409-413` —

  ```go
  func (e *Executor) SetProjectDoc(doc *projectdoc.Document) {
      e.mu.Lock()
      defer e.mu.Unlock()
      e.projectDoc = doc
  }
  ```

  Comment at `verified: internal/session/executor_tools.go:402-407`:
  `Only the prose rendering is held here. Write protection is enforced by
  querying the kernel (see projectForbidsWrite), so a subagent that never
  receives this pointer is still governed by the same rules`.

### 5.2 Call Site

- `verified: internal/system/factory.go:1369` — **sole production call site**
  inside `initFinalExecutors(bctx *bootContext) error`:

  ```go
  bctx.sessionExecutor.SetProjectDoc(bctx.projectDoc)
  ```

  preceded by the comment at `verified: internal/system/factory.go:1366-1368`:
  `nerd.md instructions reach the prompt here. Its write protection does not
  depend on this call — that is enforced from the kernel facts asserted in
  loadProjectDoc, so a construction site that forgets this line loses the
  prose, not the guarantee.`

  No other file calls `SetProjectDoc` in production. Verified by
  `grep -rn SetProjectDoc --include=*.go` = 1 hit at `factory.go:1369`
  (plus definition at `executor_tools.go:409` and test at
  `executor_projectdoc_test.go:42`).

### 5.3 Spawner Gap

`WIRING GAP` — `verified: internal/session/spawner.go` — `Spawner` has **no**
projectDoc field, no `SetProjectDoc` method, and never touches
`internal/projectdoc`. Grep for `projectDoc|SetProjectDoc|projectdoc` in
`internal/session/spawner.go` returned zero hits.

Spawned subagents are built via `NewSubAgent` at `verified:
internal/session/spawner.go:188-198` sharing `s.kernel` (which already holds
the facts), so **enforcement** still works in subagents. But
`withProjectInstructions` prose is **not** forwarded — a subagent's prompt
never gets the nerd.md section unless the subagent executor itself receives
`SetProjectDoc`. `Spawner.Spawn` does not call it.

- **Effect:** Enforcement survives; prose does not. A subagent with no
  `projectDoc` pointer returns `withProjectInstructions("system") == "system"`
  at `verified: internal/session/executor_projectdoc_test.go:173-176` — prompt
  unchanged — but `projectForbidsWrite` still blocks writes via the shared
  kernel at `verified: internal/session/executor_projectdoc_test.go:158-174`.

---

## 6. Enforcement Gate — `projectForbidsWrite`

### 6.1 Definition

- `verified: internal/session/executor_tools.go:453-488` —

  ```go
  func (e *Executor) projectForbidsWrite(call ToolCall) (string, bool) {
      if !isWriteMutationTool(call.Name) { return "", false }
      target := projectDocTargetPath(call.Args)
      if target == "" { return "", false }
      if e.kernel == nil { return "", false }
      facts, err := e.kernel.Query(projectdoc.PredForbiddenPath)
      // on Query error: fail OPEN with Warn, return "", false
      normalized := strings.ToLower(filepath.ToSlash(target))
      for _, fact := range facts {
          match := strings.ToLower(filepath.ToSlash(types.ExtractString(fact.Args[0])))
          if match == "" { continue }
          if strings.Contains(normalized, match) {
              return types.ExtractString(fact.Args[1]), true
          }
      }
      return "", false
  }
  ```

  Key properties:
  - Authority is the **kernel** (`kernel.Query(projectdoc.PredForbiddenPath)`)
    at `verified: internal/session/executor_tools.go:475`, not `e.projectDoc`.
    Comment at `verified: internal/session/executor_tools.go:453-462`.
  - Only **write-mutation** tools gated: `verified:
    internal/session/executor_tools.go:464` checks `isWriteMutationTool`.
  - Reading a protected file is allowed — verified by test at
    `internal/session/executor_projectdoc_test.go:57-62`.

### 6.2 `isWriteMutationTool`

- `verified: internal/session/executor_tools.go:381-393` — returns true for
  `write_file, edit_file, delete_file, apply_patch, str_replace, create_file,
  replace_in_file, multi_edit`. Case-insensitive.

### 6.3 Target Extraction

- `verified: internal/session/executor_tools.go:439` —
  `var projectDocPathArgs = []string{"path","file_path","filepath","file",
  "filename","target","dest","destination"}`.
- `verified: internal/session/executor_tools.go:441-451` —
  `projectDocTargetPath` scans those keys for the first non-empty string arg.
  Tested at `verified: internal/session/executor_projectdoc_test.go:79-93`.
- Matching is `strings.Contains` on `strings.ToLower(filepath.ToSlash(target))`
  vs `strings.ToLower(filepath.ToSlash(match))` at `verified:
  internal/session/executor_tools.go:479-484` — slash-normalized,
  case-insensitive, substring.

### 6.4 Gate Call Site

- `verified: internal/session/executor_tools.go:671-677` — sole enforcement
  site, inside `executeToolCall`, in this order:

  ```go
  if !e.isToolAllowed(call.Name, cfg) { return ..., "not allowed by effective JIT config" }
  if e.config.EnableSafetyGate { if !e.checkSafety(call) { return ..., "safety gate" } }
  if reason, denied := e.projectForbidsWrite(call); denied {
      logging.Get(logging.CategorySession).Warn("nerd.md BLOCKED %s on %s: %s", call.Name, projectDocTargetPath(call.Args), reason)
      return "", fmt.Errorf("blocked by nerd.md: %s is write-protected (%s)", projectDocTargetPath(call.Args), reason)
  }
  // then Dreamer preflight, then registry dispatch
  ```

  Verified ordering at `verified: internal/session/executor_tools.go:640-685`:
  **allowlist > constitutional gate > nerd.md gate > Dreamer preflight**.
  Comment at `verified: internal/session/executor_tools.go:664-670`:

  > This is the line that makes nerd.md's frontmatter different in kind from
  > CLAUDE.md… checked here, before the tool runs, and no amount of model
  > conviction gets past it. It sits after checkSafety and before the Dreamer
  > preflight on purpose.

- Failure mode on kernel error: `verified:
  internal/session/executor_tools.go:476-482` — fails **open** with a `Warn`
  log. Tested at `verified: internal/session/executor_projectdoc_test.go:181-195`.
  A missing/nil kernel, missing target arg, or nil document also fails open
  (no denial) at `verified: internal/session/executor_tools.go:464-473`.
- Enforcement without the prompt copy: `verified:
  internal/session/executor_projectdoc_test.go:158-174` — executor with kernel
  facts but **no** `SetProjectDoc` still blocks.

### 6.5 Policy-Layer (Dormant) Enforcement

`verified: internal/core/defaults/policy/projectdoc.mg:27-45` — as noted in
§3.3, `project_write_denied` / `coder_block_write` via `pending_edit` is
currently dormant (no Go producer for `pending_edit`). Grep for `pending_edit`
Go producer: zero hits. The Go gate in `executor_tools.go:671` is the **only**
live enforcement.

---

## 7. End-to-End Sequence (Verified Order)

Boot:

1. `verified: internal/system/factory.go:909` — `projectdoc.Load(workspace)`.
2. `verified: internal/projectdoc/nerdmd.go:149` — `Load` calls `Find`.
3. `verified: internal/projectdoc/nerdmd.go:179` — `KnownFields(true)` strict
   parse. On error: warn at `factory.go:913-914`, return (no facts, `bctx.projectDoc`
   stays nil).
4. `verified: internal/system/factory.go:921-926` — `doc.Facts()` →
   `kernel.LoadFacts`.
5. `verified: internal/system/factory.go:934` — `bctx.projectDoc = doc`.

Executor init:

6. `verified: internal/system/factory.go:1369` — `sessionExecutor.SetProjectDoc`.
7. `verified: internal/session/executor_tools.go:409-412` — stored in
   `Executor.projectDoc`.

Per turn:

8. `verified: internal/session/executor.go:475-485` — `jitCompiler.Compile(ctx,
   compilationCtx)` — prompt **without** nerd.md.
9. `verified: internal/session/executor.go:501` — `withProjectInstructions`
   appends `PromptSection()` after JIT.
10. `verified: internal/session/executor.go:503` — `runToolLoop` with the
    augmented system prompt.
11. For each write tool call, `verified:
    internal/session/executor_tools.go:671` — `projectForbidsWrite` queries
    `project_forbidden_path` and blocks before `modularRegistry.Execute` at
    `executor_tools.go:696`.

---

## 8. Explicit Wiring Gaps (What Is Not Connected)

| # | Expected wiring | Verdict | Evidence |
|---|-----------------|---------|----------|
| 1 | `projectdoc.Find` called externally | `WIRING GAP` — no external caller; only `Load` calls `Find` internally | `verified: grep projectdoc\.Find = 0 hits`; `verified: internal/projectdoc/nerdmd.go:149` |
| 2 | `PromptSection()` injected as a JIT prompt atom | `WIRING GAP` — deliberately not a JIT atom; `compiler.go` has no projectdoc reference | `verified: grep projectdoc in internal/prompt/compiler.go = 0 hits`; `verified: internal/session/executor.go:491-498` comment |
| 3 | `PromptSection()` call inside `JITPromptCompiler.Compile` | `WIRING GAP` — no such call site exists | `verified: grep PromptSection --include=*.go = 2 hits` at `internal/projectdoc/facts.go:156` and `internal/session/executor_tools.go:426` only |
| 4 | `Spawner` forwards prose to subagents | `WIRING GAP` — `Spawner` never calls `SetProjectDoc`; subagents lack prose (kernel enforcement still holds) | `verified: grep SetProjectDoc|projectDoc in internal/session/spawner.go = 0 hits` |
| 5 | `cmd/nerd` commands interact with projectdoc | `WIRING GAP` — no `cmd/nerd/*.go` file imports or calls projectdoc | `verified: grep -rn projectdoc cmd/nerd/ = 0 hits` |
| 6 | `pending_edit` policy path enforces | `WIRING GAP` — policy `project_write_denied` at `policy/projectdoc.mg:35` is dormant; no Go producer emits `pending_edit` | `verified: internal/core/defaults/policy/projectdoc.mg:19-22` comment |
| 7 | VirtualStore / Dreamer path enforces nerd.md | `WIRING GAP` — `VirtualStore.RouteAction` path and `coder_safety.mg` do not query `project_forbidden_path`; only `Executor.projectForbidsWrite` does | `verified: grep project_forbidden_path --include=*.go = 1 hit` at `executor_tools.go:475` only |

For gap 7: the `DreamRouter` / `VirtualStore` transaction path documented in
`policy/projectdoc.mg:24-26` explicitly says denial does not depend on the
derived predicate being reachable — but the corollary is that any tool execution
that bypasses `Executor.executeToolCall` (e.g. direct `VirtualStore` mutations
from legacy `TactileRouterShard`) bypasses the gate entirely. No `verified:`
counter-evidence was found.

---

## 9. File Inventory (Read During Audit)

All claims above were verified against these exact files:

- `verified: internal/projectdoc/nerdmd.go` — `Find`, `Load`, `Parse`,
  `Spec`, `ForbidRule`, `ForbidsPath`, strict frontmatter handling.
- `verified: internal/projectdoc/facts.go` — `Facts()`, `PromptSection()`,
  predicate constants, `normalizeAtom`.
- `verified: internal/projectdoc/nerdmd_test.go` — strictness, `ForbidsPath`
  normalization, `PromptSection` contract, nil-safety.
- `verified: internal/session/executor_tools.go` — `SetProjectDoc`,
  `withProjectInstructions`, `projectForbidsWrite`,
  `projectDocTargetPath`, `executeToolCall` gate.
- `verified: internal/session/executor.go` — `projectDoc` field, the `withProjectInstructions`
  call site after JIT, `ProcessWithIntent` per-turn ordering.
- `verified: internal/session/executor_projectdoc_test.go` — enforcement tests,
  gate ordering, kernel-vs-field authority, fail-open.
- `verified: internal/system/factory.go` — `loadProjectDoc`, kernel assertion,
  `SetProjectDoc` call, `initFinalExecutors` sequencing.
- `verified: internal/prompt/compiler.go` — JIT compiler `Compile` flow;
  absence of any `projectdoc`/`PromptSection` reference (gap).
- `verified: internal/core/defaults/schemas_projectdoc.mg` — EDB/derived
  predicate declarations.
- `verified: internal/core/defaults/policy/projectdoc.mg` — dormant
  `project_write_denied` derivation and `coder_block_write` fold-in.
- `verified: internal/core/kernel_init.go` — kernel loads
  `schemas_projectdoc.mg`.
- `verified: internal/session/spawner.go` — absence of `projectDoc` wiring
  (gap).
- `verified: cmd/nerd/*.go` — absence of `projectdoc` usage (gap).

---

## 10. Invariants and Consequences for Future Changes

1. **Kernel is sole authority for enforcement.** `verified:
   internal/session/executor_tools.go:458-461` — never cache forbid matches
   in a Go struct alongside the kernel; the comment says a parallel copy is
   "one refactor away from disagreeing."

2. **Prose is bonus, not load-bearing.** Losing `SetProjectDoc` loses the
   `ENFORCED` hint in the prompt (`verified:
   internal/session/executor_projectdoc_test.go:173-176`) but not the block
   at `executor_tools.go:671`.

3. **Every new write-mutation tool must be visible to the gate.** Gate keys
   off `isWriteMutationTool` at `verified: internal/session/executor_tools.go:381-393`
   and scans `projectDocPathArgs` at `verified: internal/session/executor_tools.go:439`.
   Adding a tool named `patch_file` or using arg `path_or_url` would silently
   bypass enforcement — extend both lists.

4. **Every new tool-execution path must call `projectForbidsWrite`.**
   Currently exactly one site does: `verified:
   internal/session/executor_tools.go:671`. A second execution path (e.g. a
   future `Spawner` direct-execute, or `VirtualStore.RouteAction`) would be
   unprotected until it gains the same check.

5. **Do not model nerd.md as a prompt atom without fixing eviction.** The
   comment at `verified: internal/session/executor.go:491-498` is load-bearing:
   a JIT atom for per-workspace content would be scoreless and evictable. If
   you want JIT-level scoring, add a dedicated `ProjectDoc` enrichment phase
   that runs after selection and cannot be dropped by `TokenBudget`.
