# internal/shards/nemesis - Adversarial Patch Analysis

This package implements the NemesisShard - a persistent specialist (Type B) that performs system-level adversarial analysis, actively trying to break patches before they are merged.

**Related Packages:**
- [internal/shards](../CLAUDE.md) - Shard registration and interfaces
- [internal/autopoiesis](../../autopoiesis/CLAUDE.md) - Ouroboros loop consuming attack tools
- [internal/regression](../../regression/CLAUDE.md) - Battery consuming Armory attacks

## Architecture

The Nemesis shard follows the adversarial co-evolution philosophy:
- **"The Nemesis does not seek destruction - it seeks truth"**
- Acts as a hostile sparring partner for the Coder Shard
- Generates compilable attack binaries (not just inputs like PanicMaker)
- Persists successful attacks in the Armory for regression testing

### The Gauntlet

A patch is only "battle-hardened" if it survives the Nemesis:
1. Analyze change for attack surface and assumptions
2. Generate targeted attack tools via Ouroboros
3. Execute attacks in sandboxed Thunderdome
4. Persist successful attacks to Armory for future regression

## File Index

| File | Description |
|------|-------------|
| `nemesis.go` | Core `NemesisShard` implementing adversarial patch analysis with VulnerabilityDB tracking. Exports `NemesisShard`, `VulnerabilityDB` (SuccessfulAttacks/FailedAttacks/LazyPatterns/HardenedAreas), `NemesisAnalysis`, `ChangeAnalysis`, and `AttackSpec` for Ouroboros-driven attack generation. |
| `armory.go` | Persistence layer for successful attack tools enabling regression testing. Exports `Armory`, `ArmoryAttack` (Name/Category/Vulnerability/Specification/BinaryPath/SuccessCount), `ArmoryStats`, `AddAttack()`, and `GetAttacksForCategory()` to retrieve attacks by type. |
| `attack_runner.go` | Sandboxed executor for generated attack scripts that probe for weaknesses. Exports `AttackRunner`, `AttackScript` (Name/Category/TargetFile/TestCode/Hypothesis), `AttackExecution` (Success/BreakageType/Output/Duration), and `RunAttack()` compiling and executing Go test attacks. |

## Key Types

### NemesisShard
```go
type NemesisShard struct {
    *BaseShardAgent
    vulnerabilityDB *VulnerabilityDB
    armory          *Armory
}
```

### AttackSpec
```go
type AttackSpec struct {
    Name        string   // Attack tool name
    Category    string   // concurrency, resource, logic, integration
    Hypothesis  string   // What invariant we expect to break
    TargetFiles []string // Files to attack
    Inputs      []string // Malicious inputs
}
```

### AttackExecution
```go
type AttackExecution struct {
    Script       *AttackScript
    Success      bool          // true = attack found a bug
    BreakageType string        // panic, timeout, assertion, race
    Output       string
    Duration     time.Duration
    ExitCode     int
}
```

## Attack Categories

| Category | Description |
|----------|-------------|
| `concurrency` | Race conditions, deadlocks, goroutine leaks |
| `boundary` | Max int, negative indices, empty slices |
| `resource` | OOM triggers, file handle exhaustion |
| `logic` | Invariant violations, state corruption |
| `integration` | Cross-component interaction failures |

## Dependencies

- `internal/core/shards` - BaseShardAgent
- `internal/articulation` - PromptAssembler for JIT identity
- `internal/regression` - Battery task suites
- `internal/build` - Build environment for attack compilation

## Testing

```bash
go test ./internal/shards/nemesis/...
```
