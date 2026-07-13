# core — Mangle Surface

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/core/` (78 non-test .go, 107 tests, 129 .mg)**


## Local / owned Mangle

| Path | Lines |
|------|------:|
| `internal/core/debug_program_ERROR.mg` | 17733 |
| `internal/core/defaults/schema/intent_campaign.mg` | 1203 |
| `internal/core/defaults/schema/intent_queries.mg` | 1152 |
| `internal/core/defaults/campaign_rules.mg` | 922 |
| `internal/core/defaults/schemas_shards.mg` | 635 |
| `internal/core/defaults/schema/intent_operations.mg` | 633 |
| `internal/core/defaults/reviewer.mg` | 610 |
| `internal/core/defaults/taxonomy.mg` | 568 |

## Global defaults (kernel)

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Guardrails

- Decl before use; `/atoms`; Upper variables; safe negation; `|>` aggregation
- See skill `mangle-programming` and `internal/mangle/agents.md` when present
