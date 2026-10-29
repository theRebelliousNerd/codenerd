# Literal preserving identity for main chat knowledge requests

## Scope and relationship to earlier repairs

Main-chat knowledge-request deduplication deliberately uses approximate textual equivalence. That equivalence is too broad for technical questions containing meaningful case or whitespace: two distinct requests can become one before execution budgeting and outcome reporting. This is a precision extension to intentional deduplication, not hidden normalization, a global consultation-cache collision, a permission bypass, or proof of an incorrect final answer.

This addendum credits the existing repairs. The consultation-coordination audit explicitly recognizes main-chat repeated-ask folding before the cap; its discussion of full length-delimited question keys concerns the separate shared consultation manager. The terminal-grounding repair preserves parent context and reports failed, empty, and not-run outcomes. The issue here occurs upstream of that repaired result projection. Neither repair needs to be characterized as absent or ineffective. [Consultation coordination](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/consultation-coordination-and-safe-reuse.md#L114-L139), [terminal grounding](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/terminal-knowledge-synthesis-grounding.md#L157-L173).

Production references below are pinned to `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, revalidated on 2026-10-29 at 16:08 UTC. Inspection also found the exact `process_knowledge.go` unchanged in active code heads `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and `aa7988dec4c62116656ea722ff73b687a8509908`, and QA head `51cfd42649552866d4aa54c53b68acf5740eb09b`. This is not a claim of full branch equivalence. Research head `317a0fcc109f31b421435cf0826b3420cb41afea` was screened; its learning-lexical-candidate-window and tester-continuation-proposal-admission additions concern different boundaries.

## Where a distinct obligation disappears

`KnowledgeRequest` contains specialist, query, purpose, and priority. The schema requires all four, permits arbitrary query and purpose strings, and restricts priority to `required` or `optional`. Direct JSON parsing copies admitted control without normalizing those strings, and parser admission explicitly does not truncate knowledge requests. A user question alone does not guarantee any consultation: the witness requires a successfully completed articulation containing an admitted envelope, initialized execution, allowed execution, and no `awaitingKnowledge` recursion blocker. The main loop then passes the requests into `handleKnowledgeRequests`. [Types](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/protocol_types.go#L143-L156), [schema](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/schema.go#L125-L150), [admission](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/emitter.go#L219-L226), [decoding](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/emitter.go#L620-L671), [parser limit boundary](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/articulation/emitter.go#L412-L419), [consumer](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L1048-L1056).

The handler deduplicates at line 81 before applying the two-request cap at line 84. Only filtered survivors beyond that cap receive not-run results. Deduplication first resolves `_any_specialist`, then builds a key from specialist, normalized query, and normalized purpose, separated by NUL characters. `normalizeAsk` lowercases the entire string and joins `strings.Fields` with one space. A repeated key logs a generic repeat and continues; it creates neither a duplicate alias nor a dropped-query result. The first original request survives. Its actual worker task still uses the original query and purpose: identity is transformed, not the retained prompt. [Cap and dispatch](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_knowledge.go#L31-L136), [identity function](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_knowledge.go#L179-L202).

This is a same-batch loss. Dispatched retained requests produce `KnowledgeResult` records; the completion message combines those records with cap-generated not-run records and retains original input and parent context. The update handler stores and counts those surviving results before synthesis. Synthesis includes recent turns, compressed context, parent evidence, and original input, and reports missing outcomes from its result list. A normalized-away request never enters that list. The missing object is a second dispatched question or explicit merge/not-run outcome at the `Results` boundary, not universal information erasure: raw response or logging may retain the asks, and original input remains available. The final answer can therefore still be correct through parent evidence, original input, general knowledge, or a first specialist's broader answer. [Result projection](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_knowledge.go#L130-L175), [update handling](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L884-L932), [grounded synthesis](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_knowledge.go#L261-L313).

## Two-request literal witnesses

The following is a schema-valid request-array fragment, not a complete articulation envelope. A real admitted envelope must contain it:

```json
[
  {
    "specialist": "researcher",
    "query": "Explain Go regexp \\s.",
    "purpose": "Choose a character class.",
    "priority": "required"
  },
  {
    "specialist": "researcher",
    "query": "Explain Go regexp \\S.",
    "purpose": "Choose a character class.",
    "priority": "required"
  }
]
```

Each decoded query contains one backslash. Go's regexp syntax distinguishes the ASCII whitespace class `\s` from its non-whitespace counterpart `\S`; lowercasing erases the distinction. [Go regexp syntax](https://pkg.go.dev/regexp/syntax#hdr-Syntax).

With the same specialist and purpose, both requests therefore receive the same key. The first query remains verbatim, and the second disappears from execution and result coverage. There are only two requests, so the cap is not responsible. Reversing order changes which question survives, not whether one loses independent execution. The second spawn attempt is absent even if the first fails; requiring a successful nonempty first result is useful only for a clean final-coverage probe. The `researcher` profile uses the ordinary research route rather than depending on arbitrary expert registration. [Persona routing](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/delegation.mg#L492-L519).

Whitespace provides a separate fixture. These decoded queries differ inside a quoted string:

```text
What is the length of Go string "a b"?
What is the length of Go string "a  b"?
```

With identical specialist and purpose, `strings.Fields` collapses the difference. Go string-literal rules make that internal spacing part of the value, so this is not merely framing whitespace. [Go string literals](https://go.dev/ref/spec#String_literals). Purpose-only differences deserve corresponding fixtures because purpose is normalized too.

The existing test deliberately coalesces `What is X?` with `  what is   x? ` and `p` with `P`, while retaining a different question. Another test confirms resolution of `_any_specialist` before comparison. These are inspected test definitions, not executed evidence. A remedy changes the intended equivalence contract and must update its controls explicitly; it is not simply filling an accidentally missing test. [Current tests](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/knowledge_requests_test.go#L11-L39).

## Conservative identity and complete dispositions

Define deduplication around preserving every original obligation, not creating one worker for every spelling variant. Use an unambiguous structured encoding of the resolved specialist, exact decoded query, and exact purpose. Include additional operational parameters if they affect admission or execution. Conservatively preserve literal bytes; even trimming framing whitespace should require an explicit contract declaring it irrelevant. Preserve first-occurrence ordering, exact-duplicate coalescing, and the existing two-request cap. Near-equivalent wording may consume another slot or leave a third request not run; that tradeoff requires honest accounting rather than silent disappearance.

Separate an original request's identity or index from its execution key. Every original request should receive a disposition: computed, reused, not admitted, failed, empty, or canceled. Exact duplicates can alias one computation and its outcome without another worker. Preserve each original question and purpose alongside identity-version and reuse provenance. These alias mappings and per-original receipts are proposed additions, not existing behavior. Synthesis should distinguish reused coverage from independent answers without requiring internal bookkeeping in every user-facing response.

One worker per distinct question is not fundamental. An optional cost-aware design could deliberately bundle both original questions into one bounded specialist task, retaining explicit coverage and receipts for each without circumventing original-request or work caps. Current code does not perform that lossless bundling: its task retains only the first query.

Do not add an LLM call to decide semantic identity or a fragile classifier that attempts to parse every programming language's quoting rules. Any later literal-aware scheme should preserve opaque literals and unknown grammars conservatively. Go can provide exact encoding at the host boundary; expanded admission and lifecycle policy should remain consistent with the Mangle executive and typed policy architecture. The current cap is a Go constant, not an existing Mangle rule. Preserve contextual authority, permission checks, cancellation, and timeouts; this proposal neither expands execution permissions nor claims to repair separate ownership or restriction findings.

## Evaluation and cost accounting

Compare current behavior with conservative exact-tuple identity. A literal-aware alternative can be exploratory; disabling deduplication is a diagnostic, not the preferred design. Exercise valid full articulation envelopes through the real parser, actual handler with a recording executor, `knowledgeGatheredMsg`, and synthesis capture. Below the cap, each literal witness should retain both correct queries, preserve their original bytes, and produce two outcomes. Measure final-answer correctness separately from execution coverage.

Controls should include exact duplicates yielding one worker and two aliases; ordinary case variation with its intentionally changed contract and cost; case-sensitive code; quoted double whitespace; Unicode and newline literals; different purposes and specialists; resolved `_any_specialist`; first-worker failure, empty output, and cancellation; a distinct third request receiving a not-run receipt; malformed or admission-rejected envelopes causing no effects; and unchanged safe permission denial. Verify receipt coverage even when a shared computation fails. These are proposed evaluations, not run results.

Report same-batch false merges, original-request receipt coverage, exact-query fidelity, verified answer accuracy, missed required questions, and unnecessary duplicate calls. Count all LLM tokens across articulation, specialists, synthesis, failures, and retries; tool, build, and search CPU; wall latency including parallel completion and provider queuing; storage and prompt bytes for receipts; and human follow-up. Metadata is linear in the number of retained original records, while encoding and hashing scale with total input bytes; keep diagnostic copies bounded. These are design expectations, not benchmark findings.

For the two-request witness, the change can raise specialist calls from one to two, fulfilling the original requested work without increasing the cap. Higher legitimate work is not automatically a regression; compare quality under equal budgets, and do not count dropping a distinct question as a cost win. Conversely, conservative identity can admit unnecessary near-duplicate work, so quantify that cost rather than promising savings.

This addendum is source-only. No tests, implementation, runtime experiments, or live model calls were performed.
