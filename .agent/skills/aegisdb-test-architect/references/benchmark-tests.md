# Benchmark Tests

## Purpose
Measure performance and allocations to detect regressions.

## Structure
- Use BenchmarkXxx with b.ResetTimer and b.ReportAllocs.
- Use sub-benchmarks for size variants.
- Avoid allocations in setup; use b.StopTimer if needed.

## Inputs
- Stable, realistic datasets.
- No network or filesystem unless explicitly benchmarking I/O.

## AegisDB focus areas
- Wormhole scoring and candidate selection
- Vector search and HNSW insert/query
- Graph traversal and query planning
- Mangle rule evaluation

## Commands
```bash
go test -run ^$ -bench . ./...
go test -bench BenchmarkName -benchmem ./...
```

## Pitfalls
- Do not mix setup work into the timed section.
- Avoid nondeterministic inputs.
