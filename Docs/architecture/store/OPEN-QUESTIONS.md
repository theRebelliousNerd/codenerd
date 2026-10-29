# store — Open Questions

> Last verified: **2026-07-13**

## Real open questions

### Q1. Should ANN drift be fail-hard on write?

Today vec_index failures warn and leave the vectors row. Fail-hard would protect search quality but could drop durable content writes on transient vec errors. Which is preferred for production?

### Q2. Single knowledge.db vs multi-file forever?

Learnings and tools are separate by design. As more journals appear (e.g. browser traces), is the rule “always separate high-volume append logs” codified, or should some merge for joinability?

### Q3. Who owns prompt atom schema evolution?

Columns live in store migrations; atom semantics live in `internal/prompt`. Is a joint schema version required, or is additive-only enough forever?

### Q4. Reflection vs online embed-at-write

Many paths embed synchronously on write; reflection fills gaps. Should new write paths always dual-write embeddings when an engine exists (stricter), or keep async reflection as primary for heavy traces?

### Q5. Embedded corpus vs LearnedCorpusStore product boundary

When both exist, which wins for intent classification, and is that policy documented only in perception?

### Q6. Require-vec default for release binaries

Should release builds flip to `sqlite_vec` required, or remain optional with brute-force fallback for wider install base?

### Q7. Stats / glass-box surface

Should store stats feed transparency/glass-box UI pages, or remain CLI/debug only?

## Resolved / non-questions

| Item | Resolution |
|------|------------|
| Is store the safety gate? | No — kernel/VirtualStore |
| Is missing local `.mg` a bug? | No |
| Is keyword fallback legitimate? | Yes when engine nil |
