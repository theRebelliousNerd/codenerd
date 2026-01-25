# campaign/ - Multi-Phase Campaign Atoms

Orchestration guidance for long-running, multi-phase campaigns.

## Files

| File | Purpose |
|------|---------|
| `planner.yaml` | Goal decomposition and task planning |
| `analysis.yaml` | Campaign analysis and progress tracking |
| `assault.yaml` | Assault campaign (aggressive implementation) |
| `extractor.yaml` | Knowledge extraction from campaigns |
| `librarian.yaml` | Campaign knowledge curation |
| `taxonomy.yaml` | Task categorization |
| `replanning.yaml` | Adaptive replanning on failure |
| `phases_encyclopedia.yaml` | All campaign phase definitions |

## Subdirectories

| Directory | Purpose |
|-----------|---------|
| `analysis/` | Detailed analysis atoms |
| `extractor/` | Extraction methodology, hallucination guards |
| `librarian/` | Edge cases, philosophy, protocols |
| `planner/` | Decomposition and validation atoms |
| `replanning/` | Replanning strategies |
| `taxonomist/` | Classification rules |

## Campaign Phases

1. **Planning** - Goal decomposition, task identification
2. **Execution** - Task-by-task implementation
3. **Review** - Quality verification
4. **Integration** - Final assembly and testing

## Selection

Campaign atoms are selected via `campaign_phase`:

```yaml
campaign_phases: ["/planning", "/execution"]
```


> *[Archived & Reviewed by The Librarian on 2026-01-25]*