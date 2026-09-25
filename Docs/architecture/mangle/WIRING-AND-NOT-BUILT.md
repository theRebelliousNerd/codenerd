# mangle wiring — and what is NOT built

Re-verified 2026-09-25 (lane A build-out) against the working tree; first
written 2026-09-21 against `0f7253b`. "Wired" means a Go caller exists and is
cited; "import only" means the import statement is the whole relationship. The
third section names what the design implies but the code does not do — that is
the part of this file most likely to save debugging time. The last section
records what this pass closed and the test that proves each.

## Wired and reachable

- `internal/core` — the primary consumer. `RealKernel.GenerateValidatedRule`
  (`internal/core/kernel_policy.go`) builds the prompts, creates the loop
  (`feedback.NewFeedbackLoop`), and passes itself as the validator.
  `RealKernel.HotLoadRule` VALIDATES a single rule in a memory-only sandbox
  kernel (`validateRuleSandbox`) and installs nothing — its comment says the
  old name promised loading and means it. Installation plus persistence is
  `RealKernel.HotLoadLearnedRule` (`kernel_policy.go:360`): repair interceptor,
  normalize, sandbox validate, learned-rule validation (`:414`), loop check,
  append to `learned.mg`.
- **Learned rules may narrow a grant, never widen one.** Learned-rule
  validation refuses three sets of heads, in every statement of the learned
  text (`SchemaValidator.learnedHeads`, `schema_validator.go:69`), not only the
  first one a regex sees:
  the static `forbiddenLearnedHeads` (`schema_validator.go:371`); the program's
  **grant path**, derived from the rules by a polarity walk back from
  `permitted` (`GrantPathOf`, `grant_path.go:92`) — every predicate whose facts
  can add a `permitted` fact, where a rule that requires a human's consent
  (`signed_approval`, `admin_override`) guards its other premises; and the host
  witnesses a control packet may not write either
  (`core.hostWitnessPredicates`, `internal/core/mangle_updates.go:271`, shared
  with `predicateAllowed`). The kernel derives the protection from its program
  as it stands at each validation (`RealKernel.learnedHeadProtectionLocked`,
  `internal/core/kernel_validation.go:79`, memoized by program hash in
  `GrantPathOfSource`), so a grant rule appended by `AppendPolicy` is covered.
  A program whose grant path cannot be parsed admits no learned rule
  (`ErrGrantPathUnknown`), and the boot-time heal of `learned.mg` judges
  existing rules by the same heads, without persisting a verdict it could not
  reach.
- `internal/shards/system` — the callers that install learned rules:
  `constitution.go:842`, `executive_autopoiesis.go:152`, `legislator.go:227`,
  all through `Kernel.HotLoadLearnedRule`. The repair shard exists at
  `internal/shards/system/mangle_repair.go` and is named as the "Phase 1:
  Syntax check via kernel" caller in the `HotLoadRule` contract.
- `cmd/nerd` — `nerd why` has two adjacent paths in `cmd_query.go`: the kernel
  trace (`kern.TraceQuery`) and a fresh-engine `ProofTreeTracer.TraceQuery`.
  The chat lifecycle also traces through the kernel
  (`cmd/nerd/chat/model_lifecycle.go`). `nerd check-mangle --eval` calls
  `Engine.Evaluate` (`cmd/nerd/cmd_mangle_check.go:106`).
- `internal/world/lsp` — `Manager.Initialize` builds an engine and an
  `LSPServer` and indexes the workspace once at startup
  (`internal/world/lsp/manager.go:117-128`) through `IndexWorkspace`, which
  opens every `.mg` file via `OpenDocument`. Completion is delegated to this
  package's provider; diagnostics project into `code_diagnostic` facts
  (`manager.go:241`). The stdio server is driven by `nerd mangle-lsp`
  (`cmd/nerd/cmd_mangle_lsp.go:107`, `:149` → `Manager.ServeStdio`,
  `manager.go:370`), registered in `cmd/nerd/main.go`.
- `internal/autopoiesis`, `internal/browser`, `internal/context` — import the
  package without owning any of its lifecycles.

## Exists but nothing calls

- `LSPServer`'s stored engine handle: `NewLSPServer` takes an `*Engine` and
  stores it (`lsp.go:105-107`), and no method reads it. The engine the manager
  passes is empty (`mangle.NewEngine(mangle.DefaultConfig(), nil)`,
  `manager.go:117`), so there is nothing for it to answer. Declined in this
  pass: semantic diagnostics need the workspace program, not this handle, and
  no rule reads `code_diagnostic`.
- `ProofTreeTracer.MaterializeToFacts` (`proof_tree.go:352`): the
  `derivation_trace` / `proof_tree_node` reification has no production caller,
  and no rule reads either predicate (`schemas_knowledge.mg` declares them;
  nothing in `internal/core/defaults` consumes them). Declined: materializing
  would feed nothing.
- `RepairLoop.ValidateAndRepair` (`grammar.go:744`) is exercised by tests
  only. Production repair runs through `feedback/autocorrect.go:FixRule`
  inside `GenerateAndValidate` and through the unrelated
  `MangleRepairShard.ValidateAndRepair` (`internal/shards/system/mangle_repair.go:220`).
  Declined: deleting it is a maintainer call.

## Assumed but not done by the code

- **Hot-load means validate.** Any reader coming from the function name
  `HotLoadRule` expects the rule to be live afterwards. Both the
  engine-native (`schema_validator.go`, `SchemaValidator.HotLoadRule`) and
  kernel (`kernel_policy.go`) versions validate and discard; only
  `HotLoadLearnedRule` installs and persists. A nil error from the former says
  "acceptable", never "installed".
- **The LSP knows syntax, not semantics.** Hover, completion, and
  jump-to-definition reflect the last index snapshot; diagnostics are
  per-line heuristics (`diagnoseLine`, `lsp.go:290`). They cannot report
  derived facts, rule firing, or arity errors the validator would catch.
- **No admission control on facts.** The validator gates rules; `AddFact`
  gates types (`convertValueToTypedTerm`, `engine.go`). Nothing gates volume
  per source or predicate — the gas limit and the fact-count warning
  (`maybeWarnFactLimit`) bound derivation cost, not store growth. Declined:
  per-source quotas are a design decision, and the kernel's derived-fact
  ceiling already fails a runaway evaluation closed.
- **One parser at a time.** `parse_lock.go` serializes all parser entry. A
  future caller that invokes the upstream parser directly would bypass it;
  `TestCodeUsesSerializedMangleParser` (`parse_lock_test.go:17`) and
  `TestProductionParserCallersShareSerializedEntryPoint`
  (`parse_callers_integration_test.go:14`) fail when one does.

## Closed in this pass (2026-09-25)

| Item | Was | Now | Proof |
|---|---|---|---|
| `forbiddenLearnedHeads` assumed complete | A hand list of seven heads; `appeal_granted`, `has_active_override`, `temporary_override`, `file_recoverable` and the world-model inputs of `safe_action` were learnable, and a learned `appeal_granted("a9", /delete_file, "learned", 1).` permitted every pending delete. A second statement in the learned text was never judged. | The grant path is derived from the rules; host witnesses are shared with the control-packet gate; every statement's head is judged. | `internal/core/learned_grant_path_test.go` (all but the narrowing guard fail with the protection disabled); `internal/mangle/grant_path_test.go` |
| stdio server "no in-repo driver" | stale | `nerd mangle-lsp` | `cmd/nerd/cmd_mangle_lsp.go:149` |
| `Engine.Evaluate` "outside tests, no caller" | stale | `check-mangle --eval` | `cmd/nerd/cmd_mangle_check.go:106` |

## Reachability ledger

| Caller | Path | Status |
|---|---|---|
| `internal/core` | `GenerateValidatedRule` → `GenerateAndValidate`; `HotLoadRule` sandbox-validate; `HotLoadLearnedRule` validate (grant path + host witnesses) + install + persist | wired |
| `internal/shards/system` | `constitution.go:842`, `executive_autopoiesis.go:152`, `legislator.go:227` → `HotLoadLearnedRule` | wired |
| `cmd/nerd` | `cmd_query.go` trace paths; `chat/model_lifecycle.go`; `cmd_mangle_check.go:106` (`Engine.Evaluate`); `cmd_mangle_lsp.go:149` (stdio) | wired |
| `internal/world/lsp` | `manager.go:117-128` init + index; `:370` stdio | wired, read-only |
| `internal/autopoiesis`, `browser`, `context` | imports only | import only |
| stored LSP engine handle | `lsp.go:105-107` | built, empty, not read (declined) |
| `MaterializeToFacts` reification | `proof_tree.go:352` | built, no production caller, no consumer (declined) |
| `RepairLoop.ValidateAndRepair` | `grammar.go:744` | tests only (declined) |
