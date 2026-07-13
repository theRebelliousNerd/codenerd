# 07 — Dependency Map: mangle

> Last verified: 2026-07-13  
> Evidence from imports of `codenerd/internal/mangle` and subpackages.

## Upstream (this package depends on)

```
internal/mangle
  ├── codeberg.org/TauCeti/mangle-go/analysis
  ├── codeberg.org/TauCeti/mangle-go/ast
  ├── codeberg.org/TauCeti/mangle-go/engine
  ├── codeberg.org/TauCeti/mangle-go/factstore
  ├── codeberg.org/TauCeti/mangle-go/parse
  ├── codeberg.org/TauCeti/mangle-go/unionfind
  ├── codeberg.org/TauCeti/mangle-go/builtin   (side-effect import)
  ├── codeberg.org/TauCeti/mangle-go/packages  (side-effect import)
  ├── codenerd/internal/logging
  ├── codenerd/internal/types          (engine)
  └── stdlib: sync, context, reflect, ...

internal/mangle/feedback
  ├── codenerd/internal/config         (LLM timeouts)
  ├── codenerd/internal/logging
  ├── codenerd/internal/mangle/synth
  ├── codenerd/internal/mangle/transpiler
  └── mangle-go/analysis

internal/mangle/transpiler
  ├── codenerd/internal/mangle         (AtomValidator)
  └── mangle-go/{analysis,ast,parse}

internal/mangle/synth
  └── mangle-go/{analysis,ast,parse}
```

**Does not import:** `internal/core` (avoids cycles). Kernel depends on mangle, not vice versa.

## Downstream (who imports mangle)

### Core runtime

| Importer | Symbols used (typical) |
|----------|------------------------|
| `internal/core/parse_serial.go` | `ParseUnit` |
| `internal/core/kernel_eval.go` | `NewEngine`, `NewDifferentialEngine`, `EnableUnifiedFastPath`, `ApplyAtomDelta`, `CopyAllFactsTo` |
| `internal/core/kernel_init.go` | `NewSchemaValidator` |
| `internal/core/kernel_validation.go` | Schema validation paths |
| `internal/core/kernel_policy.go` | `feedback.NewFeedbackLoop` |
| `internal/core/kernel_types.go` | Type refs |
| `internal/core/trace.go` | Tracing integration |
| `internal/core/defaults/policy/*_test.go` | `NewEngine` for policy logic tests |

### System shards

| Importer | Role |
|----------|------|
| `internal/shards/system/executive.go` | FeedbackLoop ownership + budget reset |
| `internal/shards/system/executive_autopoiesis.go` | GenerateAndValidate for learned rules |
| `internal/shards/system/constitution.go` | FeedbackLoop budget gates |
| `internal/shards/system/legislator.go` | Feedback + synth |
| `internal/shards/system/mangle_repair.go` | Feedback + synth repair |

### Other internal

| Importer | Role |
|----------|------|
| `internal/autopoiesis` | Engine + sanitizer for ouroboros |
| `internal/browser` | Engine for honeypot / session DOM |
| `internal/perception` | Taxonomy / transducer |
| `internal/transparency` | Explainer / proof |
| `internal/world/lsp` | LSPServer manager |
| `internal/system/factory.go` | Construction wiring |
| `internal/prompt/predicate_selector.go` | Implements FeedbackLoop selector interface |

### CLI

| Importer | Role |
|----------|------|
| `cmd/nerd/cmd_mangle_check.go` | Corpus check |
| `cmd/nerd/cmd_mangle_lsp.go` | LSP entry |
| `cmd/nerd/cmd_query.go` | Query path |
| `cmd/nerd/cmd_browser.go` | Browser cmds |
| `cmd/nerd/ui/splitpane.go` | UI may surface mangle types |
| `cmd/nerd/chat/*` | Types / testutil |

## Dependency direction diagram

```
cmd/nerd ──► core ──► mangle ──► mangle-go
   │           │         ▲
   │           │         │
   │           ├──► feedback ──► synth, transpiler
   │           │
   ├──► shards/system ──► feedback, synth
   ├──► browser ──► mangle
   ├──► autopoiesis ──► mangle, transpiler
   └──► world/lsp ──► mangle
```

## Cycle risks

| Risk | Status |
|------|--------|
| mangle → core | **Avoided** (no import) |
| feedback → core | **Avoided** |
| transpiler → feedback | **Avoided** (feedback → transpiler only) |
| synth → feedback | **Avoided** (feedback → synth only) |

## Feature-flag dependency

Differential usage depends on `codenerd/internal/features` **from core**, not from mangle itself. Mangle always offers DifferentialEngine; kernel decides when to call it.
