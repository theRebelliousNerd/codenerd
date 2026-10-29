# Test Strategy

## Overview
The testing strategy for `cmd/query-kb` ensures that the subsystem correctly connects to SQLite databases, extracts queries, and writes to output buffers securely without data races. The primary test file is `main_test.go`.

## Unit Testing Approach
Located entirely in `main_test.go`, the unit test suite ensures that database extraction logic does not panic or hang on varied schema conditions.

### `TestQueryDBOutput`
This test drives the validation of the `queryDB` function defined in `main.go`.
* **Test Flow:**
  - Employs `t.TempDir()` to construct an ephemeral workspace path (`kb.db`).
  - Uses `database/sql` to open a database connection using the `modernc.org/sqlite` driver.
  - Generates DDL queries to create standard tables (`knowledge_atoms` with `id`, `concept`, `content`, `confidence` columns; and `vectors` with `id`, `content`, `metadata`).
  - Seeds the tables with mocked values using `INSERT INTO`.
  - Injects a `bytes.Buffer` acting as the `io.Writer` interface parameter for `queryDB`, which prevents `os.Stdout` pollution and allows the capture of the textual output string.
* **Assertions:**
  - Specifically uses `strings.Contains` to ensure `Tables:` output is captured.
  - Validates `Total knowledge_atoms: 1` correctly reflects the seeded fact.
  - Validates `Total vectors: 1` correctly reflects the inserted embedding data.

## Missing Coverage
* Currently, there are no tests specifically targeting the concurrent execution path triggered by calling `main()` with zero arguments (which leverages `sync.WaitGroup` and `sync.Mutex`).
* There are no tests for `deep_query.go` output formatting logic.
* Error scenarios (such as corrupted DB files or locked DB conditions) are currently not mocked or tested in `main_test.go`. Future iteration should cover sqlite lock faults.


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
