---
doc-class: north-star
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# 12 — Debt, Orphans, and Cleanup — Sensors, Operator Clarification, and CodeDOM Transactions

This capability specification details the design of codeNERD's technical debt detection, orphan code classification, and atomic cleanup transaction subsystem. It fulfills Steve's 2026-09-29 mandate: identifying technical debt, forwarding shims, and cruft; discovering orphaned (un-specced) code and asking the operator to classify it; persisting classifications durably for the whole team; and surgically removing debt and trash through transactional CodeDOM rewrites that build and pass tests before committing.

---

## 1. Technical Debt as Relational Facts (Decision D6)

Technical debt in codeNERD is not an informal comment in a PR review; it consists of structured extensional facts emitted by Go sensors and evaluated by Mangle policy.

### The Debt Sensor Suite

Go sensors scan the repository and emit typed EDB facts directly into the kernel:

1. **Unreferenced Symbols**:
   - Grounded in `StructureIndex.Unreferenced` (`internal/world/structure_index.go:759-793`), extended across all supported languages as CodeDOM parity lands.
   - Compares identifier usage counts against declaration counts across the AST index. Emits `code_debt(SymbolRef, /unreferenced, FilePath)`.
2. **Forwarding Shims**:
   - Detects functions, methods, or types whose body merely forwards its arguments to another symbol with identical signature (e.g. legacy compatibility wrappers, re-export shims, type aliases).
   - Emits `code_is_shim(SymbolRef, TargetSymbolRef, /forwarder | /alias | /reexport)`.
3. **Deprecated Markers**:
   - Extracts `@deprecated`, `// Deprecated:`, or docstring warnings citing replacement symbols.
   - Emits `code_is_deprecated(SymbolRef, ReplacementSymbolRef)`.
4. **Dark Struct Fields**:
   - Sensorizes the static AST analysis from `cmd/tools/audit_dark_fields/main.go:1-77`. Detects exported struct fields that production code reads but never writes outside test fixtures or composite literals.
   - Emits `code_dark_field(StructName, FieldName, FilePath)`.
5. **Dead Code**:
   - Identifies unreachable functions through Rapid Type Analysis (RTA) and call-graph reachability roots.
   - Emits `code_debt(SymbolRef, /unreachable, FilePath)`.
6. **Near-Duplicate Elements**:
   - Computes cosine similarity across normalized CodeDOM element body embeddings. Elements exceeding 95% structural similarity are flagged as duplication candidates.
   - Emits `code_near_duplicate(ElementA, ElementB, SimilarityPermille)`.

---

## 2. Orphan Code and Boundary Exclusions (Decision D7)

Orphaned code represents implementation that exists in the codebase without any specification realizing it, and without any previous operator classification accounting for it.

### Strict Exclusion Discipline

To avoid false positive alarms that overwhelm operators, orphan detection strictly excludes:
- Toolchain entry points: `main`, `init`, benchmark harnesses, and CLI subcommand dispatchers.
- Test suites: files matching `*_test.go`, `test_*.py`, `*.spec.ts`, and test fixture helpers.
- Operational tree treatments: directories classified by orientation as `/index_names_only` or `/exclude` (e.g. generated protobufs, third-party vendored packages, database migrations, golden answer datasets, seed fixtures; see [08-DISCERNMENT-AND-QUESTIONS.md](08-DISCERNMENT-AND-QUESTIONS.md)).
- Private internal helpers: internal functions contained entirely within an already spec-covered module do not trigger orphan status; orphan analysis operates at the level of un-specced packages, public interfaces, and standalone abstractions.

---

## 3. Operator Clarification Dialogue (Decision D7, ADR-011)

When an authentic orphan unit is detected, codeNERD does not guess its purpose or arbitrarily delete it. It initiates an interactive dialogue with the operator via the existing kernel clarification path.

### The Clarification Seam

The dialogue plugs directly into `kernelClarification` (`cmd/nerd/chat/process_dream_delegation.go:26-62`) and the declarative clarification rules in `internal/core/defaults/policy/clarification.mg:42-88`.

### Dialogue Governance and Capping

- **Per-Package Grouping**: Orphan questions are grouped by package directory rather than presented as scattered individual symbols.
- **Session Capping**: The total number of orphan clarification prompts per session is strictly bounded by configuration: `config_param(/orient_orphan_question_cap, 5)`. Additional orphans remain queued for subsequent sessions.
- **Pre-Selected Best Guess**: Model transduction evaluates the code unit and proposes a pre-selected best guess (`orient_question` best guess), accompanied by an evidence snippet summarizing what the code does.

### The Five Canonical Answers

The operator is presented with five explicit choices:

| Answer Atom | Operator Semantic Intent | Subsequent Subsystem Action |
|---|---|---|
| `/feature` | Legitimate production feature missing documentation. | Status becomes `/missing`. The agent schedules authoring a specification in the repo's native format. |
| `/experiment_develop` | Active prototype under development. | Classified as an experiment. Spec is authored as an experimental feature with isolation tags. |
| `/experiment_park` | Useful experiment put on hold. | Code is tagged with parking metadata; excluded from production readiness gates. |
| `/failed_experiment` | Abandoned or failed exploration. | Marked for scheduled removal and caller cleanup. |
| `/trash` | Dead code, obsolete cruft, or accidental residue. | Routed immediately to the atomic CodeDOM cleanup transaction for repointing and deletion. |

### Durable, Team-Shared Persistence

User answers are never stored in ephemeral session memory or local untracked databases:
- Answers are written to the version-controlled file `.nerd/orientation/answers.json`.
- This file is committed to git, sharing classifications across the entire engineering team.
- On system boot, Go sensors load `answers.json`, asserting durable `code_classified(Unit, Answer)` facts into the kernel.
- Durable facts permanently override automated heuristics, preventing the agent from re-asking questions about previously classified code.

---

## 4. The Atomic CodeDOM Cleanup Transaction (Decision D6)

When code is classified as `/trash`, or when technical debt (such as forwarding shims) is scheduled for removal, codeNERD executes the removal through an atomic CodeDOM refactoring transaction.

### Generalizing `repointAndDelete`

Today, `repointAndDelete` (`internal/tools/codedom/repoint.go:310-342` and `element_edit.go:566-568`) rewrites uses and deletes an element in a single write, but is limited to Go package-level functions and types. Methods are explicitly refused (`repoint.go:312`), moves between files are unsupported, and non-Go languages are excluded.

The generalized `CleanupTransaction` engine expands this capability:
1. **Methods and Receivers**: Analyzes method declarations, updating interface implementations and call sites across caller packages.
2. **Cross-File Moves**: Orchestrates the atomic relocation of elements: declaring the symbol in the destination file, updating all import paths, repointing callers, and deleting the source declaration.
3. **Polyglot CodeDOM**: Applies AST-level surgical edits across Go, Python, TypeScript, and Rust files using tree-sitter syntax transformations.

### The Six-Stage Transaction Pipeline

```
[1. Pre-Check] ---> [2. Stage Edits] ---> [3. AST Parse] ---> [4. Atomic Commit] ---> [5. Verify Gate] ---> Done
       |                                                                                    |
       +-----------------------<--- (Rollback on Failure) ---<------------------------------+
```

1. **Pre-Check (Clean Workspace)**: Validates that the git worktree is clean without uncommitted edits (`gitRestoresExactly` in `internal/session/delete_recoverable.go:50`).
2. **Staged Multi-File Edits**:
   - For shims: rewrites all call sites of the shim to point directly to the target implementation.
   - For deletions: removes the declaration and cleans up newly unused imports.
3. **Pre-Commit In-Memory AST Validation**: Parses all modified file buffers using tree-sitter or native AST parsers (`go/parser`). If any syntax error or unresolved reference is introduced, the transaction aborts with zero disk writes.
4. **Simultaneous Atomic Disk Commit**: Writes all modified files to disk simultaneously via atomic file writers (`atomicfile.WriteFile`).
5. **Post-Commit Verification Gate**: Immediately executes the mechanical compiler gate (`verifyBuild`) and runs impacted tests (`run_impacted_tests.go:19-80` and `verifyTestRunner` in `internal/session/verify_outcome.go:74`).
6. **Automatic Rollback on Failure**: If the build fails or any impacted test regresses, the engine executes an immediate `git checkout` rollback, restoring the exact pre-transaction commit state and reporting the verification error. No half-broken state can survive.

---

## 5. Division of Responsibilities

| Component | Responsibility | Implementation Mechanism |
|---|---|---|
| **Debt Scanner** | Detects unreferenced symbols, forwarding shims, deprecated tags, dark fields, and dead code. | Go sensors (`structure_index.go:759`, `audit_dark_fields`, AST walkers). |
| **Orphan Filter** | Filters out toolchain entry points, tests, and excluded tree treatments. | Mangle rules (`internal/orient/orphans.mg`). |
| **Clarification Presenter** | Caps, formats, and renders orphan questions in the TUI; collects operator answers. | Go Bubbletea (`process_dream_delegation.go:26`, `model_handlers.go:417`). |
| **Answer Store** | Persists classifications to committed `.nerd/orientation/answers.json`; asserts facts at boot. | Go JSON store (`internal/orient/answers.go`). |
| **Debt Prioritizer** | Ranks blocking debt and shims in `what_next` derivation. | Mangle IDB rules (`internal/orient/what_next.mg`). |
| **Cleanup Transaction** | Multi-file repointing, deletion, AST validation, build verification, and rollback. | Go CodeDOM (`internal/tools/codedom/cleanup_transaction.go`). |

---

## 6. Verification Tests That Prove This Capability

1. **`TestDebtSensors_ForwardingShimDetection`**: Scans a fixture with legacy forwarding wrappers and type aliases. Verifies that `code_is_shim` facts are asserted with the exact target symbols.
2. **`TestOrphanDetection_ExcludesEntryPointsAndTests`**: Evaluates a repository with `main.go`, `*_test.go`, and an un-specced utility package. Verifies that only the utility package derives as an orphan code unit.
3. **`TestOrphanClarification_AnswersPersistedAndShared`**: Simulates an operator selecting `/trash` for an orphaned function. Verifies that `.nerd/orientation/answers.json` is written, and that rebooting the kernel re-asserts `code_classified(Unit, /trash)` without prompting.
4. **`TestCleanupTransaction_AtomicRepointAndDeleteSuccess`**: Executes a cleanup transaction on a forwarding shim called across three files. Verifies that all call sites are repointed to the target, the shim is deleted, the build passes, and tests succeed.
5. **`TestCleanupTransaction_RollbackOnTestFailure`**: Executes a cleanup transaction where the repointed target contains a behavioral divergence causing an impacted test to fail. Verifies that the engine executes an immediate rollback, leaving all original files intact.

---

## 7. Failure Modes and Safety Tripwires

- **Question Fatigue**: Presenting dozens of orphan questions on initial boot will cause operators to cancel or blindly click through. Mitigation: strict session cap (`config_param(/orient_orphan_question_cap, 5)`) and grouping by package.
- **Accidental Deletion of Reflection-Invoked Code**: Code invoked via reflection, string lookup, or dynamic plugin dispatch may have zero static call sites. Mitigation: code is never deleted automatically; deletion requires explicit operator confirmation (`/trash`), and is protected by the post-commit test verification gate.
- **Broken Shim Repointing Due to Type Casts**: If a forwarding shim introduced an implicit type conversion, repointing callers directly to the target could break type checking. Mitigation: in-memory AST validation and build verification catch type mismatches, triggering clean rollback before changes are kept.
