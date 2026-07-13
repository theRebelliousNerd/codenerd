# TODO — perception

> Prioritized backlog. Last verified: **2026-07-13**  
> Docs-only session produced this list from code audit; items are **not** auto-scheduled work.

## P0

- [ ] Audit all provider retry sites to wrap durable 5xx as `ErrLLMUnavailable` (beyond Gemini).  
- [ ] Ensure chat/session boot always prefers classification client when non-nil.  
- [ ] Keep break-test green on any parser/sanitize change.

## P1

- [ ] Formalize capability interfaces (`ToolCaller`, `Streamer`, `SchemaCompleter`) documented in one matrix.  
- [ ] Emit metrics when SharedSemanticClassifier is nil at first ParseIntent (counter).  
- [ ] Align prompt atoms under `internal/prompt/atoms/` with Understanding contract validators.  
- [ ] Refresh `internal/perception/README.md` default models vs factory.

## P2

- [ ] Resolve fate of `LLMTransducer.validate` (delete with comment tombstone or re-enable with policy).  
- [ ] Document which call sites still use `matchVerbFromCorpus` vs Understanding path in runtime boot.  
- [ ] Consider DI for SharedTaxonomy / SharedSemanticClassifier to ease multi-workspace tests.  
- [ ] Export `GetLLMMetrics` into observability/glass-box CLI.

## P3

- [ ] Golden test: Piggyback schema builders match articulation constant continuously.  
- [ ] Race CI job for `go test -race ./internal/perception/...` on PRs touching corpus/taxonomy.  
- [ ] xaioauth probe coverage for refresh failure modes.  
- [ ] Reduce log volume at default verbosity without losing bottleneck labels.

## Done recently (do not re-open without evidence)

- Classification model tiering (no inherit main model).  
- Stability-bypass removal.  
- Intent hydrate timeout.  
- TransientFailure signalling for Gemini outages.  
- ConsolidationWorker stopOnce.  
- Taxonomy one-shot schema load.
