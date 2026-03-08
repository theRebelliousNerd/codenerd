# Future TODOs

## Priority Legend
- ðŸ”´ P0 â€” Critical: Blocks core functionality or causes failures
- ðŸŸ  P1 â€” High: Significant improvement, should be done soon
- ðŸŸ¡ P2 â€” Medium: Valuable but not urgent
- ðŸŸ¢ P3 â€” Low: Nice to have, do when convenient

## TODO Items

### ðŸ”´ P0
- [ ] Implement exponential backoff mechanics across `sql.Open` isolating connection locks triggering standard `database is locked` error failures
- [ ] Implement Goroutine concurrency checks spinning strictly up bounded buffered channel semaphores checking limits across arbitrary database directory execution maps

### ðŸŸ  P1
- [ ] Standardize `os.Args` parameter bindings passing strictly towards established `flag` package commands mitigating directory mapping vulnerabilities
- [ ] Implement explicit integer assertions parsing schema maps correctly bypassing unstructured `err := rows.Scan` panics terminating operations randomly

### ðŸŸ¡ P2
- [ ] Consolidate tests isolating native parallel processing mechanisms spanning across concurrent SQL testing scenarios generating fake target bases within `t.TempDir()`
- [ ] Generate comprehensive integration validation tracking `--atoms` and `--vectors` inputs mapping specifically to independent `deep_query.go` processes

### ðŸŸ¢ P3
- [ ] Integrate explicit colored CLI formats mapping ANSI logs specifically isolating schema strings tracking cleanly through outputs
- [ ] Apply `strings.Builder` constraints slicing string iterations rapidly optimizing dynamic data extraction limits `s[:100]`

## Completed (Archive)
- [x] Initial binary SQLite scanner mappings (2025)


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
