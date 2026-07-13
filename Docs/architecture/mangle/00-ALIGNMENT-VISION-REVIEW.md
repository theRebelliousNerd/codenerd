# 00 — Alignment / Vision Review: mangle

> Last verified: 2026-07-13  
> Scores are evidence-based against the codeNERD north star, not marketing.

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully realized in production paths |
| 4 | Implemented with known partials |
| 3 | Core present; integration or coverage gaps |
| 2 | Prototype / half-wired |
| 1 | Spec-only or largely unused |

## Dimensions

| Dimension | Score | Evidence |
|-----------|------:|----------|
| **LLM = creative, logic = executive** | **5** | Package never decides `next_action`. It validates, sanitizes, evaluates. Feedback loop generates candidates; kernel/policy admits them. |
| **Constitutional safety (Decl + forbidden heads)** | **4** | `SchemaValidator.forbiddenLearnedHeads` blocks learning `permitted`, `safe_action`, approvals, pipeline spoof predicates (`schema_validator.go`). Gate is text-regex + Decl map — not full semantic proof. Runtime `permitted(...)` remains in core policy. |
| **JIT / structured generation path** | **4** | `feedback.FeedbackLoop` + optional `synth` (`mangle_synth_v1`) + `PredicateSelectorInterface` (wired from prompt selector). Legislator / mangle_repair / executive autopoiesis use the loop. Not every rule path requires synth. |
| **Deterministic evaluation substrate** | **5** | `Engine` + stratified `EvalStratifiedProgramWithStats`, gas limits (`DerivedFactsLimit` / kernel `derivedFactLimit`), auto-eval toggle. |
| **Incremental / differential eval** | **3** | `DifferentialEngine` + unified fast path exist; kernel routes via `features.IsDiffEvalEnabled()`. Created-fact gas is forwarded and regression-tested on unified/legacy paths. External predicates and provenance still require full fallback; unified Query/Snapshot reads remain open. |
| **Parse safety / concurrency** | **5** | Process-wide `parseMu` in `parse_lock.go`; core `parseUnit` delegates — race fixed by design. |
| **Schema drift prevention** | **4** | Decl extraction, arity checks, undeclared body predicates. Regex Decl parser is approximate vs full analysis (edge cases with multi-line Decls). |
| **Observability / glass box** | **3** | `ProofTreeTracer`, engine stats, kernel logging of eval paths. Proof tree is not the same as Codeberg `provenance.DerivationRecorder` used on full kernel path. |
| **IDE / operator tooling** | **3** | Full `LSPServer` implementation + CLI mangle-check; depth of real editor integration depends on cmd wiring and client setup. |
| **Test density** | **4** | Large `engine_test`, `differential_test`, `mangle_validation_test` (policy corpus), feedback/synth/transpiler suites, fuzz for parse atoms. Torture and concurrent parse tests present. |
| **Wiring completeness** | **4** | Kernel, shards, browser honeypot, perception taxonomy, transparency explainer, world LSP, system factory all import. Some surfaces (SIMD intersect, intent_routing.mg load path) need wiring audit before “primary path” claims. |

**Weighted overall: ~4.0 / 5** — production substrate, not a stub. Differential path and some tooling remain partial relative to the hollow-kernel vision.

## Alignment narrative

codeNERD’s inversion of control requires a **trusted Mangle runtime**. This package is that runtime’s library layer:

1. **Creative work** (LLM) emits rules/facts as text or `mangle_synth_v1` JSON.
2. **Transduction gates** (feedback, sanitizer, schema validator, GCD) reject or repair unsafe syntax and undeclared predicates.
3. **Executive evaluation** (Engine / kernel-hosted eval) derives `next_action` / `permitted` under gas and Decl constraints.

Failures of alignment would look like: learned rules defining `permitted`, unbounded inference, concurrent parse races, or raw LLM text bypassing validation. The first three are actively mitigated; bypass of validation is a **call-site discipline** issue (shards must use FeedbackLoop / HotLoad paths).

## Anti-alignment risks (watch list)

| Risk | Mitigation status |
|------|-------------------|
| Diff path silently drops external predicates | Kernel falls back to full eval when externals present (`kernel_eval.go`) |
| Sanitizer uses `parse.Unit` directly (not `ParseUnit`) | Race risk if concurrent with other parsers — see OPEN-QUESTIONS |
| Regex Decl loading misses exotic Decl forms | Prefer `UpdateFromProgramInfo` / analysis when available |
| intent_routing.mg vs core policy duplication | Treat as supplemental; verify load site before relying |
