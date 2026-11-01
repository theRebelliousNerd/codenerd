# Revision-bound Go bug-fix acceptance

`nerd -w <isolated-workspace> fix --acceptance <contract.json>` runs a caller's
task with explicit acceptance. Use a disposable checkout or worktree: this
command edits the selected workspace and does not create isolation for you.
The contract is copied before model execution; it is not inferred from a final
answer. For example:

```json
{
  "task": "Fix Add while preserving zero behavior",
  "authority": "caller-reviewed regression contract",
  "obligations": [
    {"id":"sum","description":"positive inputs add","package":".","test":"TestAdd","test_file":"add_test.go","reproducer":true},
    {"id":"zero","description":"zero remains zero","package":".","test":"TestZero","test_file":"add_test.go"}
  ]
}
```

Before editing, the reproducer must execute and fail; regression obligations
must execute and pass. After editing, every selected test must execute and pass
against a stable workspace snapshot. Existing tests, fixtures and module files
are protected. Changing them requires a newly reviewed contract. This first
path intentionally supports source bug fixes with tests already supplied.

Reports in `.nerd/evidence/` record the contract hash, before/after content
snapshots, exact commands, toolchain, environment hash, output hash and outcomes.
Snapshots include dirty/untracked files and configuration, with Git and runtime
output excluded. Environment and executable selection are pinned for the
transaction; changes to effective Go configuration invalidate witnesses.

Execution, changed artifacts, passing checks and demonstrated acceptance are
different states. `turn_executed` retains the mechanical hollow-success guards;
`turn_done` additionally requires host-issued acceptance for the current
snapshot. Ordinary turns without a contract can still edit and run checks, but
their requested behavior remains `unverified`. Later edits invalidate earlier
green results. Reports are deterministic and appended to the model response.

The evidence is bounded behavioral testing, not proof of arbitrary software.
Caller authority is an explicit input, not an authenticated signature. Tests
still need review for relevance and detection power. The initial invalidation
strategy is conservative whole-workspace invalidation.

## GAP-SESSION-GENERATED-RECEIPT-HANDOFF

A generated process receipt records process start, decoded output, validation
and durable-feedback outcomes. A successful receipt does not classify arbitrary
code as a recognized write tool or prove a task accepted. A whole-workspace
snapshot difference also does not establish the requested effect: incidental
files or another writer can change that snapshot while the target is absent.
Keep the recognized-write completion gate and exact permission checks intact.

The session result must retain each actual returned generated-bridge receipt,
including partial execution and rejected/incomplete receipts. Callers need those
separate outcomes alongside the returned error and turn verdict to determine
what happened after an unsuccessful task. A refusal before the bridge produces
no invented receipt. Retention must not replay an operation, manufacture missing
fields, turn receipt flags into authority or change model observation redaction.

Expose copies through an explicit result accessor; keep raw receipt payloads
out of automatic result serialization and provider context. Preserve per-call
and per-turn ownership, hard-error/cancellation settlement and ordinary modular
write accounting. Exit: independently reproduce the lost result on unchanged
main, check actual process/validation/reopened-learning and workspace snapshots,
retain target-absent and pre-dispatch refusal controls, and run offline race and
full integration checks. Correct write refusals remain red.

Handoff implemented: `GeneratedToolReceipts` and the call-owned log in
`internal/session/generated_receipts.go:16` retain copied bridge-return values.
`executeAndRecordToolCall` installs the log and transfers its values before
processing error exits; `executeToolCall` records immediately after the bridge
returns (`internal/session/executor_tools.go:1060`, `:1070`, `:2827`). This does not change the
primitive validation result, recognized-write accounting or turn verdict.

The identical controls on unchanged production code report 0 pass / 8 fail
unit events and 6 pass / 7 fail integration events; repaired controls pass 8
and 13 respectively. Repeated unit/generated-session/write-accounting race
checks pass 57 events and real production-path race controls pass 32. Controls
include native process cancellation/drain, concurrent callers, partial and
incomplete receipts, missing target despite a changed snapshot, exact-approval
refusal, copied access and exclusion from automatic JSON. All 2,761 existing
test files and 212 existing logic files remain byte-identical to the baseline;
generated write acceptance remains a separate, unfulfilled obligation.

The default sqlite_vec suite passes 22,465 events with 80 skips; build, tagged
compilation and both vet checks pass. A full tagged run without concurrent
heavy checks completes all 890 top-level tests: 832 pass / 58 fail, exactly the
baseline failure set, including all seven original generated-write refusals.
An earlier parallel verification run retains a learning-store performance-cutoff
failure while the spawner cutoff passes; three isolated repetitions of that
learning-store cohort pass on both baseline and candidate. Both full censuses
remain in external evidence. No timing fix, accepted pipeline or recurse
qualification follows from this handoff repair.

`go run ./cmd/tools/change_benchmark -mode minimal|current|evidence -workspace
<fresh-fixture> -config <provider-config> -contract <contract> -output <outside.json>`
runs a controlled local comparison. Each mode uses the same model, tool catalog,
call/output budgets and independent evaluator. Use a fresh fixture per run and
retain failed attempts when reporting cost. One fixture is a smoke experiment,
not a benchmark score or an estimate of general completion rates.


## Shared input measurement for mission admission

`internal/evidence/change.go:VerificationInputs` is the common measurement for
existing Go tests, module files, testdata and `nerd.md` gate definitions.
Source-acceptance transactions compare the digests; mission admission also uses
them to protect the oracle before writes. Mission receipts reference a complete
private sidecar instead of embedding an unbounded input map in a model request.
This does not widen the supported behavioral contract to other languages or
prove arbitrary mission requirements. Execution evidence for the new bridge is
pending the production fixture in `cmd/nerd/cmd_campaign_mission_test.go`.
