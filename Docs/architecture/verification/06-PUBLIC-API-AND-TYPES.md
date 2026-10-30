# verification — Public API and Types

## Retained change representation contract

An observed change must disclose a named representation bound to its attempt,
path, before-state identity and after-state identity. For known text states,
that representation is a unified diff, including removals and whole-file
deletions. Initial admission and subsequent byte ranges refer to the same
retained diff; exhausting a postimage cannot establish coverage of removed
content. `Omitted` counts bytes of the named representation excluded from the
initial packet. Requests continue after its admitted prefix without repeating
ranges. A complete small diff requires no expansion.

Unknown preimages cannot be inferred from empty strings or current files. An
existing final text file with an unknown preimage supplies only a labeled
postimage; deleted content with an unknown preimage remains unavailable. Known
empty, absent, unknown and binary states remain distinct. Binary bytes are not
rendered as text or exposed by falling back from a binary diff marker to raw
content. Labels, markers and range framing share the configured packet budget.

Only retained observations are disclosed. Historical removals remain bound to
the observed final state: recreated deleted paths or subsequent workspace
changes invalidate applicability under the existing kernel policy. Evidence
requests neither acquire arbitrary paths nor re-execute the worker. Unavailable
detail and exhausted acquisition rounds remain unresolved rather than passing.

> Last verified: **2026-07-13**  
> Package: `codenerd/internal/verification`

## Package declaration

```go
package verification
// Package verification implements the quality-enforcing verification loop.
```

## Sentinel error

| Symbol | Value | Use |
|--------|-------|-----|
| `ErrMaxRetriesExceeded` | `"max retries exceeded - escalating to user"` | Caller should escalate UX; prefer `errors.Is` |

## Enumerations

### `QualityViolation` (`string`)

| Constant | JSON / string |
|----------|---------------|
| `MockCode` | `mock_code` |
| `PlaceholderCode` | `placeholder` |
| `HallucinatedAPI` | `hallucinated_api` |
| `IncompleteImpl` | `incomplete` |
| `HardcodedValues` | `hardcoded` |
| `EmptyFunction` | `empty_function` |
| `MissingErrors` | `missing_errors` |
| `FakeTests` | `fake_tests` |

### `CorrectiveType` (`string`)

| Constant | JSON / string |
|----------|---------------|
| `CorrectiveResearch` | `research` |
| `CorrectiveDocs` | `docs` |
| `CorrectiveTool` | `tool` |
| `CorrectiveDecompose` | `decompose` |

## Structs

### `CorrectiveAction`

```go
type CorrectiveAction struct {
    Type      CorrectiveType `json:"type"`
    Query     string         `json:"query"`
    Reason    string         `json:"reason"`
    ShardHint string         `json:"shard_hint,omitempty"`
}
```

File: `internal/verification/verifier.go` (~53).

### `ShardSelectionResult`

```go
type ShardSelectionResult struct {
    ShardType    string
    ShardName    string
    Reason       string
    Confidence   float64
    Alternatives []string
}
```

No JSON tags — internal selection carrier.

### `VerificationResult`

```go
type VerificationResult struct {
    Success           bool               `json:"success"`
    Confidence        float64            `json:"confidence"`
    Reason            string             `json:"reason"`
    Suggestions       []string           `json:"suggestions,omitempty"`
    Evidence          []string           `json:"evidence,omitempty"`
    QualityViolations []QualityViolation `json:"quality_violations,omitempty"`
    CorrectiveAction  *CorrectiveAction  `json:"corrective_action,omitempty"`
}
```

### `TaskVerifier`

```go
type TaskVerifier struct {
    // unexported fields — construct via NewTaskVerifier
}
```

Not an interface. Chat holds `*verification.TaskVerifier`.

## Constructors and methods

### `NewTaskVerifier`

```go
func NewTaskVerifier(
    client perception.LLMClient,
    localDB *store.LocalStore,
    shardMgr *coreshards.ShardManager,
    autopoiesisOrch *autopoiesis.Orchestrator,
) *TaskVerifier
```

Any argument may be nil; behavior degrades (see IMPLEMENTED_SPEC / failure modes).  
`taskExecutor` starts nil until `SetTaskExecutor`.

### `(*TaskVerifier).SetTaskExecutor`

```go
func (v *TaskVerifier) SetTaskExecutor(te session.TaskExecutor)
```

Mutex-protected. Prefer over ShardManager for spawn.

### `(*TaskVerifier).SetSessionContext`

```go
func (v *TaskVerifier) SetSessionContext(sessionID string, turnCount int)
```

Call before `VerifyWithRetry` if persistence should key correctly. Chat does this every verified turn.

### `(*TaskVerifier).VerifyWithRetry`

```go
func (v *TaskVerifier) VerifyWithRetry(
    ctx context.Context,
    task string,
    shardType string,
    maxRetries int,
) (string, *VerificationResult, error)
```

| Arg | Meaning |
|-----|---------|
| `task` | Task description (mutated across retries with enrichment) |
| `shardType` | Initial intent verb / shard selector string |
| `maxRetries` | If `<= 0`, coerced to **3** |

| Return | Meaning |
|--------|---------|
| `string` | Last shard result text (even on escalation) |
| `*VerificationResult` | Last verification (may be nil only if spawn failed before verify) |
| `error` | Spawn failure, or `ErrMaxRetriesExceeded` |

## Unexported helpers (callers must not depend)

These are package-private; listed for architecture, not public contract:

| Helper | Role |
|--------|------|
| `spawnTask` | Unified execute |
| `normalizeIntentVerb` | Persona → `/verb` |
| `isReviewTask` | Prompt branch |
| `verifyTask` | LLM/heuristic judge |
| `applyCorrectiveAction` | Context gather |
| `findMatchingSpecialist` | Keyword specialist pick |
| `enrichTaskWithContext` | Retry prompt assembly |
| `storeVerification` | Persist |
| `basicQualityCheck` | Regex-ish fallback |
| `parseVerificationResponse` | JSON parse |
| `truncateForVerification` / `truncateContext` | Size limits |
| `selectBestShard` | LLM selection |
| `heuristicShardSelection` | Rule selection |
| `parseShardSelection` | Selection JSON |

## Consumer API surface (outside package)

Chat helpers that *consume* types (not part of this package but stable integration):

| Function | File | Uses |
|----------|------|------|
| `formatVerifiedResponse` | `cmd/nerd/chat/helpers.go` | `*VerificationResult` |
| `formatVerificationEscalation` | `cmd/nerd/chat/helpers.go` | `*VerificationResult` |
| `shouldVerifyDelegation` | `cmd/nerd/chat/delegation_routing.go` | gates call |

## Import path for dependents

```go
import "codenerd/internal/verification"
```

Known importers (production): `cmd/nerd/chat` packages only (helpers, model_types, process, session_boot, session_shared_boot).
