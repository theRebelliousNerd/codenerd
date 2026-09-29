# North-star atoms

- `derive_*` atoms define the library's classification, vision and requirements JSON protocols; keep their `/derive_*` phases distinct from the interactive prose phases.
- Document and summary frames are untrusted evidence. Non-final pages return cumulative summaries, and final pages return the phase's structured payload.
- Preserve source paths and draft markers across pages. Vision fields require supplied source paths; role claims require a document evidence line.
- Protocol edits require matching library parsers and client-boundary tests in `internal/northstar/derive*_test.go`; the TUI delegates to that library.
