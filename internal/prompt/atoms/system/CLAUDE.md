# system/ - System Shard Atoms

Prompts for Type S (System) permanent shards that provide OODA loop infrastructure.

## Files

| File | Shard | Purpose |
|------|-------|---------|
| `executive.yaml` | ExecutivePolicyShard | Strategy derivation, next_action |
| `router.yaml` | TactileRouterShard | Action-to-tool routing |
| `perception.yaml` | PerceptionFirewallShard | NL-to-intent transduction |
| `world_model.yaml` | WorldModelIngestorShard | Fact maintenance |
| `legislator.yaml` | LegislatorShard | Rule synthesis system prompt |
| `autopoiesis.yaml` | All system shards | Autopoiesis/learning prompts |
| `requirements_interrogator.yaml` | RequirementsInterrogator | Socratic questioning for clarification |

## Architecture

System shards run continuously, unlike ephemeral domain shards:

```text
User Input -> Perception -> Executive -> Constitution -> Router -> Tool
```

## Key Pattern

System atoms use `shard_types` with system identifiers:

```yaml
shard_types: ["/executive", "/system_executive"]
```

These atoms are JIT-compiled - no hardcoded fallback constants exist in Go code.


> *[Archived & Reviewed by The Librarian on 2026-01-25]*