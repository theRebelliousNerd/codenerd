# internal/campaign - Multi-Phase Campaign Orchestration

This package implements long-running campaign orchestration for complex, multi-phase development tasks.

## Architecture

Campaigns break down large goals into phases and tasks, managing execution across multiple shard invocations with progress tracking and rollback capabilities.

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `orchestrator.go` | ~920 | Main campaign execution engine |
| `decomposer.go` | ~700 | Goal → Phase → Task breakdown |
| `types.go` | ~200 | Campaign, Phase, Task types |

## Key Types

### Campaign
```go
type Campaign struct {
    ID          string
    Title       string
    Goal        string
    Type        CampaignType
    Status      CampaignStatus
    Phases      []Phase
    Progress    Progress
    CreatedAt   time.Time
}
```

### Phase
```go
type Phase struct {
    ID          string
    Name        string
    Description string
    Tasks       []Task
    Status      PhaseStatus
    Order       int
}
```

### Task
```go
type Task struct {
    ID          string
    Description string
    ShardType   string
    Status      TaskStatus
    Result      string
    Attempts    int
}
```

### CampaignType
| Type | Description |
|------|-------------|
| CampaignTypeFeature | New feature implementation |
| CampaignTypeGreenfield | New project/module |
| CampaignTypeMigration | Code/data migration |
| CampaignTypeAudit | Security/quality audit |
| CampaignTypeRemediation | Bug fixes, tech debt |

## Orchestrator Flow

1. **Decompose**: Goal → LLM → Phases & Tasks
2. **Execute**: Phase-by-phase, task-by-task
3. **Track**: Progress, results, failures
4. **Checkpoint**: Save state for resume
5. **Complete**: Aggregate results

## Decomposer

Uses LLM to break down goals:
```go
decomposer.Decompose(ctx, goal, campaignType) → []Phase
```

Considers:
- Project context
- File structure
- Complexity estimates
- Dependencies between tasks

## Progress Tracking

```go
type Progress struct {
    TotalPhases    int
    CompletedPhases int
    TotalTasks     int
    CompletedTasks int
    CurrentPhase   string
    CurrentTask    string
}
```

## Pause/Resume

Campaigns can be paused and resumed:
```go
orchestrator.Pause(campaignID)
orchestrator.Resume(campaignID)
```

State is checkpointed to `.nerd/campaigns/`.

## Dependencies

- `internal/core` - ShardManager for task execution
- `internal/perception` - LLMClient for decomposition
- `internal/store` - Campaign persistence

## Testing

```bash
go test ./internal/campaign/...
```
