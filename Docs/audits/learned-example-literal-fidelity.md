# Learned-example literal fidelity at the persistence boundary

Status: source-grounded proposal, not an observed incident or measured result. Reviewed CodeNERD main at [95654a6](https://github.com/theRebelliousNerd/codenerd/commit/95654a64562f02c00040ac28ca9cda4733c96fd9). No tests, inference, training, or implementation were performed.

## Finding

A valid learned example containing an escaped quotation mark can lose its quoting when normalized. [ParseLearnedFact](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/learning.go#L437-L478) strips field quotes and converts escaped quotes to literal quotes. [NormalizeLearnedFact](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/learning.go#L485-L512) then interpolates the decoded fields into quoted Mangle source without re-escaping. Thus an example whose user text contains say \"hello\" can become source containing an unescaped quotation inside a quoted field. This is a source-derived witness, not an executed reproduction.

[PersistLearnedFact](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/learning.go#L330-L375) passes the normalized source to the loader before appending the durable file or writing projections. The [loader](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/mangle/engine.go#L317-L334) rejects parse errors before mutation and removes an analysis-failing fragment. The supported impact is malformed or changed literal serialization and possible failure to retain a legitimate correction. This review does not establish a successful injection, permission bypass, persistent exploit, or frequency of failure.

## Actual routes and existing protection

- The [consolidation worker](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/consolidation.go#L246-L271) passes learned results to persistence.
- [Automatic chat correction](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/cmd/nerd/chat/learning_auto.go#L13-L85) includes fit and confidence gates and escapes producer text, but the shared parser subsequently decodes escaped quotes before raw reconstruction.
- [Explicit confirmation](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/cmd/nerd/chat/model_handlers.go#L374-L402) is another caller. A prior [lifecycle audit](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/Docs/audits/learned-correction-integrity-lifecycle.md) addresses related transport and persistence concerns; its confirmation-focused description does not cover the newer automatic route.
- The existing parser checks the predicate prefix, five fields, and full confidence consumption. These are useful restrictions, but string arguments are not validated as typed AST values and the reconstructed verb is interpolated as raw text. Neither universal attacker control nor complete downstream protection is established.

## Proposed contract

Parse one input unit through the existing [serialized Mangle parser boundary](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/mangle/parse_lock.go). Require exactly one ground learned_exemplar with five expected argument types, no extra declarations or clauses, an allowed verb representation, and confidence normalized according to the existing compatibility contract. Convert the parsed values to a typed descriptor once. Use canonical Mangle serialization for the hot-load and derive durable and semantic projections from that same descriptor, not by independently reparsing the generated source. A validation failure must leave all three destinations unchanged.

A small escape-on-output patch could repair this quote case with less change, but retains the bespoke grammar and separate raw verb interpolation. A structured model-output migration is a broader alternative with prompt and compatibility costs. Neither the parser helper nor the larger migration should be presumed a drop-in: verify complete input consumption, AST kinds and the existing parser lock. See the [Mangle repository](https://github.com/google/mangle) for language and parser documentation; the exact deployed fork and version remain the implementation authority.

## Evaluation and full costs (proposed, not run)

1. Use independent typed-value round-trip oracles for quotes, backslashes, commas, Unicode, newlines, empty fields, slash-leading strings and confidence forms. Keep existing valid no-pattern cases.
2. Reject extra clauses, wrong predicate/arity/types, variables, unfinished literals and invalid confidence before mutation. Verify that parse and analysis failure leaves hot-load, durable file and projections unchanged.
3. Exercise consolidation, automatic chat correction and explicit confirmation, then reopen and retrieve the same value. Separate successful persistence from retrieval/index freshness and from unverified crash recovery.
4. Compare the current implementation, targeted escaping, and typed parsing on identical examples, using exact-value preservation, false rejection and failed persistence as correctness outcomes. Report parser calls, CPU, allocations, lock contention, storage work, latency and failure recovery. No model call or embedding is intrinsically needed for validation; include any embedding, model tokens, retries and migration/backfill costs actually used by each complete system.

The goal is preserving a valid correction across the full boundary, not merely making one helper test pass. No speedup or accuracy gain is claimed.
