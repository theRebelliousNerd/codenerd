# protocol/ - Communication Protocol Atoms

Defines how agents communicate with the kernel and format their outputs.

## Files

| File | Protocol | Purpose |
|------|----------|---------|
| `piggyback.yaml` | Piggyback Protocol | Dual-channel output (surface + control packet) |
| `reasoning_trace.yaml` | Reasoning Traces | Step-by-step reasoning documentation |
| `shard_output_translation.yaml` | Output Translation | Cross-shard result formatting |

## Piggyback Protocol

The core communication protocol for all shards:

```json
{
  "surface_response": "Human-readable response",
  "control_packet": {
    "task_status": "/completed",
    "facts_to_assert": [...],
    "facts_to_retract": [...]
  }
}
```

## Usage

Protocol atoms are typically **mandatory** and included for all shard types that interact with the kernel.
