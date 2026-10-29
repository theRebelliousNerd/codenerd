# pending_edit — Policy References & Expected FilePath/Content Shapes

> **Date:** 2026-05-13 / 2026-08-08 / 2026-09-01 (evidence window)  
> **Question:** List policy files referencing `pending_edit` and record expected `FilePath`/`Content` shapes.  
> **Method:** grep (`pending_edit`, `pending_mutation`, `Decl`) + direct `read_file` of policy workspace + schemas (`schemas_shards.mg`, `schemas_safety.mg`, `schemas_coder.mg`) + `MANGLE_FACT_APIS_INVENTORY.md` + `WRITE_MUTATION_INVENTORY.md` (Researcher Shard, encyclopedic mode).  
> **Confidence:** High for `pending_edit` Decl shape and policy file list (grep 20+ hits + 7 direct file reads verified); High for `pending_mutation` analog shape (`transaction_manager.go:186` read verified); Low for filesystem-level completeness due to earlier grep-engine staleness for Go files (see §7).

---

## 1. Verdict: `pending_edit` IS Referenced in Policy — Central 2-Arg EDB Predicate

**Initial grep failure corrected:** An earlier grep run via the `grep` tool returned **0 matches** for `pending_edit` / `pending` / `FilePath` — proven stale by subsequent `search_code` and `grep` with `path` scoping. The authoritative result is the **path-scoped grep** (§3) plus 7 `read_file` verifications.

**No Go producer exists today** — comment in `projectdoc.mg:27` states explicitly: `pending_edit has no Go producer today (the whole coder_safety.mg block family is dormant on that account)`. This matches `MANGLE_FACT_APIS_INVENTORY.md:12` which noted `pending_edit` had no Go assertion at time of that inventory, with `pending_mutation` as the only asserted analog. The predicate is **policy-declared EDB consumed by rules**, but **not yet asserted by `TransactionManager` or `VirtualStore`** — wiring it will activate dormant safety blocks.

---

## 2. Canonical Decl — Expected FilePath/Content Shapes

### 2.1 Single Source of Truth: `schemas_shards.mg:131-132` (read verified)

```mangle
# SECTION 30: CODER SHARD HELPERS — internal/core/defaults/schemas_shards.mg:131-132
Decl pending_edit(FilePath, Content) bound [/string, /string].
```

| Arg | Name | Mangle Type | Go Type (when wired) | Shape / Constraint |
|---|---|---|---|---|
| 0 | `FilePath` | `string` (`/string`) | `string` | Repo-relative or absolute path after `VirtualStore.resolvePath`; examples in policy: `coder_impact.mg:14` uses `File`, `coder_safety.mg:21` uses `Path`; no truncation; `path_contains(Path, Match)` checks substring |
| 1 | `Content` | `string` (`/string`) | `string` | File content preview or full content (policy leaves as opaque string; Go analog truncates 200 chars — see §2.2); `_` wildcard used in all 20+ rule bodies indicating second arg is currently unused beyond existence check |

**Bound annotation:** `[/string, /string]` — both args are `string`. No `/number`, no `/name` atom.

**Corpus confirmation:** `predicate_corpus.db` entry (binary grep hit):
```
pending_edit  2  EDB  shard  pending_edit(FilePath, Content) - pending edits  schemas_shards.mg
```
Two args, EDB, shard scope, declared in `schemas_shards.mg`.

### 2.2 Analog: `pending_mutation` — 4-Arg Sibling with Go Producer (`transaction_manager.go:186-191` + `schemas_safety.mg:183-184`)

```mangle
# schemas_safety.mg:183-184 (read verified)
Decl pending_mutation(MutationID, FilePath, OldContent, NewContent) bound [/string, /string, /string, /string].

# transaction_manager.go:186-191 (read verified)
// AddEdit asserts truncated preview (200 chars + "..."), full content used for os.WriteFile
mutationID := fmt.Sprintf("%s_edit_%d", txn.ID, len(txn.Edits)-1) // txn.ID = txn_<UnixNano>
tm.kernel.Assert(Fact{
    Predicate: "pending_mutation",
    Args:      []any{mutationID, edit.FilePath, oldContentTrunc, newContentTrunc},
})
```

| Property | `pending_edit` (2-arg) | `pending_mutation` (4-arg) | Evidence |
|---|---|---|---|
| Arity | 2 | 4 | `schemas_shards.mg:132` vs `schemas_safety.mg:184` |
| Arg0 | FilePath `string` | MutationID `string` (`txn_<nanos>_edit_<idx>`) | `transaction_manager.go:170` |
| Arg1 | Content `string` | FilePath `string` | same |
| Arg2 | — | OldContent `string` (200-char trunc) | `transaction_manager.go:174-178` |
| Arg3 | — | NewContent `string` (200-char trunc) | `transaction_manager.go:181-185` |
| Producer | **None yet** (`projectdoc.mg:27` comment verified) | `TransactionManager.AddEdit` + `Commit` emits `file_written` | `transaction_manager.go:186`, `397` |

**If wiring `pending_edit` to Go:** Recommend either (a) keep 2-arg shape `pending_edit(FilePath, ContentPreview)` for minimal policy drift, or (b) mirror 4-arg `pending_mutation` preview pattern with truncation helper `trunc200` (`len > 200 ? s[:200]+"..." : s`). Do **not** invent a 5-arg hash/status variant without a schema migration — no such Decl exists.

### 2.3 FilePath — Expected Shape Details (from VirtualStore + TransactionManager context)

| Property | Expected Value | Evidence |
|---|---|---|
| Type | `string` | `schemas_shards.mg:132` `/string` |
| Form | Relative repo path e.g. `"internal/core/foo.go"` or absolute via `resolvePath`; `..` traversal blocked (`virtual_store_actions.go:60` contains check, analogous for files via `ValidatorRegistry`) | `virtual_store_file_actions.go:193` `resolvePath(req.Target)` + `WRITE_MUTATION_INVENTORY.md:28` |
| Normalization | `path.Clean`, no `\`, no `../`, no leading `/` (illustrative from `policy_inventory.go:108-115` `IsDefaultPolicyFile`) | `policy_inventory.go:108-115` |
| Empty allowed? | No — handlers require path; `handleWriteFile` requires `payload["content"] string` and valid `req.Target` | `virtual_store_file_actions.go:195-198` |
| Max length | No cap observed; `pending_mutation` does not truncate FilePath | `transaction_manager.go:186-191` |
| Examples | `"src/app/main.py"`, `"internal/core/kernel_facts.go"`, `".nerd/mangle/policy_overrides.mg"` | Derived |

### 2.4 Content — Expected Shape Details

| Property | Expected Value | Evidence |
|---|---|---|
| Type | `string` (Mangle double-quoted, Go `string`) | `schemas_shards.mg:132` |
| Full vs truncated | Policy treats as opaque; Go analog stores **200-char preview + "..."** in EDB, full `edit.Content` for `os.WriteFile` | `transaction_manager.go:174-185` |
| Encoding | UTF-8 text; `extractCodeBlockForFile` strips ``` fences before write | `WRITE_MUTATION_INVENTORY.md:29` + `virtual_store_file_actions.go:202-206` |
| Hash | `sha256` of full content for `file_written(path, hash, sessionID, timestamp)` — not stored in `pending_edit` Content | `virtual_store_file_actions.go:232-233` |
| Empty | Allowed for create/delete edge cases; `_` wildcard in policy means empty check not enforced | `coder_tdd.mg:52-53` `edit_is_implementation` vs `edit_is_test` |
| Line-range variants | Code DOM paths (`edit_lines`, `insert_lines`, `delete_lines`) carry `start_line`/`end_line`/`after_line` as `float64` + `content string` fragment, not whole-file content | `virtual_store_codedom.go:325-457` |
| Wildcard usage | All 20+ rule bodies use `pending_edit(File, _)` — second arg ignored today; wiring full preview will not break callers | Grep list §3 |

---

## 3. Policy Files Referencing `pending_edit` — Complete Inventory (grep `path=internal/core/defaults` + reads verified)

**Total: 7 files, 20+ rule bodies.** All references are **body atoms `pending_edit(File|Path, _)`** with wildcard second arg; no Decl in policy files themselves (Decl lives only in `schemas_shards.mg`).

| # | File (canonical `internal/core/defaults/policy/...`) | Lines (grep hit) | Verified via `read_file`? | Role / Section | Example Rule Head |
|---|---|---|---|---|---|
| 1 | `policy/coder_impact.mg` | **33, 39, 43, 48** (4 hits) | Yes — full file read (68 lines) | §4 Impact Analysis — classification | `high_impact_edit(File) :- pending_edit(File, _), dependent_count(File, N), N > 5.` (`:33`) |
| 2 | `policy/coder_observability.mg` | **26** (1 hit) | Yes — full file read (58 lines) | §13 Observability — blocked reason | `coder_blocked_reason(File, Reason) :- coder_block_action(/edit, Reason), pending_edit(File, _).` (`:26`) |
| 3 | `policy/coder_quality.mg` | **14, 21, 29, 37** (4 hits) | Yes — full file read (228 lines) | §10 Quality Gates — Go/context/goroutine/interface | `go_needs_error_check(File) :- pending_edit(File, _), detected_language(File, /go), ...` (`:14`) |
| 4 | `policy/coder_safety.mg` | **14, 21, 26, 31, 36, 47, 75, 79** (8 hits) | Yes — full file read (106 lines) | §5 Safety & Blocking — central gate file | `coder_block_write(File, "uncovered_impact") :- pending_edit(File, _), dependency_link(Dependent, File, _), ...` (`:14`) |
| 5 | `policy/coder_tdd.mg` | **52, 57** (2 hits) | Yes — full file read (68 lines) | §9 TDD Integration — impl/test classification | `edit_is_implementation(File) :- pending_edit(File, _), !is_test_file(File).` (`:52`) |
| 6 | `policy/coder_workflow.mg` | **31, 37, 78, 249** (4 hits) | Yes — full file read (292 lines) | §7 Next Action + §13.1 blocked helpers | `next_coder_action(/apply_edit) :- coder_state(/code_generated), pending_edit(File, _), coder_safe_to_write(File).` (`:31`) |
| 7 | `policy/projectdoc.mg` | **34** (1 hit, plus 2 comment lines 27,30) | Yes — full file read (42 lines) | nerd.md write protection (§11) | `project_write_denied(Path, Reason) :- pending_edit(Path, _), project_forbidden_path(Match, Reason), path_contains(Path, Match).` (`:34`) |

**Grepping artifact — raw hits (path-scoped `grep pending` + `grep pending_edit|pending_mutation`):**
```
internal/core/defaults/policy/coder_impact.mg:33: pending_edit(File, _),
internal/core/defaults/policy/coder_impact.mg:39: pending_edit(File, _),
internal/core/defaults/policy/coder_impact.mg:43: pending_edit(File, _),
internal/core/defaults/policy/coder_impact.mg:48: pending_edit(File, _),
internal/core/defaults/policy/coder_observability.mg:26: pending_edit(File, _).
internal/core/defaults/policy/coder_quality.mg:14: pending_edit(File, _),
internal/core/defaults/policy/coder_quality.mg:21: pending_edit(File, _),
internal/core/defaults/policy/coder_quality.mg:29: pending_edit(File, _),
internal/core/defaults/policy/coder_quality.mg:37: pending_edit(File, _),
internal/core/defaults/policy/coder_safety.mg:14: pending_edit(File, _),
internal/core/defaults/policy/coder_safety.mg:21: pending_edit(Path, _),
internal/core/defaults/policy/coder_safety.mg:26: pending_edit(Path, _),
internal/core/defaults/policy/coder_safety.mg:31: pending_edit(Path, _),
internal/core/defaults/policy/coder_safety.mg:36: pending_edit(Path, _),
internal/core/defaults/policy/coder_safety.mg:47: pending_edit(_, _),
internal/core/defaults/policy/coder_safety.mg:75: pending_edit(File, _).
internal/core/defaults/policy/coder_safety.mg:79: pending_edit(File, _),
internal/core/defaults/policy/coder_tdd.mg:52: pending_edit(File, _),
internal/core/defaults/policy/coder_tdd.mg:57: pending_edit(File, _),
internal/core/defaults/policy/coder_workflow.mg:31: pending_edit(File, _),
internal/core/defaults/policy/coder_workflow.mg:37: pending_edit(File, _),
internal/core/defaults/policy/coder_workflow.mg:78: pending_edit(File, _),
internal/core/defaults/policy/coder_workflow.mg:249: pending_edit(File, _),
internal/core/defaults/policy/projectdoc.mg:27: # A pending edit that a project rule forbids. pending_edit has no Go producer
internal/core/defaults/policy/projectdoc.mg:30: # now so that wiring pending_edit later turns nerd.md protection on across the
internal/core/defaults/policy/projectdoc.mg:34: pending_edit(Path, _),
```

**Files NOT referencing `pending_edit` (spot-checked via read_file):** `.nerd/mangle/policy_overrides.mg` (10 lines, no hit), `internal/mcp/policy_mcp.mg` (178 lines, no hit), `internal/core/defaults/policy/commit_gate.mg` (63 lines — uses `pending_mutation` not `pending_edit`), `policy/shadow_mode.mg` (29 lines — uses `pending_mutation`), `policy/campaign_*.mg` (only `/pending` status atoms), `internal/core/defaults/*.mg` root modules (only `/pending` workflow atoms). Full `**/*.mg` glob listed ~80 files; path-scoped grep over `internal/core/defaults/policy` is the complete surface for coder policy.

---

## 4. Schema Neighbors & Lifecycle Context

### 4.1 Where `pending_edit` lives in the schema graph

- **Decl:** `schemas_shards.mg:131-132` — Section 30 Coder Shard Helpers (EDB).
- **Siblings in same file (Section 30):** `file_content(FilePath, Content)`, `coder_state(State)`, `coder_block_write(FilePath, Reason)`, `coder_safe_to_write(FilePath)`, `is_binary_file(FilePath)`, `is_core_file(FilePath)`, `dependent_count(Target, Count)`, `path_contains(Path, Pattern)`, `is_test_file(FilePath)` — all used alongside `pending_edit` in safety/impact rules.
- **Sibling schema:** `schemas_safety.mg:183-184` Decl `pending_mutation(MutationID, FilePath, OldContent, NewContent)` — transactional counterpart asserted today.
- **Campaign/workflow siblings:** `campaign_task(TaskID, PhaseID, _, /pending, _)` and `pending_test`/`pending_review` — unrelated status atoms, not file-edit predicates.

### 4.2 Consumer rules — what fires when `pending_edit` is asserted (all 20+ heads)

| Derived Head | File | Trigger Pattern | Effect when `pending_edit` appears |
|---|---|---|---|
| `high_impact_edit(File)` | `coder_impact.mg:33` | `pending_edit(File, _), dependent_count >5` | Impact warning → may block or require review |
| `critical_impact_edit(File)` | `coder_impact.mg:39,43` | `pending_edit(File, _), is_core_file/is_interface_file` | `impact_warning(File, "critical_file")` |
| `cross_package_impact(File)` | `coder_impact.mg:48` | `pending_edit(File, _), file_package, dependency_link, coder_impacted` | `impact_warning(File, "cross_package")` |
| `go_needs_error_check` etc. | `coder_quality.mg:14,21,29,37` | `pending_edit(File, _), detected_language(/go)` | Quality gate recommendations |
| `coder_block_write(File, Reason)` | `coder_safety.mg:14` | `pending_edit(File,_), dependency_link, coder_impacted, !test_coverage` | Blocks write — currently dormant (no fact to match) |
| `coder_block_action(/edit, Reason)` | `coder_safety.mg:21,26,31,36,47` | `pending_edit(Path,_), !path_in_workspace / is_binary / is_generated / is_vendor / tdd_red_phase` | Blocks edit action |
| `has_coder_block / coder_safe_to_write` | `coder_safety.mg:70-80` | Aggregation over `coder_block_*` + `pending_edit` | Safe-to-write gate |
| `edit_is_implementation / edit_is_test` | `coder_tdd.mg:52,57` | `pending_edit(File,_), is_test_file` | TDD phase checks |
| `next_coder_action(/apply_edit)` | `coder_workflow.mg:31` | `pending_edit(File,_), coder_safe_to_write` | Workflow progression |
| `next_coder_action(/request_review)` etc. | `coder_workflow.mg:37,78,249` | `pending_edit + high/critical impact` | Review/rollback |
| `project_write_denied(Path, Reason)` | `projectdoc.mg:34` | `pending_edit(Path,_), project_forbidden_path, path_contains` | nerd.md protection — folds into `coder_block_write` |
| `coder_blocked_reason(File, Reason)` | `coder_observability.mg:26` | `pending_edit(File,_), has_edit_block` | Diagnostics |

### 4.3 Expected Go wiring (proposed, modeled on `pending_mutation` — NOT yet in codebase)

```go
// Proposed — mirrors transaction_manager.go:186 pattern, 2-arg shape
func trunc200(s string) string {
    if len(s) > 200 { return s[:200] + "..." }
    return s
}
mutationContent := trunc200(newContent) // or full content if policy needs whole file
kernel.Assert(types.Fact{Predicate: "pending_edit", Args: []any{filePath, mutationContent}})
// Batch variant
kernel.AssertBatch([]Fact{{Predicate:"pending_edit", Args:[]any{path1, c1}}, {Predicate:"pending_edit", Args:[]any{path2, c2}}})

// Atomic swap on commit — pending_edit ↔ file_written in one KernelTransaction (per MANGLE_FACT_APIS_INVENTORY.md:274)
tx := kernel.Transaction() // or types.NewKernelTx(kernel)
tx.RetractFact(types.Fact{Predicate: "pending_edit", Args: []any{filePath}}) // first-arg match
tx.Assert(types.Fact{Predicate: "file_written", Args: []any{filePath, newHash, txnID, timestamp}})
if err := tx.Commit(); err != nil { /* handle */ }

// Control-packet path (LLM mangle_updates)
facts, blocked := core.FilterMangleUpdates(kernel, envelope.Control.MangleUpdates, policy)
if len(facts) > 0 { _ = kernel.AssertBatch(facts) }
```

**Truncation note:** `pending_mutation` truncates Old/New to 200 chars; `pending_edit` second arg should follow same convention if wired via `TransactionManager.AddEdit`, or store full content if policy needs to inspect it (no Decl constraint enforces length).

---

## 5. How to Query / Retract `pending_edit`

From `MANGLE_FACT_APIS_INVENTORY.md` + `kernel_facts.go:750,858`:

| Operation | Go API | Semantics for `pending_edit` |
|---|---|---|
| Assert | `kernel.Assert(Fact{Predicate:"pending_edit", Args:[]any{path, content}})` | Single file; `sanitizeFactForNumericPredicates` + dedup via `canonFact` |
| Batch | `kernel.AssertBatch(facts)` | N files, single dirty flag, single `rebuild()` |
| Query | `kernel.Query("pending_edit")` | Returns `[]Fact` with `[FilePath, Content]` pairs |
| Retract all | `kernel.Retract("pending_edit")` | Removes **all** `pending_edit` facts (parallel filter) |
| Retract one file | `kernel.RetractFact(Fact{Predicate:"pending_edit", Args:[]any{filePath}})` | First-arg match — predicate + Args[0] equality |
| Retract exact | `kernel.RetractExactFact(Fact{Predicate:"pending_edit", Args:[]any{path, content}})` | Predicate + all args equality |
| Tx atomic | `tx := kernel.Transaction(); tx.RetractFact(...); tx.Assert(...); tx.Commit()` | Single lock, single rebuild — for `pending_edit` ↔ `file_written` swap |

---

## 6. Sources & Anchors (every fact cited)

- `internal/core/defaults/schemas_shards.mg:131-132` — Decl `pending_edit(FilePath, Content) bound [/string, /string]` (read_file verified).
- `internal/core/defaults/schemas_safety.mg:183-184` — Decl `pending_mutation(MutationID, FilePath, OldContent, NewContent) bound [/string, /string, /string, /string]` (read_file verified).
- `internal/core/defaults/schemas_coder.mg` — No `pending_edit` Decl (read_file verified, 161 lines).
- `internal/core/defaults/policy/coder_impact.mg:33,39,43,48` — 4 rule bodies (read_file verified).
- `internal/core/defaults/policy/coder_observability.mg:26` — 1 rule body (read_file verified).
- `internal/core/defaults/policy/coder_quality.mg:14,21,29,37` — 4 rule bodies (read_file verified).
- `internal/core/defaults/policy/coder_safety.mg:14,21,26,31,36,47,75,79` — 8 rule bodies (read_file verified).
- `internal/core/defaults/policy/coder_tdd.mg:52,57` — 2 rule bodies (read_file verified).
- `internal/core/defaults/policy/coder_workflow.mg:31,37,78,249` — 4 rule bodies (read_file verified).
- `internal/core/defaults/policy/projectdoc.mg:27,30,34` — comment + 1 rule body (read_file verified); comment confirms no Go producer.
- `internal/core/defaults/policy/commit_gate.mg:43,47,59` — `pending_mutation` (3 hits, read_file verified, 63 lines).
- `internal/core/defaults/policy/shadow_mode.mg:28` — `pending_mutation` (1 hit, read_file verified).
- `internal/core/transaction_manager.go:186-191` — `pending_mutation` assertion with truncation + `mutationID` format (read_file verified).
- `internal/core/transaction_manager.go:397-400` — `file_written(path, newHash, txnID, timestamp)` counterpart (read_file verified).
- `internal/core/virtual_store_file_actions.go:186,248,314` — `handleWriteFile/EditFile/DeleteFile` handlers with payload shapes `content`/`old`/`new`/`confirmed` and Facts `file_written`/`file_edited`/`file_deleted` (read_file verified).
- `internal/core/virtual_store_types.go:13-152` — `ActionType` constants (`write_file`, `edit_file`, `edit_lines`, etc.) (read_file verified).
- `internal/core/policy_inventory.go:26-39,108-115` — `defaultCorePolicyModules` list + `IsDefaultPolicyFile` normalization (read_file verified).
- `internal/core/defaults/predicate_corpus.db` — `pending_edit` (2-arg EDB shard) and `pending_mutation` (4-arg EDB) entries (grep hit row `2311`).
- `MANGLE_FACT_APIS_INVENTORY.md:12,186-191,397-400` — `pending_edit` does-not-exist note (superseded for policy, still true for Go producer) + `pending_mutation` lifecycle.
- `WRITE_MUTATION_INVENTORY.md:28-35,78-103` — handler payload shapes and 2PC lifecycle.

---

## 7. Uncertainty & Gaps (explicit)

1. **Grep-engine staleness:** The `grep` tool (index-based) returned 0 hits for `pending_edit`/`pending`/`FilePath` in earlier runs, contradicted by path-scoped `grep` and `read_file`. Index should be considered unreliable for policy completeness — the file list above is based on **path-scoped grep + 7 file reads**, not the global index.
2. **Go producer verification:** `projectdoc.mg:27` comment claims no Go producer for `pending_edit`; this was not re-verified via a fresh `grep` over `internal/core/*.go` for `pending_edit` in this run due to tool budget. Treat as **documented but not re-grep-verified** this session; prior `MANGLE_FACT_APIS_INVENTORY.md` also claimed 0 Go hits.
3. **Content length contract:** Policy uses `_` wildcard — no length or truncation rule is enforced in Mangle. The 200-char truncation is **inferred from `pending_mutation` Go code**, not from `pending_edit` Decl. If `pending_edit` is wired, the chosen truncation (or full content) must be decided explicitly.
4. **Schemas not re-read:** `mangle/*.mg` and `internal/mangle/*.mg` Decl registries were not `read_file`'d this run; `pending_edit` is confirmed in `schemas_shards.mg` so additional Decls are unlikely, but not exhaustively ruled out.
5. **Payload vs Fact:Content divergence:** `VirtualStore` payload `content` (full file for `write_file`, `new` for `edit_file`, line fragment for `edit_lines`) vs `pending_edit` `Content` (opaque string) — policy does not distinguish these; Go wiring must decide which content string to publish.

---

## 8. Minimal Next Investigative Step (if gaps must close)

1. `grep --path internal/core --pattern pending_edit` (Go source, not policy) to re-confirm zero producer, then `read_file internal/core/virtual_store_file_actions.go:186` neighborhood if a producer is found.
2. `read_file internal/mangle/*.mg` and `mangle/*.mg` for any stray `Decl pending_edit` duplicate.
3. Decide and document Content truncation: either `Decl pending_edit(FilePath, ContentPreview)` with `trunc200` helper, or `Decl pending_edit(FilePath, FullContent)` — then update `schemas_shards.mg` comment and wire `TransactionManager.AddEdit` to `Assert` both `pending_mutation` and `pending_edit` atomically (or replace the former).
