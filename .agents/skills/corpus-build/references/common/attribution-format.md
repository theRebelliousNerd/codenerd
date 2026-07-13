# Spec Attribution Format

New or changed Go (and TypeScript, where the corpus specifies frontend
surface) files produced by the corpus-build fleet carry a machine-checkable
pointer back to the architecture doc section that justifies them:

```go
// SPEC: docs/architecture/causal/05-IDENTIFICATION-STRATEGIES.md#adjustment-set-recommendation
func RecommendAdjustmentSet(...) (...) {
```

## Placement and format

- One `// SPEC: <doc>#<section>` comment near the package doc comment or
  directly above the primary symbol (type, func, handler) that implements
  the described surface. Do not attribute every line — attribute the
  entry point a reader would look for first.
- `<doc>` is the repo-relative path to the owning doc, e.g.
  `docs/architecture/wormhole/05-08-TRIFECTA-SCORING.md`.
- `#<section>` is a lowercase, hyphenated anchor matching the doc's heading
  (mirrors GitHub-flavored anchor generation) — enough to jump straight to
  the relevant paragraph, not just the file.
- If a symbol implements a `codeNERD_FEATURE`-tagged surface, prefer citing
  the tag's owner doc + section over a paraphrase; cross-check the id
  against `docs/architecture/roadmap/33_corpus_context_index.json`
  (`features.<id>.owner_doc`) when available so the attribution and the
  tag index agree.
- Multiple spec sections implemented by one symbol: stack multiple
  `// SPEC:` lines rather than combining paths in one comment.

## Enforcement

- `spec-attribution-check.ps1` (PostToolUse on Write/Edit for builder
  agents) checks new/changed `.go` files for at least one `// SPEC:` line
  near the package or primary symbol. It is **warn-level**
  (`additionalContext`), not block-level, in the first iteration — a
  missing attribution surfaces to the agent but does not fail the tool
  call.
- `corpus-critic` treats a missing or clearly-wrong attribution (pointing
  at a doc/section that does not describe the code beneath it) as a review
  finding, not an automatic reject — judge intent, not just presence.

## Why this exists

Attribution is what lets a future agent (or `corpus-doc-auditor`) walk from
code back to spec without re-deriving intent from the diff. It is also what
makes IMPLEMENTED_SPEC reconciliation mechanical instead of a fresh
grep-and-guess exercise every run.
