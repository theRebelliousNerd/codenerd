# core — Safety and Invariants

> Last verified: **2026-07-13**  
> Deep Mangle Decl surface: [13-MANGLE-SURFACE.md](13-MANGLE-SURFACE.md)

## 1. Safety stack (ordered)

```
1. Boot guard          — no RouteAction until first user interaction
2. Dreamer             — speculative panic_state (destructive only)
3. Go constitution      — hard string/path rules
4. Mangle permitted    — default deny executive policy
5. Binary/env allowlist— tactile exec constraints
6. Post validators     — did the effect actually succeed / stay valid?
7. Result facts        — security_violation / execution_error for later rules
```

Missing earlier layer must not be “compensated” by hoping a later layer exists.

## 2. Constitutional Mangle (default deny)

`defaults/policy/constitution.mg`:

- `permitted(Action, Target, Payload)` requires `safe_action(Action)` and pending envelope, with `!dangerous_content` on payload **and** target.  
- Dangerous actions need `signed_approval` + `admin_override`.  
- Executor bridge via `permitted_action` + `permission_check_result(..., /permit, ...)`.  
- `permission_denied` rules document deny reasons for dangerous actions without override.

Schema: `Decl permitted(ActionType, Target, Payload)` in `defaults/schemas_safety.mg`.

## 3. Go constitution rules

`virtual_store_constitution.go`:

| Rule | Blocks |
|------|--------|
| `no_destructive_commands` | `rm -rf`, `mkfs`, `dd if=`, fork bombs, `chmod 777` in exec targets |
| `no_secret_exfiltration` | secret-like payload tokens + network exfil binaries |
| `path_traversal_protection` | `..` after Clean; symlink best-effort |
| `no_system_file_modification` | `/etc`, `/usr`, Windows system paths |

These are **not** a substitute for allowlists or Dreamer.

## 4. Dreamer invariants

| Invariant | Enforcement |
|-----------|-------------|
| Fail closed | Nil kernel/context, eval fail, missing Decl → Unsafe |
| Critical paths | Hardcoded prefixes asserted as `critical_path_prefix` |
| Critical files | Policy `critical_file("go.mod")` etc. |
| Cache coherence | Invalidate on kernel/policy change (caller duty) |
| Destructive set | `isDestructiveAction` must stay complete for new verbs |

## 5. Boot invariants

| Invariant | Mechanism |
|-----------|-----------|
| Constitution compiles or process fails boot | `NewRealKernel` evaluate error |
| No ephemeral action replay at boot | `filterBootFacts` / `IsEphemeral` |
| No effect routing until ready | `bootGuardActive` |
| Learned cannot outrank constitution | Concat order in `rebuildProgram` |

## 6. EDB / eval invariants

| Invariant | Mechanism |
|-----------|-----------|
| Deduped facts | `factIndex` canon keys |
| Hard EDB ceiling | `maxFacts` default 250k |
| Derived fact ceiling | default 500k |
| Retract invalidates diff engine | Force full rebuild |
| Provenance off by default | Memory bound |
| Dirty query correctness | `ensureEvaluated` before Query |

## 7. Concurrency invariants

| Invariant | Notes |
|-----------|-------|
| Kernel maps protected by `mu` | Readers use RLock |
| `factsDirty` atomic | Fast path without lock when clean |
| Singleflight eval | Avoid stampedes |
| Cortex lock order | cortex.mu then shard ops without nested inversion (comments in code) |
| VS long I/O | Prefer unlock during executor.Execute |

## 8. Mangle authoring invariants (core-owned corpus)

From repo Mangle guardrails (apply to all `defaults/**/*.mg`):

1. Every predicate needs `Decl` before use.  
2. Variables uppercase; atoms `/lowercase`.  
3. Negation only on positively bound variables (no `!foo(_, _)` anonymous traps).  
4. Aggregations use pipeline syntax.  
5. Core additionally: `checkUnsafeNegation` rejects `!pred(..._)` patterns on hot-load.

## 9. Allowlist invariants

Default VS config (`DefaultVirtualStoreConfig`):

- Env: `PATH`, `HOME`, `GOPATH`, `GOROOT`  
- Binaries: shells + go/git/node/python/cargo/make family  

Caller-provided env is filtered (`filterCallerEnv`) so duplicate keys cannot smuggle `LD_PRELOAD`-class overrides past allowlist intent.

## 10. Transaction / shadow invariants

- Transaction manager aborts active tx on VS `Close`.  
- Shadow mode simulations must not commit effects without explicit approval path (CLI/e2e).  
- Dreamer projections are ephemeral clones — never write projected facts into the live kernel permanently as “real.”

## 11. Security-relevant fact predicates

| Predicate | Meaning |
|-----------|---------|
| `security_violation` | Gate denied or constitution failed |
| `dream_blocked_action` | Dreamer blocked |
| `execution_error` | Handler failure |
| `execution_result` | Outcome log |
| `panic_state` | Speculative hazard (IDB) |
| `permitted` | Allow derivation |

Downstream policy can escalate on repeated violations (`system_core.mg` family references anomaly patterns).

## 12. Threat model (core-centric)

| Threat | Mitigations |
|--------|-------------|
| LLM emits `rm -rf /` | constitution + dreamer + binary path + allowlist |
| LLM writes SSH keys out via curl | secret exfil rule |
| Path `../../.ssh` | traversal Clean + workspace discipline |
| Poisoned learned.mg | sandbox HotLoad, heal, interceptor |
| Stale session next_action | boot guard + ephemeral filter |
| Policy author Decl mistake | analyze fail + debug dump at boot |
| Resource exhaustion via assert flood | maxFacts / derived limits / spawn limits |

## 13. Explicit non-goals

- Full multi-tenant isolation inside one process  
- Cryptographic attestation of every tool call  
- Replacing OS permissions  

Core is a **logic executive**, not a hypervisor.
