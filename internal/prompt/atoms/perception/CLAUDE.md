# perception/ - NL Transduction Atoms

Guidance for natural language to intent transduction.

## Files

| File | Purpose |
|------|---------|
| `transducer.yaml` | NL parsing and intent extraction |
| `understanding.yaml` | Semantic understanding patterns |

## Transduction Pipeline

```
User Input (NL)
    |
    v
Intent Extraction (verb, target, constraints)
    |
    v
Focus Resolution (ground to concrete paths/symbols)
    |
    v
Confidence Scoring
    |
    v
user_intent fact
```

## Usage

Perception atoms guide the PerceptionFirewallShard in:
- Classifying user intent
- Resolving ambiguous references
- Calculating confidence scores
- Triggering clarification when needed
