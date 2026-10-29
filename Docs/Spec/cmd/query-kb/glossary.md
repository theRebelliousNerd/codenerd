# Glossary

## Core Terms

### Knowledge Atom (`knowledge_atoms`)
A fundamental structured fact natively injected into the SQLite system. The definition inside `cmd/query-kb` requires identifying columns `id`, `concept`, `content`, and a `confidence` floating rating evaluating fact integrity via mathematical multiplier conversions (`confidence * 100`).

### Vector Metadata (`vectors`)
An embedded associative array stored natively under the `vectors` SQLite table. Specifically extracted within `deep_query.go` separating `content` from explicitly tied `metadata` properties.

### Deep Query (`deep_query.go`)
An ancillary tool executing explicitly constrained queries against targeted `dbPath` variables. It exposes flag manipulation logic specifically utilizing `--atoms` and `--vectors`.

### Buffer Injection (`bytes.Buffer`)
A mechanism inside `main.go` circumventing concurrent trace output overlap. A unique target memory structure used by `queryDB(dbPath, 5, &buf)` masking arbitrary `io.Writer` interface dependencies from system OS standard variables.

### Wait Group (`sync.WaitGroup`)
A Go standard structurally counting active goroutines tracking ongoing `sql.Open` processes connecting database execution loops concurrently over `os.ReadDir(shardsDir)` files.


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
