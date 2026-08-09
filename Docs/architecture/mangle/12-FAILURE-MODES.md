# 12 — Failure Modes: mangle

> Last verified: 2026-07-13  
> Concrete failures, symptoms, mitigations.

## FM1 — Analysis failure on policy merge

| | |
|--|--|
| **Symptom** | Kernel boot or rebuild fails; `debug_program_ERROR.mg` written |
| **Causes** | Duplicate Decl, syntax error in learned rules, schema/policy inconsistency |
| **Mitigation** | `mangle_validation_test` / `nerd mangle-check`; SchemaValidator before learn; inspect dump |

## FM2 — Fact limit exceeded

| | |
|--|--|
| **Symptom** | `fact limit exceeded: N` on AddFacts |
| **Causes** | Unbounded EDB growth, missing ReplaceFactsForFile |
| **Mitigation** | Raise FactLimit carefully; file-scoped replace; Clear/session hygiene; watch 85% warn |

## FM3 — Derived facts / gas explosion

| | |
|--|--|
| **Symptom** | Eval error mentioning limit; memory pressure; slow OODA |
| **Causes** | Recursive learned rules, missing stratification, high fan-out joins |
| **Mitigation** | DerivedFactsLimit; reject bad learned rules; disable bad learned; full path has 500k default |

## FM4 — Query timeout

| | |
|--|--|
| **Symptom** | `query execution timed out after …` |
| **Causes** | Expensive top-down query, large store, low timeout |
| **Mitigation** | Increase QueryTimeout; narrow query; ensure IDB pre-materialized via eval |

## FM5 — Undeclared predicate in generation

| | |
|--|--|
| **Symptom** | Feedback retries; CategoryUndeclaredPredicate; HotLoad fails |
| **Causes** | LLM invents preds; JIT list incomplete |
| **Mitigation** | Inject predicates; synth with known preds; UpdateFromProgramInfo |

## FM6 — Forbidden head attempt

| | |
|--|--|
| **Symptom** | ValidateLearnedRule error naming protected predicate |
| **Causes** | Model tries to grant permissions or spoof pipeline |
| **Mitigation** | Expected deny; improve prompts; never relax forbidden map lightly |

## FM7 — Stratification / cyclic negation

| | |
|--|--|
| **Symptom** | Analysis or eval error; feedback CategoryStratification |
| **Causes** | `!p` depends on `p` through cycle |
| **Mitigation** | Rewrite rules; positive binding generators; classifier guidance |

## FM8 — Parse race (historical / guarded)

| | |
|--|--|
| **Symptom** | Race detector hits in ANTLR adaptivePredict; flaky parse |
| **Causes** | Concurrent parse.Unit without parseMu |
| **Mitigation** | Always ParseUnit/ParseAtom; whole-module AST guard plus mixed ParseUnit/ParseAtom/sanitizer/synth race regression |

## FM9 — Diff path incorrect with externals

| | |
|--|--|
| **Symptom** | Rules using external predicates would miss callbacks |
| **Causes** | ApplyAtomDelta eval without WithExternalPredicates |
| **Mitigation** | Kernel detects externals and uses **full** path — do not remove this gate |

## FM10 — Encoding skew (string vs name)

| | |
|--|--|
| **Symptom** | Facts “missing” in queries; wrong constant type |
| **Causes** | Auto-atomizer vs strict StringType; kernel vs Engine conversion |
| **Mitigation** | Prefer ApplyAtomDelta + ToAtom for kernel; match Decl bounds |

## FM11 — Feedback budget exhaustion

| | |
|--|--|
| **Symptom** | Autopoiesis suspended; “session validation budget exhausted” |
| **Causes** | Many bad generations in one session |
| **Mitigation** | ResetBudget at session start; fix prompts; raise budget only with monitoring |

## FM12 — Snapshot memory blow-up

| | |
|--|--|
| **Symptom** | OOM or huge alloc on Snapshot |
| **Causes** | Full fact copy of large EDB |
| **Mitigation** | Limit snapshot use; future structural COW |

## FM13 — Sanitizer parse failure

| | |
|--|--|
| **Symptom** | Feedback falls back to unsanitized or retries; KernelDebug sanitizer failed |
| **Causes** | Input not parseable even after QuickFix |
| **Mitigation** | PreValidator QuickFix; synth mode; progressive prompts |

## FM14 — Persistence failure after in-memory success

| | |
|--|--|
| **Symptom** | ReplaceFactsForFile returns persist error; memory updated but disk not |
| **Causes** | Persistence backend error |
| **Mitigation** | Treat as failed operation; retry; check isNilPersistence typed-nil guard |

## Operator playbook (short)

1. Parse/analyze errors → open `debug_program_ERROR.mg` if present.  
2. Slow eval → check diff flag, gas logs, fact counts.  
3. Bad learned rules → schema validator + feedback errors, not manual store edits.  
4. Race flakes → ensure ParseUnit everywhere.  
5. Permission bugs → **not** mangle package first; check core `permitted` policy.
