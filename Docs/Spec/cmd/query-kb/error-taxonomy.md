# Error Taxonomy

## Overview
This document categorizes the error types and handling mechanisms utilized within the `cmd/query-kb` subsystem. Error handling is primarily simplistic, opting directly to output error strings rather than complex wrapping.

## Fatal Initialization Errors
* **`os.ReadDir` Failures:**
  - Encountered in `main.go`.
  - Condition: Triggered when the initial `.nerd/shards` directory scan fails.
  - Logging Strategy: `fmt.Printf("Error reading shards dir: %v\n", err)`.
  - System Impact: Immediate execution halt via `os.Exit(1)`.
* **Argument Insufficiency in `deep_query.go`:**
  - Encountered in `deep_query.go`.
  - Condition: Process triggered without at least one parameter for `<database.db>`.
  - Logging Strategy: `fmt.Println("Usage: go run deep_query.go <database.db> [--vectors|--atoms]")`.
  - System Impact: Immediate execution halt via `os.Exit(1)`.

## SQL Query Failures
* **Database Connection Failures (`sql.Open`):**
  - Encountered across `main.go`, `deep_query.go`, and `main_test.go`.
  - Condition: Unreadable database file or locked file.
  - Logging Structure: `fmt.Fprintf(w, "Error opening DB: %v\n", err)` or `fmt.Printf("Error: %v\n", err)`.
  - Recovery Strategy: Returns locally, halting query processing but not crashing the host procedure.
* **Schema Query Failures:**
  - Originates from `SELECT name FROM sqlite_master WHERE type='table'`.
  - Condition: Schema extraction failure.
  - Logging Strategy: `fmt.Fprintf(w, "Error querying tables: %v\n", err)`.
* **Missing Tables (`PRAGMA table_info(knowledge_atoms)`):**
  - Condition: `knowledge_atoms` does not exist in the referenced `dbPath`.
  - Logging Strategy: `fmt.Fprintf(w, "No knowledge_atoms table\n")`. Returns early.
* **Extraction Failures (`rows.Scan`):**
  - Condition: Failure applying row cell memory pointers dynamically in `valuePtrs`.
  - Logging Strategy: `fmt.Fprintf(w, "Scan error: %v\n", err)`.
  - Recovery Strategy: Single row is skipped via `continue` keyword, the iteration continues.


## Extended Documentation Notes

The above specification details represent the functional baseline for the system. Code definitions in `main.go` establish the connection loops.
The sub-routines located in `deep_query.go` handle the iterative data parsing.
Validation checks rely on testing logic found inside `main_test.go` to ensure correctness.
This subsystem primarily reads from SQLite schemas without modifying any core tables directly.
Goroutine synchronization guarantees that standard output remains ungarbled across threads.
Database locks encountered during execution indicate activity from concurrent agents.
We maintain a strict zero CGo dependency policy for maximal cross-platform compatibility.
Future iterations will refine the command-line flags to offer structured output caps.

These constraints limit memory explosion when querying tables that store gigabytes of embedded vectors.
Overall, the query tools function as diagnostic read-only interfaces rather than state mutators.
