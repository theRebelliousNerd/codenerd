# Embedding package guidance

- Keep this package a semantic leaf. It may feed retrieval, perception, and JIT,
  but it must not own Mangle permission or action dispatch.
- Treat provider responses as untrusted data. Reject empty or non-finite vectors,
  truncated batches, and mixed batch dimensions before returning them to stores.
- All mutable Ollama model/readiness/pull state belongs under `ensureMu`. Use
  `Model`, `Name`, and the locked invalidation path instead of unsynchronized
  field reads.
- Retry waits must observe the caller context. A short bootstrap ensure timeout
  must not permanently disable a later request-scoped model pull.
- Do not tighten provider vectors to `Dimensions()` until the store migration and
  alternate-model contract are changed together; those values are still reported
  configuration contracts rather than runtime discovery.
- Run both default and race tests after changes. The optional SIMD path requires
  both the build tag and Go experiment:

  ```powershell
  go test -count=1 ./internal/embedding/...
  go test -race -count=1 ./internal/embedding/...
  $env:GOEXPERIMENT = "simd"
  go test -count=1 -tags simd ./internal/embedding/...
  ```
