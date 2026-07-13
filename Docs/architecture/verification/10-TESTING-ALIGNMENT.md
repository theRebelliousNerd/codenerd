# verification — Testing Alignment

> Last verified: **2026-07-13**

## Commands

```powershell
go test ./internal/verification/...
go test -count=1 ./internal/verification/...
go test -race ./internal/verification/...
go test -v ./internal/verification/ -run TestBasicQualityCheck
```

Chat-side gate:

```powershell
go test ./cmd/nerd/chat/ -run TestShouldVerify -count=1
# routing_arbitration_roundtrip_test.go includes shouldVerifyDelegation cases
```

Store-side persistence (sibling):

```powershell
go test ./internal/store/ -run Verification -count=1
```

## Test file map

| File | ~LOC | Focus |
|------|-----:|-------|
| `verifier_test.go` | 103 | isReviewTask table; basicQualityCheck; parse fences; truncate |
| `verifier_normalize_test.go` | 42 | normalizeIntentVerb cases; SetTaskExecutor nil |
| `verifier_gaps_test.go` | 801 | Expanded edges across almost all helpers |

## Coverage by behavior

| Behavior | Tested? | Where |
|----------|---------|-------|
| Review keyword classification | Yes | test + gaps |
| basicQualityCheck violations | Yes | multi + individual |
| Parse JSON + fences | Yes | |
| Parse invalid JSON | Yes | |
| Truncation limits | Yes | |
| NewTaskVerifier nil deps | Yes | |
| SetSessionContext | Yes | |
| spawnTask no executor | Yes | |
| enrichTaskWithContext | Yes | |
| heuristicShardSelection mappings | Yes | hallucinated→research, etc. |
| parseShardSelection | Yes | |
| VerifyWithRetry no executor | Yes | |
| VerifyWithRetry maxRetries 0 path | Partial | errors via no executor (defaults to 3 attempts attempted but fails first spawn) |
| applyCorrectiveAction nil/decompose/tool | Yes | no-orch |
| findMatchingSpecialist no mgr | Yes | |
| storeVerification no DB | Yes | no panic |
| Constants values | Yes | |
| verifyTask nil client | Yes | soft success |
| selectBestShard nil mgr | Yes | fallback |
| **Live LLM verifyTask** | **No** | would need mock LLMClient |
| **Full multi-attempt success after fail** | **No** | needs fake executor + LLM |
| **Specialist matching with real ShardManager list** | **No** | |
| **Autopoiesis GenerateTool success path** | **No** | |
| **normalize + TaskExecutor accept** | **No** | integration |
| **Race on concurrent VerifyWithRetry** | **No** | |

## Test quality notes

Strengths:

- Gaps file is thorough for pure functions  
- Table-driven classification tests  
- Explicit constant string locking  

Weaknesses:

- Almost no interface fakes for `LLMClient` / `TaskExecutor`  
- End-to-end `VerifyWithRetry` happy path untested  
- Fail-open path when LLM returns error untested at loop level  
- Chat string-compare for `ErrMaxRetriesExceeded` not asserted against `errors.Is`  

## Recommended test additions (docs only — not implementing)

1. Fake `TaskExecutor` that fails quality once then returns clean code; fake LLM that fails then passes → assert single success.  
2. Fake LLM that always errors → assert fail-open Soft success behavior (documents policy).  
3. Fake LLM returning Success=true with non-empty violations → assert retry continues.  
4. ShardManager with specialist named `rod` + query containing `browser` → findMatchingSpecialist.  
5. `errors.Is(err, verification.ErrMaxRetriesExceeded)` after forced failures.  
6. Race test optional if concurrent use becomes real.

## Alignment with package principles

| Principle | Test support |
|-----------|--------------|
| I1 loop bound | Indirect only |
| I3 spawn hard fail | Yes |
| I5 normalize | Strong |
| I6 review mode | Classification only (not LLM prompt content) |
| Fail-open documented | Partial (nil client) |

## CI expectation

Package tests are pure-Go unit tests; no CGO requirement specific to verification. Safe in default `go test ./internal/verification/...`.
