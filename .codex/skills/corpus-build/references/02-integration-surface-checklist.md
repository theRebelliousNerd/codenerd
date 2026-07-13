# codeNERD integration-surface checklist

`surfaces.yaml` is the machine-readable candidate registry. `scripts/verify_surfaces.py` classifies it against a run manifest. A path existing does not prove the feature is wired; the auditor must attach file:line plus an executable reachability oracle.

## Executive path

Check only the surfaces applicable to the requirement:

| Surface | Live location | Question |
|---|---|---|
| Perception | `internal/perception/` | Does user input become the intended structured atoms? |
| Schemas | `internal/core/defaults/schemas.mg` | Is every predicate declared with the correct types/bounds? |
| Policy | `internal/core/defaults/policy/` | Does the kernel derive the action, and does dangerous behavior require `permitted(...)`? |
| Kernel | `internal/core/` | Are facts loaded, derived, retracted, and audited at the correct lifecycle point? |
| VirtualStore | `internal/core/virtual_store.go` and split files | Is the derived action routed to a real effect with validation? |
| Articulation | `internal/articulation/` | Does the result return without bypassing the executive or Piggyback contract? |

## LLM and specialist path

| Surface | Live location | Question |
|---|---|---|
| Prompt compiler | `internal/prompt/` | Is behavior selected JIT from atoms with budget/dependency coverage? |
| Prompt atoms | `internal/prompt/atoms/` | Does each new behavior have canonical atom content and metadata? |
| Shard manager | `internal/core/shards/` | Are lifecycle, model/client selection, context, cancellation, and result ownership correct? |
| Shard registry | `internal/shards/registration.go` | Can the runtime construct the specialist? |
| Agent data | `.nerd/agents/` | Are project/user specialist prompts and durable data in the intended runtime location? |

## External and operator path

| Surface | Live location | Question |
|---|---|---|
| CLI/chat | `cmd/nerd/` | Is the operation registered, documented, bounded, and reachable from help/chat routing? |
| Session | `internal/session/` | Does the clean execution loop carry state/cancellation correctly? |
| Tools | `internal/tools/` | Is the capability registered and schema-compatible? |
| MCP | `internal/mcp/` | Is the MCP operation advertised and dispatched to the same core behavior? |
| Campaign | `internal/campaign/` | If long-running, are tasks, checkpoints, recovery, and artifacts durable? |
| Config | `internal/config/` and `.nerd/config.json` | Is configuration parsed once, defaulted, and observable? |
| Store/memory | `internal/store/`, `internal/memory/` when applicable | Is state scoped, migrated, closed, and recoverable? |
| Logging/observability | `internal/logging/`, `internal/observability/` | Can a run be proven without leaking payloads or credentials? |

## Quality and corpus path

- Package-local unit tests cover invariants and failure cases.
- Cross-system tests prove the complete fact/action route where applicable.
- Race/fuzz/benchmark profiles are selected by risk, not ritual.
- `go test ./...` is the repo-level gate; record existing unrelated failures honestly.
- Architecture status in `Docs/architecture/<feature>/` is changed only by the doc-auditor from live evidence.
- `.quality_assurance/remediation/` receives a bounded failure packet only after the local fix budget is exhausted; codeNERD has no `internal/testing/remediation` runtime.

## Verdicts

Every candidate is `REQUIRED`, `OPTIONAL`, `N-A`, or `BLOCKED`. A required surface passes only with:

1. a concrete registration/routing citation;
2. an executable oracle or deterministic structural validator;
3. no contradictory stale implementation path;
4. a named owner for any residual.
