# nerd.md Subsystem — Safety and Invariants

> **Scope:** Safety guarantees and invariants of the `nerd.md` / `internal/projectdoc` subsystem
> derived from `internal/projectdoc`, `internal/core/defaults`, and `internal/session`.
> Every claim cites an exact `file:line`. Claims tagged `verified:` were read directly or
> via the prior wiring audit (`08-WIRING-AND-INTEGRATION.md`). Claims that could not be
> re-read this turn due to exhausted exploration budget are tagged `VERIFICATION GAP`.

## 1. Strict Parse Invariant — Unknown Keys Are Hard Errors

The frontmatter parser is intentionally strict. An unknown key fails the load; it is never silently dropped.

- `verified: internal/projectdoc/nerdmd.go:166-188` — `Parse` splits frontmatter via `splitFrontmatter` and decodes with `yaml.Decoder.KnownFields(true)` at line 179.
- `verified: internal/projectdoc/nerdmd.go:1-22` — package doc states frontmatter is strict machine-readable; Markdown body is advisory prose.
- `verified: internal/projectdoc/nerdmd.go:198-232` — `validate()` enforces `schema == "nerd/v1"`, and non-empty `forbid[].match`, `forbid[].reason`, `conventions[].id`, `conventions[].rule`, `require[]` entries.
- `verified: internal/projectdoc/nerdmd.go:34-35` — `const SchemaVersion = "nerd/v1"` pinned; `verified: internal/projectdoc/nerdmd.go:198-215` rejects any other schema with an error that names both versions. This is not range-checked by design (`verified: internal/projectdoc/nerdmd.go:34-35` comment).
- `verified: internal/projectdoc/nerdmd.go:234-245` — `splitFrontmatter` requires opening `---` at line 1 and a closing `---`; buffer is 4 MiB at `verified: internal/projectdoc/nerdmd.go:242` (`scanner.Buffer(..., 4*1024*1024)`).
- `verified: internal/projectdoc/nerdmd.go:234-245` — truncated scan error handling via `scanner.Err()` ensures a body problem is not misreported as a frontmatter problem.

**Invariant:** If a directive survives `Parse`, it is exactly the directive the author wrote. If it would be half-understood, `Parse` returns `error` and `Load` wraps it with the file path at `verified: internal/projectdoc/nerdmd.go:157`.

## 2. Optional-File Invariant — Absence Is Not Error

`nerd.md` is optional. The subsystem is safe when the file is absent.

- `verified: internal/projectdoc/nerdmd.go:129-141` — `Find(workspace string) string` returns `""` when neither `filepath.Join(workspace, "nerd.md")` nor `filepath.Join(workspace, ".nerd", "nerd.md")` exists. Not an error.
- `verified: internal/projectdoc/nerdmd.go:143-165` — `Load` returns `(nil, nil)` when `Find` is empty (lines 148-151). Returns error only when the file exists and is unreadable or invalid at `verified: internal/projectdoc/nerdmd.go:152-159`.
- `verified: internal/projectdoc/facts.go:55-59` — `Facts()` on `nil` document returns `nil` (no facts). Callers can pass `Load` result through without a nil check.
- `verified: internal/projectdoc/facts.go:150-156` — `PromptSection()` on `nil` returns `""`.
- `verified: internal/projectdoc/nerdmd.go:32-33` — `const FileName = "nerd.md"`; only these two locations are searched, no parent-directory walk.

**Invariant:** Deleting `nerd.md` removes all `project_*` facts and all prompt prose, but cannot crash boot or session.

## 3. Frontmatter-Body Separation Invariant

Only frontmatter becomes kernel facts. Body never becomes a fact.

- `verified: internal/projectdoc/facts.go:49-112` — `Facts()` projects only frontmatter. Comment at `verified: internal/projectdoc/facts.go:51-55` states body is prose and belongs in the prompt, not the fact store — asserting free text as a fact would invite policy to pattern-match natural language.
- `verified: internal/projectdoc/nerdmd.go:42-51` — `type Document { Path string; Spec Spec; Body string }` carries both; `verified: internal/projectdoc/nerdmd.go:63-94` enumerates `Spec` fields that are projected.
- `verified: internal/projectdoc/facts.go:150-220` — `PromptSection()` renders frontmatter *again* in prose plus verbatim `Body` because the model cannot read the fact store (`verified: internal/projectdoc/facts.go:155-160` — comment: denial without prior prose costs a turn).

**Invariant:** Enforcement reads facts; the model reads prose. Neither channel leaks into the other.

## 4. Kernel-as-Authority Invariant

Write-protection enforcement authority is the Mangle kernel, not the `*Document` pointer held by the executor.

- `verified: internal/session/executor_tools.go:453-488` — `projectForbidsWrite(call ToolCall) (string, bool)` queries `e.kernel.Query(projectdoc.PredForbiddenPath)` at `verified: internal/session/executor_tools.go:475`, not `e.projectDoc`.
- `verified: internal/session/executor_tools.go:453-462` — comment states enforcement reads the kernel so a subagent that never receives the pointer is still governed.
- `verified: internal/system/factory.go:921-926` — sole assertion site: `doc.Facts()` → `[]core.Fact` → `bctx.kernel.LoadFacts(coreFacts)` at `verified: internal/system/factory.go:921-926`.
- `verified: internal/system/factory.go:903-937` — `loadProjectDoc` stashes `bctx.projectDoc = doc` at line 934 for prompt rendering, but enforcement does not depend on it (`verified: internal/system/factory.go:1366-1368` — comment: forgetting `SetProjectDoc` loses prose, not guarantee).
- `verified: internal/session/executor_tools.go:475` — on `Query` error, `projectForbidsWrite` fails open with `Warn` and returns `("", false)` — it does not deny.

**Invariant:** A missing or stale `e.projectDoc` cannot weaken enforcement. A missing or unreachable kernel fails open to avoid a livelock, but logs.

## 5. Prompt-Immune-to-Eviction Invariant

`nerd.md` prose is appended after JIT compilation and is immune to budget-driven eviction.

- `verified: internal/session/executor.go:491-498` — comment: appended after JIT compilation rather than modelled as a prompt atom because per-workspace user content is not in the shipped corpus and budget eviction could silently drop the project's own rules.
- `verified: internal/session/executor_tools.go:421-432` — `withProjectInstructions(systemPrompt string) string` reads `e.projectDoc` under `RLock` and returns `systemPrompt + "\n\n" + section` when non-empty.
- `verified: internal/session/executor.go:501` — sole call site: `systemPrompt := e.withProjectInstructions(compileResult.Prompt)` between `Compile` (475-485) and `runToolLoop` (503).
- `VERIFICATION GAP: internal/prompt/compiler.go` — prior audit found zero references to `projectdoc`/`PromptSection`/`nerd.md`; JIT selector has no atom or budget slot for nerd.md. Could not re-read this turn to re-confirm line numbers; citing prior audit `08-WIRING-AND-INTEGRATION.md §4.2`.

**Invariant:** Enforcement and prose are independent paths. Broken prompt injection loses prose but not protection; `TokenBudget` fitting cannot evict `nerd.md` instructions.

## 6. Write-Gate Semantics Invariant

The gate is narrow: only write-mutation tools, only substring match on slash-normalized case-insensitive paths.

- `verified: internal/session/executor_tools.go:381-393` — `isWriteMutationTool(name string) bool` is case-insensitive and returns true for exactly: `write_file, edit_file, delete_file, apply_patch, str_replace, create_file, replace_in_file, multi_edit`.
- `verified: internal/session/executor_tools.go:439` — `projectDocPathArgs = []string{"path","file_path","filepath","file","filename","target","dest","destination"}`.
- `verified: internal/session/executor_tools.go:441-451` — `projectDocTargetPath` scans those keys for first non-empty string arg. Returns `""` when no key matches.
- `verified: internal/session/executor_tools.go:453-488` — gate logic: if `!isWriteMutationTool` →`("", false)` at line 464; if `target == ""` → `("", false)` at line 467; if `e.kernel == nil` → `("", false)` at line 472.
- `verified: internal/session/executor_tools.go:479-484` — matching is `strings.Contains(strings.ToLower(filepath.ToSlash(normalized)), strings.ToLower(filepath.ToSlash(match)))` with empty-match skip. Substring, not glob (`verified: internal/projectdoc/nerdmd.go:116-121` — `ForbidRule.Match` comment: substring deliberately, not glob, so Go and Mangle agree).
- `verified: internal/projectdoc/nerdmd.go:249-262` — `Document.ForbidsPath(target string)` mirrors same normalization for local checks.
- `verified: internal/session/executor_tools.go:671-677` — sole enforcement site inside `executeToolCall`, ordered after `isToolAllowed` and `checkSafety`: `if reason, denied := e.projectForbidsWrite(call); denied { return blocked }`.
- Reading a protected path is allowed — `verified: internal/session/executor_projectdoc_test.go:57-62` (test: read of protected file not denied).

**Invariant:** A write whose target `strings.Contains(normalized, match)` denies before the tool runs. A read, a write with no path arg, or a non-mutation tool never denies. Matching cannot be walked through by capitalisation or `\` vs `/`.

## 7. Dormant Mangle Policy Invariant

The pure-Mangle write-protection derivation exists but is dormant; enforcement today is the Go gate.

- `verified: internal/core/defaults/policy/projectdoc.mg:7-19` — derivations: `has_project_doc() :- project_doc(_, _)`, `project_write_protected() :- project_forbidden_path(_, _)`, `project_has_command(Kind) :- project_command(Kind, _)`.
- `verified: internal/core/defaults/policy/projectdoc.mg:27-45` — dormant derivation:
  ```mangle
  project_write_denied(Path, Reason) :- pending_edit(Path, _), project_forbidden_path(Match, Reason), path_contains(Path, Match).
  coder_block_write(Path, Reason) :- project_write_denied(Path, Reason).
  ```
- `verified: internal/core/defaults/policy/projectdoc.mg:19-22` — comment: `pending_edit` has no Go producer today; whole block is dormant.
- `verified: internal/core/defaults/schemas_projectdoc.mg:16-62` — declares `project_doc`, `project_name`, `project_command`, `project_command_env`, `project_forbidden_path`, `project_requirement`, `project_convention`, plus derived `has_project_doc`, `project_write_protected`, `project_has_command`.
- `verified: internal/core/kernel_init.go:329` — `schemas_projectdoc.mg` listed among kernel schema modules.

**Invariant:** No `pending_edit` facts are asserted, so `coder_block_write` never fires. Enforcement does not depend on this derivation remaining dormant; activating it would need a producer and must reuse `strings.Contains` semantics.

## 8. Spawner Separation Invariant (Enforcement Survives, Prose Does Not)

Subagents inherit enforcement via the shared kernel but do not inherit prompt prose.

- `verified: internal/session/executor_tools.go:402-413` — `SetProjectDoc(doc *projectdoc.Document)` comment: only prose rendering is held; write protection is enforced by querying the kernel, so a subagent that never receives this pointer is still governed.
- `verified: internal/system/factory.go:1369` — sole production call site `bctx.sessionExecutor.SetProjectDoc(bctx.projectDoc)` inside `initFinalExecutors`.
- `verified: internal/session/spawner.go:188-198` — `NewSubAgent` shares `s.kernel`; grep for `projectDoc|SetProjectDoc|projectdoc` in `spawner.go` was zero hits (prior audit §5.3).
- `verified: internal/session/executor_projectdoc_test.go:158-176` — subagent with shared kernel but no `projectDoc` pointer: `withProjectInstructions` returns unchanged prompt, but `projectForbidsWrite` still blocks via kernel.

**Invariant:** Charters, every-task handoff, and subagent spawning can lose the prose rendering without losing the safety guarantee. The converse is also true: adding `SetProjectDoc` to the spawner would only affect prose, not enforcement.

## 9. Schema and Predicate Invariants (internal/core/defaults)

- `verified: internal/projectdoc/nerdmd.go:63-94` — `Spec` fields map 1:1 to predicates at `verified: internal/projectdoc/facts.go:10-40`: `project_doc(Path,Schema)`, `project_name(Name)`, `project_language(Lang)` where Lang is `types.MangleAtom` via `normalizeAtom` at `verified: internal/projectdoc/facts.go:74-78`, `project_command(Kind,Command)` with Kind in `/build|/test|/lint|/run` at `verified: internal/projectdoc/facts.go:80-97`, `project_command_env(Name,Value)` at `verified: internal/projectdoc/facts.go:99-101`, `project_forbidden_path(Match,Reason)` at `verified: internal/projectdoc/facts.go:99-105`, `project_requirement(Text)` and `project_convention(ID,Rule)` at `verified: internal/projectdoc/facts.go:107-112`.
- `verified: internal/core/defaults/schemas_projectdoc.mg:16-62` — schema declares those predicates; `verified: internal/core/defaults/policy/projectdoc.mg:7-19` derives the `has_*`/`protected` predicates.
- `verified: internal/projectdoc/nerdmd.go:32-35` — adding a field to `Spec` is a schema change requiring a `SchemaVersion` bump because older binaries reject unknown keys.
- `VERIFICATION GAP: internal/core/defaults/go_safety.mg, schemas_safety.mg, schemas_codedom.mg` — listed at `internal/core/defaults` but could not be read this turn; any safety predicate they contribute (e.g., `deny_edit`, `edit_warning`) is not restated here beyond noting the subsystem does not remove error handling or introduce `unsafe` (covered by those modules when present).

## 10. Boot-Order and Failure-Mode Invariants

- `verified: internal/system/factory.go:903-937` — `loadProjectDoc` is called after world facts; `doc == nil` → silent return (no rules); `Load` error → `logging.Warn` + stderr at line 914 but boot continues with no rules in force; `LoadFacts` error → stderr warning at `verified: internal/system/factory.go:923-926` but boot continues.
- `verified: internal/system/factory.go:595-598` — boot context field `projectDoc *projectdoc.Document` is nil when absent or invalid.
- `verified: internal/system/factory.go:909` — sole production call to `projectdoc.Load`; `verified: internal/projectdoc/nerdmd.go:148-149` — `Load` is sole caller of `Find`.
- `verified: internal/session/executor_tools.go:475` — `projectForbidsWrite` on `Query` error fails open with `Warn`.

**Invariant:** A missing or malformed `nerd.md` never blocks boot; a failed kernel load never blocks boot. The failure mode degrades to open (no rules) with a warning, which is safer than degrading to closed (deny all writes) or to half-applied.

## 11. What Is NOT an Invariant (Documented Gaps)

- `WIRING GAP: internal/prompt/compiler.go` — no `nerd.md` atom; intentional bypass via `withProjectInstructions` (`verified: internal/session/executor.go:491-498`).
- `WIRING GAP: internal/session/spawner.go` — no `projectDoc` forwarding; prose gap documented in §8.
- `WIRING GAP: pending_edit` — Mangle write-protection derivation dormant (`verified: internal/core/defaults/policy/projectdoc.mg:19-22`).
- `VERIFICATION GAP: internal/session/executor*.go` line numbers beyond those re-audited in `08-WIRING-AND-INTEGRATION.md` — ordering at `executor_tools.go:671-677` (`isToolAllowed` → `checkSafety` → `projectForbidsWrite`) was read in prior audit but not re-opened this turn; revisit with a fresh read to re-anchor exact line numbers.
- `VERIFICATION GAP: internal/core/defaults` policy files other than `schemas_projectdoc.mg` and `policy/projectdoc.mg` — not read this turn; safety invariants that depend on `go_safety.mg`/`schemas_safety.mg` deny/warn rules should be added after a direct read.

## 12. Verification Matrix

| Source | File | Lines | Claim |
|--------|------|-------|-------|
| projectdoc | `internal/projectdoc/nerdmd.go` | 1-22 | Strict vs advisory split |
| projectdoc | `internal/projectdoc/nerdmd.go` | 32-33 | `FileName = "nerd.md"` |
| projectdoc | `internal/projectdoc/nerdmd.go` | 34-35 | `SchemaVersion = "nerd/v1"` pinned |
| projectdoc | `internal/projectdoc/nerdmd.go` | 42-51 | `Document` type |
| projectdoc | `internal/projectdoc/nerdmd.go` | 63-94 | `Spec` fields |
| projectdoc | `internal/projectdoc/nerdmd.go` | 113-129 | `ForbidRule` |
| projectdoc | `internal/projectdoc/nerdmd.go` | 129-141 | `Find` two-path search |
| projectdoc | `internal/projectdoc/nerdmd.go` | 143-165 | `Load` semantics |
| projectdoc | `internal/projectdoc/nerdmd.go` | 166-188 | `Parse` KnownFields(true) |
| projectdoc | `internal/projectdoc/nerdmd.go` | 198-232 | `validate()` |
| projectdoc | `internal/projectdoc/nerdmd.go` | 234-245 | `splitFrontmatter` + 4 MiB buffer |
| projectdoc | `internal/projectdoc/nerdmd.go` | 249-262 | `ForbidsPath` normalization |
| projectdoc | `internal/projectdoc/facts.go` | 10-40 | Predicate constants |
| projectdoc | `internal/projectdoc/facts.go` | 49-112 | `Facts()` projection |
| projectdoc | `internal/projectdoc/facts.go` | 55-59 | `Facts()` nil → nil |
| projectdoc | `internal/projectdoc/facts.go` | 74-78 | `MangleAtom` for language |
| projectdoc | `internal/projectdoc/facts.go` | 150-220 | `PromptSection()` |
| projectdoc | `internal/projectdoc/facts.go` | 155-160 | Prose restatement comment |
| defaults | `internal/core/defaults/schemas_projectdoc.mg` | 16-62 | Schema decl |
| defaults | `internal/core/defaults/policy/projectdoc.mg` | 7-19 | Derived predicates |
| defaults | `internal/core/defaults/policy/projectdoc.mg` | 19-45 | Dormant `project_write_denied` |
| defaults | `internal/core/kernel_init.go` | 329 | Schema registered |
| session | `internal/session/executor.go` | 135-137 | `projectDoc` field |
| session | `internal/session/executor.go` | 491-498 | JIT-bypass comment |
| session | `internal/session/executor.go` | 501 | `withProjectInstructions` call site |
| session | `internal/session/executor_tools.go` | 402-413 | `SetProjectDoc` |
| session | `internal/session/executor_tools.go` | 421-432 | `withProjectInstructions` |
| session | `internal/session/executor_tools.go` | 381-393 | `isWriteMutationTool` |
| session | `internal/session/executor_tools.go` | 439-451 | Path arg extraction |
| session | `internal/session/executor_tools.go` | 453-488 | `projectForbidsWrite` |
| session | `internal/session/executor_tools.go` | 671-677 | Enforcement ordering |
| session | `internal/session/spawner.go` | 188-198 | Shared kernel, no doc |
| session | `internal/system/factory.go` | 595-598 | Boot context field |
| session | `internal/system/factory.go` | 903-937 | `loadProjectDoc` |
| session | `internal/system/factory.go` | 921-926 | `LoadFacts` assertion |
| session | `internal/system/factory.go` | 1369 | `SetProjectDoc` call site |
