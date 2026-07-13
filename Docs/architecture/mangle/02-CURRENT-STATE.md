# 02 — Current State: mangle

> Last verified: 2026-07-13  
> Precise inventory of `internal/mangle/` as implemented.

## Package layout

```
internal/mangle/
├── engine.go                 # Core Engine wrapper (~1100 LOC)
├── differential.go           # DifferentialEngine + FactStoreProxy (~866)
├── grammar.go                # AtomValidator + RepairLoop (~787)
├── lsp.go                    # LSPServer (~1055)
├── proof_tree.go             # ProofTreeTracer (~482)
├── schema_validator.go       # SchemaValidator (~412)
├── parse_lock.go             # ParseUnit / ParseAtom (~44)
├── simd_intersect_amd64.go   # SIMD intersect (build-tagged)
├── simd_intersect_generic.go # Portable fallback
├── intent_routing.mg         # Intent → action/persona rules
├── *_test.go                 # Engine, diff, grammar, lsp, validation, torture, …
├── feedback/
│   ├── types.go              # Categories, budgets, protocols
│   ├── loop.go               # FeedbackLoop.GenerateAndValidate
│   ├── pre_validator.go      # Regex AI-error detection + QuickFix
│   ├── error_classifier.go   # Compiler error → structured feedback
│   ├── prompt_builder.go     # Progressive retry prompts
│   └── normalize.go          # Rule extraction / normalization
├── synth/
│   ├── spec.go / schema.go   # mangle_synth_v1 model + JSON schema
│   ├── compile.go / validate.go
│   ├── decoder.go            # Extract JSON / piggyback surfaces
│   └── README.md             # Design notes for tool path
└── transpiler/
    └── sanitizer.go          # Atom interning, agg repair, safety injection
```

## Scale (approximate)

| Class | Count | Notes |
|-------|------:|-------|
| Non-test Go sources (all subpackages) | **~21** | Root + feedback + synth + transpiler |
| Test Go files | **~39** | Includes benchmarks, fuzz |
| Local `.mg` | **1** | `intent_routing.mg` |
| Largest single file | **engine.go ~1100** | Fact conversion + query |
| Second | **lsp.go ~1055** | Full LSP protocol surface |
| Third | **differential.go ~866** | Incremental eval |

## Component status matrix

| Component | Status | Primary types | Used by |
|-----------|--------|---------------|---------|
| `Engine` | **Production** | `Engine`, `Config`, `Fact`, `Persistence` | Kernel (diff build), policy tests, browser, perception, CLI |
| Stratified eval + gas | **Production** | `evalWithGasLimit` | Engine auto-eval / recompute |
| `DifferentialEngine` | **Production (opt-in)** | `DifferentialEngine`, `ChainedFactStore` | Kernel `evaluateDiff`, ouroboros, torture tests |
| Unified fast path | **Production (opt-in)** | `EnableUnifiedFastPath` | Kernel prefers this after `NewDifferentialEngine` |
| `ParseUnit` lock | **Production** | `parseMu` | Engine + core `parseUnit` |
| `SchemaValidator` | **Production** | forbidden heads, arity | Kernel learned-rule gate, feedback `RuleValidator` |
| `FeedbackLoop` | **Production** | validate-retry | Executive, constitution, legislator, kernel_policy |
| `Sanitizer` | **Production** | transpiler | FeedbackLoop auto-repair |
| `synth` | **Production** | `Compile`, `DecodeSpec` | FeedbackLoop synth modes; mangle_repair / legislator |
| `AtomValidator` / GCD | **Production** | `RepairLoop` | Sanitizer; atom-level gates |
| `ProofTreeTracer` | **Implemented** | `DerivationTrace` | Transparency / explain paths |
| `LSPServer` | **Implemented** | docs, defs, diagnostics | `cmd/nerd` mangle-lsp; world LSP manager |
| SIMD intersect | **Implemented** | `IntersectSIMD` | Available utility; confirm call sites before perf claims |
| `intent_routing.mg` | **Source present** | rules | Validate via tests; confirm runtime load in core |

## Hotspots (by operational risk)

1. **`engine.factToAtomLocked` / auto-atomizer** — type coercion heuristics affect every fact assert path; kernel uses its own `types.Fact.ToAtom()` for diff deltas to avoid skew.
2. **`DifferentialEngine` dual path** — legacy per-stratum vs unified store; Snapshot/Query still oriented to strataStores.
3. **`FeedbackLoop` budgets** — session exhaustion suspends autopoiesis (by design); must reset at session start.
4. **`parseMu`** — global serialization; correctness over throughput.
5. **Sanitizer `parse.Unit`** — may bypass process lock (see gaps).

## Configuration knobs

| Knob | Location | Effect |
|------|----------|--------|
| `Config.FactLimit` | `engine.go` Default 100000 | Hard EDB insert cap |
| `Config.DerivedFactsLimit` | Default 100000 | Eval gas via `WithCreatedFactLimit` |
| `Config.QueryTimeout` | Default 30s | Query context timeout |
| `Config.AutoEval` | Default on unless `MANGLE_AUTO_EVAL=0` | Eval after inserts |
| `features.diff_eval` / `CODENERD_DIFF_EVAL` | `internal/features` + env | Kernel differential path |
| `feedback.RetryConfig` | MaxRetries 3, SessionBudget 20 | Generation loop limits |
| LLM timeouts | `config.GetLLMTimeouts()` | Per-attempt / total in DefaultConfig |

## What “done” means today

The package is **living production code** integrated into kernel boot and system shards. It is not a pre-implementation placeholder. Remaining work is primarily **diff-path completeness**, **parse-lock completeness**, and **unifying generation on synth** — not greenfield construction.
