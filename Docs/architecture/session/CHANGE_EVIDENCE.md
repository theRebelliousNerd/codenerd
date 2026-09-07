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

`go run ./cmd/tools/change_benchmark -mode minimal|current|evidence -workspace
<fresh-fixture> -config <provider-config> -contract <contract> -output <outside.json>`
runs a controlled local comparison. Each mode uses the same model, tool catalog,
call/output budgets and independent evaluator. Use a fresh fixture per run and
retain failed attempts when reporting cost. One fixture is a smoke experiment,
not a benchmark score or an estimate of general completion rates.
