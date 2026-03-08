# Design Decisions

## Architecture Decision Records

### ADR-001: Zero-CGo SQLite Driver Target
- **Date**: 2025-Q1
- **Status**: accepted
- **Context**: Relying structurally on SQLite requires importing database bindings originally tied directly to standard C GCC compilations (`github.com/mattn/go-sqlite3`).
- **Decision**: Implemented `modernc.org/sqlite` acting as a purely Go-compiled database driver.
- **Alternatives Considered**: (1) Utilize the standard C bindings â€” rejected because Windows cross-compilation historically struggles compiling CGo natively. (2) Build custom JSON database stores â€” rejected because Mangle kernels run exclusively over SQLite graphs.
- **Consequences**: Positive: Single-binary cross-compilation operates perfectly. Negative: Query extraction latency suffers minor performance degradations parsing extreme blob sizes natively.

### ADR-002: Dynamic Pointer Array Row Maps
- **Date**: 2025-Q1
- **Status**: accepted
- **Context**: The `knowledge_atoms` tables update properties actively depending strictly on external intelligence schema layers which remain unstructured.
- **Decision**: Developed `valuePtrs := make([]interface{}, len(cols))` mapping `rows.Scan` inputs dynamically instead of enforcing structured `struct` field tags.
- **Alternatives Considered**: (1) Require strictly defined Go models handling queries â€” rejected because external ShardAgent evolutions constantly break internal CLI tools.
- **Consequences**: Positive: The subsystem dynamically survives arbitrary table upgrades gracefully rendering generic values. Negative: Output text occasionally requires generic string format mappings slicing raw interface cells `s[:100]`.

### ADR-003: Isolated Bytes Buffer Sink
- **Date**: 2025-Q2
- **Status**: accepted
- **Context**: Printing dynamically directly scanning rows to terminal outputs while executing `.db` files concurrently creates massively interleaved string text outputs randomly slicing `fmt.Fprintf` logs.
- **Decision**: Wrapped targeted SQL writes passing `io.Writer` bounds tracking exclusively inside a local `var buf bytes.Buffer` struct.
- **Alternatives Considered**: (1) Serialize the database searches linearly â€” rejected because loading dozens of SQLite databases takes extensive wall-clock execution limits. (2) Pass global stdout safely â€” rejected because `os.Stdout` lacks thread-safety protections inherently.
- **Consequences**: Positive: Guaranteed pristine readable format structures cleanly rendering each shard block uniquely. Negative: Unbounded queries consume linear memory structures mapping outputs strictly internally before finally locking `mu.Lock()`.

## Open Questions
* Should dynamic schema row iterations explicitly parse embedded JSON schemas formatting keys cleanly rather than printing unstructured string interfaces?
* Should limits map automatically relative to dynamic terminal height sizes rather than enforcing fixed static counts internally inside the source code?

## Revisit Candidates
* ADR-003 chose `bytes.Buffer` arrays holding formatting blocks before execution locks map strings universally to `os.Stdout`. If shard databases output 10M rows erroneously violating the limit constraints, this will trigger explicit Out Of Memory panics crashing the client.


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
