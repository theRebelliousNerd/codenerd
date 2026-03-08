# System Wiring

## Overview
The wiring inside `cmd/query-kb` establishes how individual SQL query logic correctly interfaces with terminal standard output without generating concurrent memory race panics.

## Goroutine Execution Wiring (`main.go`)
1. **Trigger Condition:** Command initiates with `len(os.Args) < 2`.
2. **Setup:** Loop triggers across directories (`os.ReadDir`). For each `.db` extension (`filepath.Ext`), `wg.Add(1)` configures the thread counter.
3. **Execution Block:**
   - Instead of passing `os.Stdout` securely, an isolated instance of `bytes.Buffer` is initialized (`var buf bytes.Buffer`).
   - The buffer string `=== [FileName] ===` receives formatting via `fmt.Fprintf(&buf, ...)`.
   - The functional block `queryDB(dbPath, 5, &buf)` connects the SQL driver directly strictly to the isolated `buf`.
4. **Resolution Block:**
   - Once `queryDB` concludes database locking, a secondary mutex lock (`mu.Lock()`) initializes.
   - The buffered stream binds to standard system output (`os.Stdout.Write(buf.Bytes())`).
   - Finally, `mu.Unlock()` releases terminal control to arbitrary goroutines, and `wg.Done()` logs thread termination.

## Test Injection Wiring (`main_test.go`)
* **Mock Execution:**
  - Testing triggers `queryDB` via `queryDB(dbPath, 1, &buf)`.
  - The `dbPath` relies explicitly on `filepath.Join(t.TempDir(), "kb.db")`.
  - The output avoids manual verification by leveraging `buf.String()` which transforms `db.Query()` loops specifically checking constraints like `"Total vectors: 1"`.


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
