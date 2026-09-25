# 10 — Recurse: improve everything, slow and steady, bottom to top, forever

Program of record for the owner's operating mode (2026-09-25): codeNERD runs on
the owner's machine in an unending loop that improves, builds and bug-fixes the
workspace it is in. Not only codeNERD — any workspace. One cycle at a time. It
walks the workspace's own dependency graph from the leaves to the entry points,
then starts again at the bottom, forever. Each cycle leaves the gates strictly
no worse, and the only stop is the owner cancelling it.

**Recurse is this loop.** `nerd campaign recurse` and `/recurse` in chat are
its entry points. It is built out of the recurse code that exists today
(`internal/campaign/recurse_*.go`), not beside it, and there is no second
command. It is the R7 instrument of the ladder (`05-elite-harness-ladder.md`)
made general, plus the unattended-hardening bar
(`06-unattended-hardening.md`).

## What recurse is today, and what changes

`nerd campaign recurse` plans its waves in Go from a fixed table of codeNERD's
own packages (`internal/campaign/recurse_dag.go:37`), never measures a gate, and
its stall fuse counts tasks the model *said* it completed. Pointed at any other
repository, it plans work on paths that do not exist. The same codeNERD-shaped
Go constants sit under three other decisions:

| Where | Constant | Becomes |
|---|---|---|
| `campaign/recurse_dag.go:37` | codeNERD's package DAG | derived from the world model's import graph |
| `campaign/risk_scoring.go:20` | protected roots `internal/core`, … | `workspace_critical_path` facts |
| `core/dreamer.go:117` | catastrophic-delete prefixes | `workspace_critical_path` facts (`.git`, `.nerd` always) |
| `northstar/types.go:421` | high-impact paths | `workspace_critical_path` facts |
| `session/build_verify.go` + forcing gates | `go build`, `go test`, `go vet` | the workspace's own gates, per language |

codeNERD's current lists move into codeNERD's own `nerd.md`, so its behaviour is
unchanged when it works on itself.

## The loop

```
  ┌─────────────────────────────────────────────────────────────────────┐
  │ PASS n   (angle pair rotates each pass: stabilize·harden → optimize… )│
  │                                                                     │
  │   for node in bottom_to_top(workspace DAG):        one at a time    │
  │     1 MEASURE   gates scoped to the node      → finding facts       │
  │     2 PICK      kernel: recurse_next(Finding)  (none → 5 DISCOVER)   │
  │     3 FIX       one gated turn / campaign, the finding's check      │
  │                 as its acceptance witness                           │
  │     4 RATCHET   re-measure: keep (commit) only if the target is     │
  │                 resolved AND no gate anywhere got worse; else revert │
  │                 the attempt's writes and record why                 │
  │     5 IMPROVE   ALWAYS, on every visit, red or green: the pass's     │
  │                 angle proposes an improvement to this node, kept    │
  │                 only if a measured metric moves the right way AND   │
  │                 no gate or metric anywhere got worse                 │
  │   after the top node: full-workspace gates, then PASS n+1            │
  └─────────────────────────────────────────────────────────────────────┘
```

* **Sequential.** One cycle, one model turn or campaign at a time. No lane
  parallelism inside the loop.
* **Bottom to top.** `recurse_node_order(Node, Rank)` is derived: a node's rank
  is one more than the highest rank of anything it imports. Leaves go first and
  entry points last. Cycles in the import graph collapse into one node.
* **Nodes.** A node is a directory package (Go), a Python package, a JS/TS
  package directory, or a Rust crate/module. The world model already asserts
  `file_package`, `file_imports`, `dependency_link` (tree-sitter, multi-language)
  and `entry_point`.
* **Forever, and always improving.** There is no max-pass bound, and no node is
  ever "finished". A visit ends when its fix queue is empty or stalled *and*
  its improvement step has run. The loop then moves on, and the node comes up
  again next pass under a different angle. Stopping the whole run takes the
  owner's cancel.

## Improvement: every visit, measured

Fixing what is red is the floor. Every node visit also runs an improvement step,
because the owner's rule is "always improve". "Improve" means a metric moved,
never a model's opinion, so the loop cannot churn: a change that moves no
metric is reverted.

* **Angles rotate by pass.** Each pass leads with one angle. The angle decides
  what the improvement step attempts and which metric must move.

  | Angle | Attempts | Must move |
  |---|---|---|
  | stabilize | pin untested behaviour, fix flaky tests | tests that pin the node's functions; coverage |
  | harden | error paths, bounds, input validation | coverage of error branches; failing fault-injection tests go green |
  | optimize | hot paths the node's benchmarks name | benchmark time or allocations |
  | simplify | dead code, duplication, complexity | dead-code count; complexity; lines, at equal behaviour |
  | wire | capability with no production caller | unreachable-function count; starved predicates |
  | document | docs that cite code that moved | doc citations that resolve |
  | extend | a missing capability the node's own docs or north star ask for | a new behavioural test that passes (and fails without the change) |

* **Metrics are facts.** `node_metric(Node, Metric, Value, Cycle)` is measured
  before and after, workspace-derived like the gates. Coverage comes from the
  language's coverage tool, benchmarks from the language's bench runner, dead
  code from the workspace's audit gate if it declares one. A metric the
  workspace cannot measure is absent, and an angle with no measurable metric on
  a node is skipped for that node, visibly.
* **The ratchet covers metrics too.** Keep only if the angle's metric improved,
  every gate is at least as green, and no other metric regressed past its noise
  band (benchmarks are measured several times; the band is config, not a
  constant).
* **The north star feeds "extend".** When a workspace declares a north star (the
  `northstar:` frontmatter in `nerd.md`), the extend angle draws from the
  distance between it and the evidence. The loop keeps pulling the codebase
  toward what its own north star says it should become.

## Gates: the workspace's own, per language

`workspace_gate(ID, Kind, Command, Scope)`, where Kind ∈ {/build, /test, /lint,
/audit}. Command is an argv, never a shell string. Scope is a node, or /all for
a workspace-wide gate.

Sources, in precedence order:
1. `nerd.md` frontmatter. `commands.build/test/lint` already exist. New keys:
   `gates:`, a list of `{id, kind, run, scope}` for extra audits (codeNERD lists
   its `cmd/tools/audit_*`, dead-code budget, predicate corpus check), and
   `critical:`, a list of path globs.
2. Detection from the project profile:
   * `go.mod` → `go build ./...`, `go vet <pkg>`, `go test <pkg>`
   * `pyproject.toml` / `pytest.ini` / `setup.cfg` → `python -m pytest <pkg>`, `python -m compileall`
   * `package.json` → its `build`, `test` and `lint` scripts through the detected package manager
   * `Cargo.toml` → `cargo build`, `cargo test -p <crate>`, `cargo clippy`

A gate that cannot run (toolchain missing) asserts `gate_unavailable(ID,
Reason)`. The node's verdict is then `/unverified`, never a pass. The session
forcing gates use the same registry: build and test rounds run the workspace's
gates for the written files' language. Coverage and pinning stay Go-only for
now, and are owed only for /go writes, as today.

## Findings and the kernel's choice

* `gate_result(Cycle, Gate, Verdict, FindingCount)`
* `finding(ID, Gate, Kind, Target, Signature)`. ID is stable across cycles: a
  hash of gate, kind, target and normalised message, so the same failure keeps
  its identity.
* `finding_attempt(ID, Cycle, Outcome, Signature)`, where Outcome ∈ {/kept,
  /reverted, /refused, /unverified}.
* Policy `recurse.mg` (the policy recurse already reads, extended):
  * `recurse_candidate(ID, Priority)`, ordered as red build > red test >
    regression introduced this pass > lint/audit. The improvement step runs
    after these on every visit, whatever the queue held.
  * `recurse_angle(Pass, Angle)` and `recurse_metric_for(Angle, Metric)` decide
    the step's target. `recurse_improvement_kept(Cycle)` is derived from the metric
    before and after, plus no regression.
  * `finding_stalled(ID)`, derived when two attempts ended with the same failure
    signature and nothing in the finding's target changed in between. That is a
    repeated failure, not a counter.
  * `recurse_next(ID)`, the best non-stalled candidate on the current node.
  * `recurse_forbidden(Path)`, from `nerd.md` `forbid:` and the constitution's
    own files. The loop never edits its own safety or permission logic.

## Durability and bounds

* `.nerd/recurse/journal.jsonl` is append-only, with one record per state
  change (measure, pick, fix, ratchet). A killed run resumes at the last
  committed record and neither loses nor repeats a kept change.
* Commits go to a dedicated branch, `nerd/recurse` by default. There is no push
  unless configured, and never a force-push. Each commit names its finding and
  cycle.
* Bounded growth: journal compaction per pass; outputs retained per finding
  under a byte budget from config; the working-set databases already pruned
  (wave 4 item C).
* `nerd campaign recurse status` shows the ledger: pass, node, what was measured,
  picked, kept, reverted and stalled, with the gate trend.

## CLI

* `nerd campaign recurse` runs forever by default, and `/recurse` in chat does
  the same. `--waves N` stays as an opt-in bound for a short run. The loop
  needs no `--yolo`: its stops are derived.
* `nerd campaign recurse --plan` is a dry run. It prints the derived DAG order,
  the detected gates and the first measurement. No model is involved, so the
  owner can inspect it before running for real.
* `nerd campaign recurse status` shows the ledger, and
  `nerd campaign recurse stop` stops the loop.
* The existing flags keep working. `--subsystem` narrows the derived DAG
  instead of the Go table. `--angles` picks the pass's angles from the
  improvement table.

## Ready to hand over

The loop runs on the owner's machine only when all of these hold. Each one is a
test or a recorded run, not an opinion.

1. **Generality.** `--plan` on three fixture workspaces (Go, Python, TS) derives
   the right DAG order and the right gates. On codeNERD itself it derives an
   order the owner recognises.
2. **Ratchet.** End to end with a scripted fake model:
   * a fixing turn is kept and committed;
   * a turn that fixes its target but breaks another gate is reverted;
   * a turn that does nothing is reverted and its finding eventually stalls;
   * an improvement that moves its metric is kept;
   * one that moves nothing, or regresses another metric, is reverted;
   * a green node still gets its improvement step every visit.
3. **Resume.** A run killed mid-fix resumes without losing or repeating a kept
   change (the journal test kills the process at every state boundary).
4. **Safety.** A fake model that tries to edit a forbidden path or the
   constitution is refused, and the finding is marked /refused, not stalled
   silently.
5. **Bounds.** A 1,000-cycle fake run leaves journal, logs and databases under
   their budgets.
6. **Gates honest.** A workspace whose test gate cannot run reports
   `/unverified`, never a pass.
7. **CI green,** including the Windows path-alias step.

Then the owner starts `nerd campaign recurse` locally, and the first hours are monitored
live.

## Status

| Phase | What | State |
|---|---|---|
| 1 | Derived DAG (`campaign.DeriveWorkspaceDAG`: Go via `go list`, Python and JS/TS imports, Rust crates; cycles collapse); per-language gates (`internal/gates`); `nerd.md` `gates:` and `critical:`; `nerd campaign recurse --plan` | done |
| 2 | Measure → findings as facts (`gates.Run`, `gates.Findings`), `policy/recurse.mg` pick/stall/ratchet, git ratchet keep/revert on `nerd/recurse`, journal + in-flight settle on restart, CLI and chat both on `RunRecurseCycles`; wave planner and wave runner deleted; end-to-end tests with a scripted fake model on the single-store and the production kernel | done |
| 3 | Forever by default, `status`/`stop`, measured improvement angles (every visit), `workspace_critical_path` replacing the three Go constants | next |
| 4 | Session forcing gates run the workspace's gates for non-Go writes; `/unverified` when a gate cannot run | |
