# Integration fixture guidance

- Config factory fixtures must resolve the same tool envelope before prompt
  compilation that their `Generate` method later returns. Preserve each test's
  failure injection and assertions when adapting interfaces.
- Compile with `go test -tags integration ./tests/e2e -run '^$'` after shared
  executor interface changes, then run the affected behavioral tests.
