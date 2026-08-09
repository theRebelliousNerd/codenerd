# 09 — Safety and Invariants: mangle

> Last verified: 2026-07-13

## Constitutional relationship

codeNERD safety is **logic-owned**:

- Core policy defines `permitted(...)` (default deny).
- This package **must not** let learned rules redefine permission or spoof pipeline predicates.
- This package **must** make evaluation resource-bounded and parse-race-free.

## Invariants

### I1 — Declared predicates only

**Statement:** Every inserted fact’s predicate is in `predicateIndex`; every learned rule body predicate is Declared or builtin.

**Enforcement:**

- `Engine.factToAtomLocked` error on unknown predicate.
- `SchemaValidator.ValidateRule` / `ValidateLearnedRule`.
- FeedbackLoop Phase 4 schema validation.

### I2 — Forbidden learned heads

**Statement:** Learned rules/facts cannot define protected control-plane predicates.

**Enforcement:** `forbiddenLearnedHeads` map in `schema_validator.go` (permitted, safe_action, admin_override, signed_approval, pending_action, permitted_action, permission_check_result, routing_result, execution_result, system_shard_state).

### I3 — Arity consistency

**Statement:** Argument counts match Decl.

**Enforcement:** Engine arity check; SchemaValidator `validateHeadArity`.

### I4 — Inference gas

**Statement:** Fixpoint evaluation cannot unbounded-explode memory via recursive rules.

**Enforcement:**

- Engine: `WithCreatedFactLimit(DerivedFactsLimit)`.
- Kernel full and differential paths:
  `internal/core/kernel_init.go#RealKernel.effectiveDerivedFactLimitLocked`
  resolves the same 500_000 default.
- DifferentialEngine: `evalOptions` forwards every positive configured limit to
  unified atom, legacy atom, and legacy fact evaluator calls.

**Regression:** `TestDifferentialEngine_DerivedFactsLimit` proves direct
fail-closed enforcement; `TestKernelEval_ZeroConfigDerivedFactLimitParity`
proves unset full/diff kernel ceilings match.

### I5 — EDB capacity

**Statement:** Fact store respects `FactLimit`.

**Enforcement:** `insertFactLocked` hard error; 85% warn once.

### I6 — Parse mutual exclusion

**Statement:** No concurrent mutation of ANTLR global prediction state.

**Enforcement:** `parseMu` around `ParseUnit` / `ParseAtom`; core, sanitizer,
synth, system adapters, and tests delegate. A whole-module AST source guard rejects
raw parser selectors outside `parse_lock.go`, and mixed callers pass under
`go test -race`.

### I7 — Schemas before use

**Statement:** Operations requiring `programInfo` fail with `errNoSchemas` if unloaded.

### I8 — Negation safety (generation)

**Statement:** Variables in negation should be bound by positive atoms; Sanitizer attempts generator injection; PreValidator flags unbound negation.

Mangle engine itself enforces stratification (no cyclic negation); classifier maps violations.

### I9 — Budget termination

**Statement:** Feedback generation terminates.

**Enforcement:** MaxRetries per prompt hash; SessionBudget; total/per-attempt timeouts.

### I10 — Diff invalidation on non-incremental ops

**Statement:** Retract, clear, policy rebuild cannot leave stale differential state.

**Enforcement:** Kernel `invalidateDiffEngineLocked`.

### I11 — Encoding consistency on kernel delta path

**Statement:** Atoms applied via `ApplyAtomDelta` use kernel `ToAtom`, not Engine auto-atomizer, avoiding silent type skew.

### I12 — Auto-eval is controllable

Bulk load paths can disable auto-eval (`ToggleAutoEval` / warm path) then recompute once — avoids N-1 partial evals.

## Concurrency safety

| Object | Lock | Notes |
|--------|------|-------|
| Engine | RWMutex | Writers for mutate; Query takes RLock then eval outside carefully |
| DifferentialEngine | RWMutex | Stratum KG has own mutex |
| parseMu | Mutex | Global |
| ValidationBudget | Mutex | |
| LSP / ProofTree | RWMutex | |

## What this package does **not** guarantee alone

| Concern | Owner |
|---------|-------|
| User action permission | core `permitted` rules |
| Tool sandbox / filesystem policy | VirtualStore + policy |
| Prompt injection | perception + prompt atoms |
| Authenticity of admin overrides | human/admin channels |

## Failure closed examples

| Input | Result |
|-------|--------|
| Fact for undeclared pred | Error, not silent insert |
| Learned `permitted(...) :- ...` | SchemaValidator reject |
| Cyclic negation rule | Analysis/eval error; feedback CategoryStratification |
| Session budget exhausted | Autopoiesis skips; no infinite retry |
| Diff + external predicates | Kernel full path (not silent wrong answers) |
