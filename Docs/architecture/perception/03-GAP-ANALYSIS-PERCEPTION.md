# perception — Gap Analysis

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/perception/` (49 non-test .go, 47 tests, 0 .mg)**


## Spec vs reality

| Area | Status | Notes |
|------|--------|-------|
| Package exists on disk | Yes | `internal/perception/` |
| Primary types present | Yes | Sampled exports |
| Tests present | Yes | 47 test files |
| Mangle local sources | N/A or global only | 0 .mg |
| Wiring registration | Check parent | shards/registration, VirtualStore, cmd hooks |

## Gaps (ordered by gate, not calendar)

1. **Documentation depth** — deepen deep-dives when this package is under active design.
2. **Wiring proof** — ensure registration/routes/tests cover public entrypoints.
3. **Mangle Decl hygiene** — any new predicates require Decl + policy review.
4. **Prompt-atom discipline** — LLM-facing changes should land as atoms when applicable.

## Non-gaps

Living implementation exists; do not treat this corpus as 0% pre-implementation.
