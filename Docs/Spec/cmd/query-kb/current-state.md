# query-kb — Current State

> **Spec Status**: 🟢 Complete
> **Last Updated**: 2026-03-07
> **Source of Truth**: `cmd/query-kb/`

---

## Implementation Status

The `cmd/query-kb` package provides a standalone set of debugging utilities for inspecting `.nerd/shards/*.db` SQLite knowledge bases directly from the CLI without invoking the full codeNERD Cortex.

### What's Built and Working

* **Database Discovery**: The `main.go` file discovers and samples `.db` files in `.nerd/shards/` automatically running highly concurrent loops.
* **Direct Querying**: The `main.go` tool natively dumps schema, row counts, and samples of `knowledge_atoms` and `vectors`.
* **Deep Querying**: `deep_query.go` provides formatted extraction of explicit concept atoms and numeric vectors utilizing strict iteration arrays.

### What's Built but Incomplete

* These are standalone `.go` scripts; they are exceptionally isolated, not formally integrated into a proper Cobra CLI hierarchy, and distinctly lack `--help` or any unified structured command flags.

### What's Stubbed or Planned Only

* Additional filtering metrics. As dedicated targeted debugging capabilities, they currently serve their singular purpose successfully without complex long-term roadmaps requiring stubbed systems.

## Step-by-Step Behavior Trace

1. The developer executes the utility sequentially (e.g., `go run cmd/query-kb/deep_query.go <db_path>`).
2. The `main` function executes natively analyzing the `os.Args` parameters passed over the CLI bounds.
3. The underlying pure-Go `modernc.org/sqlite` database driver successfully opens a read-only filesystem lock on the specified `.db` target mapping.
4. The system iterates tightly through explicitly constructed SQL string selections (`SELECT * FROM...`).
5. A dynamic sequential `rows.Scan()` maps active database column responses into empty memory slices immediately on the fly.
6. The resulting string mapping bounds are instantly pushed safely to terminal `os.Stdout` streams natively utilizing raw `fmt.Printf()` functions without structured logging overhead.
7. The process terminates naturally resulting exactly in a zero exit code output status.

## File Breakdown

| File | Purpose | Lines | Status |
|------|---------|-------|--------|
| `main.go` | Dumps schemas, table structured counts, and basic numeric visual samples. | 162 | Fully Functional |
| `deep_query.go` | Extracts concepts and arrays natively bypassing structural offsets. | 93 | Fully Functional |
| `main_test.go` | Simple tests validating utility output structures rapidly. | 50 | Passing CI Checks |

## Key Types and Interfaces

This package exports precisely zero significant externally usable compilation interfaces or types. It relies exclusively on the standard library native `database/sql` combined tightly with the `modernc.org/sqlite` pure-Go translation drivers to securely interface with the active memory layout `.db` physical files.

## Known Limitations

* **Hardcoded Schemas**: The secondary tools assume the rigid existence of specifically named tables. If the semantic storage schema eventually dynamically evolves wildly locally inside `internal/store`, these specific static scripts will cleanly break backwards compatibility parameters or silently omit resulting database row extraction variables.
* **Pagination Constraints**: Running queries currently deploy hardcoded `LIMIT` clauses; there is precisely zero support actively enabled allowing users interactive dynamic visual terminal scrolling mechanisms across massive localized multi-gigabyte storage tables.

## Technical Debt

* **Duplication Architectures**: The isolated utility files `main.go` and `deep_query.go` both perform largely aggressively overlapping connection tasks and absolutely aren't cleanly consolidated into reusable connection libraries.
* **Lack of Formalized CLI Structure**: Aggressive unmanaged `os.Args` numerical array length parsing logic makes the script tools noticeably fragile terminating actively in the presence of unexpected unstructured user typing inputs.
