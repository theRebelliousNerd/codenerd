# Store guidance

- A successful write must be recallable after reopening the store. New learning
  records get deterministic, sanitized lexical handles before any embedding work.
  Legacy missing handles must remain readable from their original saved facts.
- Reinforcement preserves an existing descriptor and its embedding identity;
  reflection owns descriptor migration and re-embedding.
- Use context-aware recall from runtime paths. Keep borrowed database ownership,
  query limits, provenance, cancellation, and missing-result behavior explicit.
- Test fresh reopen, unembedded and legacy records, and negative queries. A row
  count or an embedding-worker test alone does not establish runtime recall.
- `RecallLearningContentContext` hydrates one ranked fact with a bounded,
  cancellable query and existing secret redaction. Preserve its qualifications;
  reject oversized content instead of treating the short handle as a full fact.
