# mangle wiring — and what is NOT built

Verified 2026-09-21 against commit `0f7253b`. "Wired" means a Go caller exists
and is cited; "import only" means the import statement is the whole
relationship. The third section names what the design implies but the code
does not do — that is the part of this file most likely to save debugging time.

## Wired and reachable

- `internal/core` — the primary consumer. `RealKernel.GenerateValidatedRule`
  (`internal/core/kernel_policy.go:247-331`) builds the prompts, creates the
  loop (`feedback.NewFeedbackLoop`, 294), and passes itself as the validator
  (298-305). `RealKernel.HotLoadRule` (`kernel_policy.go:180-233`)
  VALIDATES a single rule in a memory-only sandbox kernel
  (`validateRuleSandbox`, 220) and installs nothing — the 160-179 comment
  says the old name promised loading and means it. Installation plus
  persistence is `RealKernel.HotLoadLearnedRule` (`kernel_policy.go:360-441`):
  repair interceptor, normalize, sandbox validate, schema check, loop check,
  append to `learned.mg`.
- `internal/shards/system` — the callers that install learned rules:
  `constitution.go:842`, `executive_autopoiesis.go:152`,
  `legislator.go:227`, all through `Kernel.HotLoadLearnedRule`. The repair
  shard exists at `internal/shards/system/mangle_repair.go` and is named as
  the "Phase 1: Syntax check via kernel" caller in the `HotLoadRule` contract
  (`kernel_policy.go:167-169`).
- `cmd/nerd` — `nerd why` has two adjacent paths in `cmd_query.go`: the
  kernel trace (`kern.TraceQuery`, 227) and a fresh-engine
  `ProofTreeTracer.TraceQuery` (308-310). The chat lifecycle also traces
  through the kernel (`cmd/nerd/chat/model_lifecycle.go:233-244`).
- `internal/world/lsp` — `Manager.Initialize` builds an engine and an
  `LSPServer` and indexes the workspace once at startup
  (`internal/world/lsp/manager.go:117-128`) through `IndexWorkspace`
  (`lsp.go:676-715`), which opens every `.mg` file via `OpenDocument` (call at
  710) and skips the `debug_program*.mg` dumps and unreadable files (699-707).
  Completion is delegated to this package's provider (`manager.go:250-270`);
  diagnostics pass through from it (`manager.go:286-303`). `ServeStdio`
  (`manager.go:368-380`) delegates to the stdlib server; the only in-repo
  reference to driving it is the package README
  (`internal/world/lsp/README.md:71`).
- `internal/autopoiesis`, `internal/browser`, `internal/context` — import the
  package (e.g. `internal/autopoiesis/ouroboros.go:77`,
  `internal/context/working_set.go:19`) without owning any of its lifecycles.

## Exists but nothing calls

- `LSPServer`'s stored engine handle: `NewLSPServer` takes an `*Engine` and
  stores it (`lsp.go:26,105-107`), and no method in the file reads it —
  indexing is pure regex over text, and hover/completion/definition answers
  come from the snapshot maps, never from a live evaluation. Contrast
  `OpenDocument` (`lsp.go:133-147`), which IS exercised: `IndexWorkspace`
  opens every `.mg` file through it (call at 710), and the
  `didOpen`/`didChange` notification handlers (835,852) route through it.
- The stdio server itself (`lsp.go:722-1018`): built, reachable only through
  stdin/stdout; no in-repo driver beyond the package README example.
- `Engine.Evaluate` (`engine.go:550-558`) outside tests: the session reaches
  evaluation through fact insertion and `ReplaceControlFacts`, not through the
  named operation (the `engine.go:544-549` comment says exactly this).
- `ProofTreeTracer.MaterializeToFacts` (`proof_tree.go:352-375`): the
  `derivation_trace` / `proof_tree_node` reification has no production caller;
  `nerd why` renders the trace directly.
- `RepairLoop.ValidateAndRepair` (`grammar.go:744-766`) is the documented
  repair path, but the production repair traffic runs through
  `feedback/autocorrect.go:FixRule` inside `GenerateAndValidate`; the loop
  type itself is exercised by tests.

## Assumed but not done by the code

- **Hot-load means validate.** Any reader coming from the function name
  `HotLoadRule` expects the rule to be live afterwards. Both the
  engine-native (`schema_validator.go:212-224`) and kernel
  (`kernel_policy.go:180-233`) versions validate and discard; only
  `HotLoadLearnedRule` (`kernel_policy.go:360-441`) installs and persists.
  A nil error from the former says "acceptable", never "installed".
- **The LSP knows syntax, not semantics.** Hover, completion, and
  jump-to-definition reflect the last index snapshot; they cannot report
  derived facts, rule firing, or arity errors the validator would catch,
  because the server never queries the engine it holds.
- **Learning new predicates is legitimate; learning new control is not.**
  `ValidateLearnedRule` lets a learned rule derive a fresh predicate
  (`schema_validator.go:193-199`) but refuses protected control-plane heads
  (174-177). The package assumes the `forbiddenLearnedHeads` set is complete;
  nothing in the package checks that assumption — a missing entry is a silent
  promotion path from learned rule to control plane.
- **No admission control on facts.** The validator gates rules; `AddFact`
  gates types (`convertValueToTypedTerm`, `engine.go:735-864`). Nothing gates
  volume per source or predicate — the gas limit (`engine.go:220-267`) and
  the fact-count warning (`maybeWarnFactLimit`, `engine.go:658-670`) bound
  derivation cost, not store growth.
- **One parser at a time.** `parse_lock.go` serializes all parser entry. The
  package assumes every load path goes through the guard; a future caller
  that invokes the upstream parser directly bypasses the only thread-safety
  the package has.

## Reachability ledger

| Caller | Path | Status |
|---|---|---|
| `internal/core` | `GenerateValidatedRule` → `GenerateAndValidate`; `HotLoadRule` sandbox-validate; `HotLoadLearnedRule` install+persist | wired |
| `internal/shards/system` | `constitution.go:842`, `executive_autopoiesis.go:152`, `legislator.go:227` → `HotLoadLearnedRule` | wired |
| `cmd/nerd` | `cmd_query.go:227` + `:308-310`; `chat/model_lifecycle.go:233-244` | wired |
| `internal/world/lsp` | `manager.go:117-128` init + index; `:368-380` stdio passthrough | wired, read-only |
| `internal/autopoiesis`, `browser`, `context` | imports only (`ouroboros.go:77`, `working_set.go:19`) | import only |
| `OpenDocument`, stdio server, stored engine handle | `lsp.go:26,224-280,840-1018` | built, not exercised |
| `MaterializeToFacts` reification | `proof_tree.go:352-375` | built, no production caller |
