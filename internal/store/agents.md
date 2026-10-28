# Store guidance

- A successful write must be recallable after reopening the store. New learning
  records get deterministic, sanitized lexical handles before any embedding work.
  Legacy missing handles must remain readable from their original saved facts.
- Reinforcement preserves an existing descriptor and its embedding identity;
  reflection owns descriptor migration and re-embedding.
- Use context-aware recall from runtime paths. Keep borrowed database ownership,
  query limits, provenance, cancellation, and missing-result behavior explicit.
- Learned-store bootstrap carries the caller context through connection
  admission, per-connection pragmas, readiness, schema transactions, and vector
  backfill. Failure rolls back and joins owned cleanup before returning; no
  partially initialized store or detached native open may be published.
- Test fresh reopen, unembedded and legacy records, and negative queries. A row
  count or an embedding-worker test alone does not establish runtime recall.
- `RecallLearningContentContext` hydrates one ranked fact with a bounded,
  cancellable query and existing secret redaction. Preserve its qualifications;
  reject oversized content instead of treating the short handle as a full fact.
- Generic `vectors` rows carry `embedding_model`, the producing engine's `Name`.
  Every semantic read (vec_index, brute force), the vec_index builders and the
  count/contents helpers admit only rows stamped with the attached engine's
  space (`vector_identity.go`); stamp from the engine captured before the
  provider call. Unstamped or foreign rows stay stored and keyword-recallable,
  and are never stamped by guesswork. A force re-embed returns
  `*IncompleteReembedError` unless every embeddable row is in the target space.
