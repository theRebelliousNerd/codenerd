---
doc-class: deep-dive
subsystem: campaign-recurse-python-verification
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-04
verified-against: cfe155e28bf5865920da16e716fb4bb9586e3f67
verification-method: static-source-inspection
main-rechecked: 2026-11-04
supersedes: []
---

# Bind recurse Python checks to the current source bytes

## Finding and decision

**A newly launched Python build and test can pass against old bytecode while recurse measures and commits different source.** The detected build is `python -m compileall -q {node}`, without forced recompilation. The detected test is a fresh `python -m pytest -q {node}` process in the same checkout. For an ordinary timestamp-based CPython cache, a changed module with the same byte length and integer-second modification time can retain a valid-looking cache of its previous implementation. `compileall` can skip it, and the test process can import that older implementation. [Detected gates][detect] [Command construction][command] [Compiler skip][py-skip] [Import validation][py-validate] [Cached-code return][py-load]

The conditional consequence reaches the ordinary simplify ratchet. Static line counting reads the new source, while unchanged tests can exercise old bytecode. A source change that reduces lines but breaks behavior can therefore supply the existing inputs for `/keep`: a changed file, a favorable metric, a completed nonempty test and no observed gate or guarded-metric regression. The driver then commits the changed source. This report does not claim that a model proposes the bad edit, that independent review necessarily misses it, or that this happened in a deployed run. It identifies a verification boundary that cannot independently exclude the constructed candidate. [Remeasurement][after] [Tested projection][tested] [Metric acquisition][metrics] [Keep rule][keep-rule] [Commit branch][adopt]

**Bounded correction:** make the built-in Python verification contract establish that relevant project modules are compiled or loaded from the admitted current bytes. Preserve real host execution, applicable tests, owner commands, source stability, cancellation, permissions, and Mangle's keep/revert authority. Choose and qualify a cache strategy explicitly. A fresh process and a zero exit are insufficient substitutes for this source-to-executable association.

**Evidence boundary:** public main was pinned to [cfe155e28bf5865920da16e716fb4bb9586e3f67][pin]. Root/scoped guidance, campaign architecture, the recurse journey, the current 384-file audit inventory, related reports, implementation and authored test bodies were inspected. Retained CodeNERD source bytes were checked against the pin's Git blob identities. External implementation analysis uses CPython [60403a5409ff2c3f3b07dd2ca91a7a3e096839c7][py-pin], resolved from `v3.13.0`, plus official Python and pytest documentation. That is an explicitly bounded interpreter example, not a claim about the user's installed version or every Python implementation. No repository program, interpreter fixture, test, build, gate, model call, inference, training or benchmark was run. No implementation, credential, setting or permission was changed. All fixture outcomes below are source deductions and proposed evaluations, not captured results.

## Architectural role and implementation status

Root guidance assigns creative work to models and executive control to the deterministic host/kernel, including revision-aware evidence. Campaign guidance requires actual host testing rather than model prose. The recurse journey intends each improvement to move a measured property without making the workspace worse. Those obligations require the executed artifact to correspond to the candidate being judged, in addition to executing the correct command over the correct package. [Root][root] [Campaign guidance][campaign-guidance] [Journey][journey]

This issue needs neither mission mode nor the optional error-free gate ladder. Both CLI and chat use the same cycle driver and return the attempt campaign's retained writes. Improvement campaigns have no single declared acceptance command; the outer driver supplies the measured gates and metric comparison after the executor returns. Other task/session checks can catch the error, but their possible success does not establish the outer Python cache contract. [CLI][cli] [Chat][chat] [Attempt contract][attempt-contract]

| Boundary | Status at the pin | Relevant limit |
|---|---|---|
| Python project and node discovery | Implemented | A Python marker supplies gates; admitted `.py` files supply nodes. The witness uses one root node and no missing dependency edge. [Detection][detect] [Graph][graph] |
| Fresh host subprocesses after a changed attempt | Implemented | Each own-node gate is run again. This is not reuse of a CodeNERD verdict cache. [Remeasurement][after] [Runner][runner] |
| Current source line and test-function metrics | Implemented | Metrics read source files; the witness preserves the test and lowers production lines. [Metric reader][metric-reader] |
| Forced source compilation or isolated Python cache for these gates | Not established | The built-in argv and command builder add neither force nor a fresh bytecode namespace. [Detection][detect] [Command construction][command] |
| Successful Python test projection | Implemented | A completed test that is not classified as no-tests sets `Tested`. No executable-source digest accompanies that fact. [Tested projection][tested] [Fact bridge][fact-bridge] |
| Source-associated cache preparation and its failure disposition | Proposed | No implementation or passing evaluation is claimed. |

The intended correction belongs first in the host's language-specific execution contract. Mangle cannot infer that CPython loaded older code from a Boolean process verdict that does not distinguish that state. More deliberation or a stricter prompt would leave the missing observation unresolved.

## The producer contract that makes the witness possible

### Timestamp caches identify metadata rather than complete source content

Python documents two bytecode invalidation families: timestamp/size and hash-based. PEP 552 retains timestamp invalidation and describes the extra source-reading/hash work of checked hashes. Timestamp metadata can agree for different source bytes; content identity does not follow from that agreement. [Python import contract][import-doc] [PEP 552][pep552]

The pinned CPython loader makes this precise. It obtains the source modification time, converts it to an integer, reads a cache for the current interpreter/cache tag and classifies its header. For a timestamp cache it compares the stored modification-time word and source-size word. If those checks succeed, it returns the code object from the cache before the source-reading/compilation fallback. The witness assumes the same interpreter magic, optimization/cache location and ordinary source loader throughout. [Loader sequence][py-load] [Header validation][py-validate]

The relevant time is **the source file's recorded modification time**, not when the cache was created, when a gate started, or the current wall clock. Running an edit soon after a baseline test is not enough if the original source has an older timestamp. The witness expressly requires an actual collision or preservation of the recorded integer-second timestamp. Once such a collision exists, simply waiting longer does not invalidate the cache: the source metadata has not advanced.

### The detected compile step can preserve the old cache

The pinned `compileall` fast path is even narrower than the runtime loader. Unless `force` is enabled, it compares the first twelve cache bytes against the interpreter magic, timestamp-cache flags and integer source mtime. It returns success when they agree, before calling `py_compile.compile`. That fast path does **not** compare the source-size field. Equal size is needed for the subsequent ordinary import in this witness, not for `compileall`'s skip. [Compiler skip][py-skip]

When compilation does happen with no configured `SOURCE_DATE_EPOCH`, the pinned `py_compile` default is timestamp mode. Its timestamp output stores the source metadata. `compileall` documents `-f` as the way to force rebuilding despite matching timestamps and exposes separate invalidation-mode selection. These facts support a bounded implementation choice; they do not imply that CodeNERD already selects it. [Default mode][py-default] [Cache writing][py-write] [Compileall options][compileall-doc]

### A fresh pytest process can still use the old project implementation

The example's production module is neither a collected test module nor a plugin or a module explicitly registered for assertion rewriting. Pytest's documented rewriting scope therefore does not replace that module's normal import with a fresh parse of its source. The unchanged test itself executes, but its call resolves to the code object loaded from the ordinary production-module cache. This is a filesystem bytecode issue, not reuse of `sys.modules` across CodeNERD attempts. [Pytest rewriting scope][pytest-rewrite] [CPython import path][py-load]

The report does not assume every plugin, loader, interpreter or installation follows this route. A custom loader, a preexisting checked-hash cache that is actually checked, a different source timestamp or size, or a cache miss can break the witness. These are required controls, not reasons to represent the unqualified built-in command as source-certified.

## Source trace through the ordinary recurse path

### Detection and execution preserve the cache namespace

`gates.Detect` honors canonical owner commands over detected gates. With no override and a Python marker, detection creates node-scoped `python:compile` and `python:pytest`. The interpreter selector prefers `python3` when found. For the root node when `python3` is selected, their argv are `python3 -m compileall -q .` and `python3 -m pytest -q .`. [Owner override][override] [Detected commands][detect] [Interpreter selection][interpreter]

`Command` expands the node placeholder and uses `WorkspaceCommand`. The latter constructs the fresh process with its working directory set to the workspace, supplies the allowlisted environment and overlays declared variables. It does not copy the tree, remove bytecode, introduce a new Python cache prefix or add a source-content invalidation policy. `SOURCE_DATE_EPOCH` and `PYTHONPYCACHEPREFIX` are not in the default pass-through list; an explicitly allowed or declared value is a separate configuration and must be assessed as such. The primary witness has neither. [Gate expansion][gate-command] [Command construction][command] [Environment list][environment]

Recurse works in the owner's checkout and deliberately avoids reset-hard/stash/clean behavior. Its untracked census uses Git's `--exclude-standard`. An ordinary `.gitignore` entry for `__pycache__/` lets the cache survive without becoming an adopted source file. This is appropriate artifact hygiene; the missing requirement is truthful verification, not blanket deletion of ignored owner files. [Git ownership][git] [Untracked census][git-census]

### Own-node checks really run again after the edit

On a visit, the driver measures the node's applicable gates before selecting work. Python compile and pytest are both own-node gates. The improvement admission accepts the available Test gate. After `Execute` returns, the driver detects the actual changed paths and reruns every own-node gate when there is a change. No reused CodeNERD receipt is needed. [Visit][visit] [Improvement admission][improve] [After checks][after]

The runner records actual process exit, error and output; completed exit zero passes both native gates. It does not inspect the code objects imported by Python. A wholly passing result produces no finding. Thus the host can correctly report what the subprocess said while lacking the stronger relationship that the ratchet needs. [Runner][runner] [Passing finding handling][findings]

### Metrics and policy join observations of different implementations

`SourceLines` reads non-test source and counts nonblank lines. `CountTests` reads the unchanged test file. After checks, `metrics` reads those current files again. Meanwhile the test predicate is set for a completed pytest result that did not use the detector's no-tests exit 5. The primary witness collects and runs one test with exit zero, so no-tests handling is intact. [Metric reader][metric-reader] [Metric acquisition][metrics] [No-tests disposition][no-tests] [Tested projection][tested]

The policy's simplify angle rewards fewer lines. Its keep rule requires a real change, success of the angle plus a test observation, no worsening measured gate, no forbidden path and no guarded metric regression. Under the witness, build and test remain pass, the test count remains one, no coverage metric is claimed, and the sole changed production file is allowed. The fact bridge asserts those values; the keep branch commits the source paths. [Metric rules][metric-rules] [Keep rule][keep-rule] [Fact bridge][fact-bridge] [Adoption][adopt]

This is an incorrect association of verification with the retained candidate, not a claim that Python executed an assertion incorrectly or that Mangle evaluated the supplied facts incorrectly.

## Concrete conditional witness

### Workspace and environmental assumptions

Use a clean, admitted Git workspace with these tracked files:

```text
.gitignore       ignores .nerd/, __pycache__/ and .pytest_cache/
pytest.ini       contains an ordinary [pytest] section
calc.py          production module below
test_calc.py     unchanged test below
```

Both Python files are directly in the root, giving one ordinary Python node `.`. Use a focused ordinary sweep of that node that reaches the simplify pass, without mission mode, optional error-free gates or an owner replacement build/test command. Interpreter and pytest are available and compatible. There is no special import loader, production-module rewrite registration, coverage plugin, source timestamp mutation during checks, external writer or additional check that independently catches the bad edit. Other admission, permission, executor, budget, journal and Git conditions needed by the keep branch succeed.

The before bytes of `calc.py`, using ASCII, LF and one final newline, are:

```python
def answer():
    x = 42
    return x
```

The proposed erroneous simplification is:

```python
def answer():
    return 0 # shorter!
```

Both byte strings are **38 bytes**. The current source's nonblank line count falls from three to two. This deliberate negative candidate probes the verifier; it is not a suggestion to make the edit or evidence that a model would choose it.

The unchanged test is:

```python
from calc import answer

def test_answer():
    assert answer() == 42
```

Before the candidate, a valid timestamp cache contains the first implementation. Let its stored source-time word be `t` and its stored size be 38. The candidate's recorded mtime has the same integer-second value `t`, with no relevant wraparound or interpreter change. A same-second pair of source writes can create this condition; a file-restoration workflow preserving mtime can also create it. Neither is inferred merely from a fast test run. A deterministic future fixture should set and inspect the source timestamps explicitly instead of depending on a lucky sleep schedule.

The old `calc.py` cache payload must survive the entire executor/candidate phase and both outer checks unchanged. In particular, no inner tool, reviewer, import hook or cleanup step removes it or force-compiles the replacement before the outer pytest import. This is an explicit premise to inspect in a reproduction. Git's changed-path list alone cannot establish what happened to an ignored cache.

For the primary fixture, use distinct subsecond modification times within that same integer second and verify that Git actually reports the content delta. Do not accidentally create a separate Git stat-cache witness by preserving every file-stat field. The source digest and committed content change; the external Python cache key still collides. The verifier is precisely the boundary being asked to distinguish those generations.

### Expected trace from source inspection

| Step | Source-derived observation under these assumptions |
|---|---|
| Before visit and candidate | Build and the unchanged test pass on the correct implementation; the timestamp cache is available. |
| Candidate return | Only `calc.py` has changed; the new bytes are 38 bytes with mtime word `t`. |
| Fresh compile gate | `compileall` sees its matching twelve-byte header and returns success without compiling the replacement. |
| Fresh pytest gate | The ordinary loader accepts the cache's timestamp and size, then loads the old `answer` code object. |
| Test behavior | The existing test executes and sees 42; pytest exits zero. There is no empty test collection. |
| Source metrics | Current production lines are two rather than three; the same one test remains. |
| Ratchet | The stated facts satisfy simplify success and the remaining keep guards. |
| Adoption | The commit contains the replacement returning 0, not the old cached implementation exercised by pytest. |

A clean-cache control over the identical replacement should execute the current `answer`, return 0 and fail the unchanged assertion. That differential result is the crucial proposed oracle. It must be established by future execution, not reported here as already observed.

### What this witness does and does not establish

The strongest claim is conditional false adoption by the **outer ordinary ratchet** once the specified candidate and cache state reach it. The causal premise includes an executable artifact from a different source generation, rather than a wrong test selector, missing test, forged stdout, model success statement or changed acceptance rule.

The same cache mismatch can make the compile command skip a new syntax error, because its fast path does not read the current source. That is a supporting boundary case, not a second end-to-end claim: a later import with a size mismatch could invalidate the cache and expose the error. The complete keep witness deliberately uses equal size and a valid but behaviorally wrong replacement.

A more capable worker, reviewer or an independently source-bound check can reject the candidate. A different mtime or size, absent cache, forced rebuild over the relevant module, or valid checked-hash loading also prevents this exact failure. The report does not broaden the result to every Python project or all mission acceptance modes.

## Required verification contract

Specify the following before selecting an implementation:

1. **Candidate identity.** The test obligation names the admitted candidate source generation and project modules it may execute. Stable filenames, a fresh PID, command identity and before/after source stability are useful but do not individually identify the executable code loaded from disk caches.
2. **Cache admission.** For built-in Python gates, either the relevant bytecode is freshly derived from that source, its content association is validated under a supported interpreter profile, or it cannot be reused. Inability to establish preparation is unverified; a determinate source compilation failure remains a failed build. Neither is evidence of a clean candidate.
3. **Dependency coverage.** A node-local compile step must not silently stand in for source validation of imported project modules outside that node. Establish the relevant scope or use an explicitly conservative workspace strategy. This report's single-root witness avoids that separate graph question.
4. **Preparation and test ordering.** Preparation completes before the dependent test starts, uses compatible interpreter/optimization/cache settings, and is invalidated by a later source change. Separate commands must share the intended prepared cache or each independently establish source fidelity.
5. **Authority and ownership.** Do not delete arbitrary ignored owner files, change global interpreter settings, weaken a project's commands or widen a model's tool permissions. Cache directories created for verification need scoped ownership, cleanup and a defined recovery disposition.
6. **Policy and receipts.** Preserve the exact command/environment receipt and existing no-tests, unavailable, canceled and failed outcomes. Host adapters establish cache/source observations; Mangle decides whether they satisfy the required test/keep obligation. Do not let the model assert a cache-validity fact.

The source snapshot around a command does not, by itself, solve this case. The candidate can be completely stable throughout both processes while those processes reuse a preexisting mismatched cache. Even hashing cache bytes into the same snapshot would show stability, not prove that their code came from the candidate. Any stronger statement requires an association rule, not only an unchanged filesystem check.

## Implementation choices and limits

| Option | What it can establish | Costs and limits to qualify |
|---|---|---|
| Force compile the admitted verification source scope before testing | Breaks the primary cache fast path and produces code from the current bytes. | Recompiles source, writes cache artifacts, can fail on permissions/disk space, and needs complete relevant scope plus sequencing. `-f` on only one directory is not proof about every imported sibling. |
| Rebuild into checked-hash caches and retain checked loading | Makes later source-content changes detectable rather than relying on timestamp/size. | Requires migration of existing timestamp caches, source hashing, supported interpreter/runtime settings and ownership of generated files. An unchecked-hash cache is a different contract. |
| Use an owned fresh bytecode-cache namespace for verification | Can prevent ordinary project imports from reusing the checkout's old cache. | Must cover all relevant processes and supported loader/cache forms, including how a failed prep or later source write is handled. Adds cold compilation, storage, configuration and cleanup work, potentially for standard-library and pytest/plugin imports as well as project code; reusing the namespace across candidates needs its own identity. |
| Copy into an admitted source-only verification workspace | Can remove old checkout artifacts while isolating verification effects. | Larger filesystem/setup cost; exact configuration, data, dependencies, path-sensitive behavior and owner authority must survive. This is not a drop-in promise of hermeticity. |

These are alternatives to assess, not implemented changes. The smallest repair may force recompilation for the built-in gate, with a stated supported domain, while a fuller verification profile handles import closure and isolated artifacts.

Several tempting shortcuts are insufficient at the inspected CPython boundary:

- **Only add `-B` or `PYTHONDONTWRITEBYTECODE`.** The pinned loader's cache-read/return path precedes its `dont_write_bytecode` check, which controls writing the new cache after source compilation. Existing cache reads remain possible. [Loader order][py-load] [Python command options][python-cli]
- **Only ask for checked-hash output.** `compileall`'s existing timestamp fast path can return before invoking `py_compile`; migration must actually occur. A mode label is not evidence that old artifacts were rebuilt. [Compiler skip][py-skip]
- **Only use `--check-hash-based-pycs always`.** The loader applies that setting in its hash-based branch. It does not convert an existing timestamp cache into content-validated bytecode. [Loader branches][py-load]
- **Sleep or rerun the same commands.** The recorded source timestamp stays unchanged. Another process can reproduce the same stale import indefinitely until a relevant input or cache policy changes.
- **Delete all ignored files.** This exceeds the needed cache contract and violates recurse's ownership discipline. Isolation or bounded owned preparation is preferable to indiscriminate cleanup. [Git ownership][git]

## Full cost and failure accounting

Let `B` be the bytes of relevant project source, `N` the number of source modules, `K` the bytecode bytes produced, and `V` the number of verification rounds. A warm timestamp path uses directory/stat/header work and cached-code loading; forcing compilation adds source reads and parsing/compilation. Checked hashing adds source reads and hashing, and cold isolation can compile imports that a warm run reused. The exact cost depends on interpreter, filesystem, module shape and runner/plugin behavior. PEP 552 explicitly recognizes the extra read/hash cost. No speedup or fixed overhead is claimed. [PEP 552][pep552]

Account for the whole pipeline rather than only pytest wall time:

- Discovery/stat/header operations, bytes read and hashed, parse/compile CPU, cache writes and peak disk/memory.
- Preparation launches, test launches, process startup, total elapsed time and repeated rounds. A simple per-round force strategy may pay source preparation on each of `V` rounds; safe reuse requires a validated identity rather than silently dropping that cost.
- Cleanup and recovery, including disk-full, read-only cache destinations, cancellation between prep and test, partial artifacts and concurrent candidates. Preserve bounded process ownership and do not turn cache failures into test passes.
- Source/configuration/import-domain drift between preparation and execution. Reverification costs should be counted when a candidate changes, not hidden as warm-cache savings.
- Downstream repair/review calls, tokens and mistaken-kept changes. Stronger checks may reveal real failures and therefore increase immediate repair work while avoiding unsupported acceptance. That is an accuracy tradeoff to measure, not a guaranteed lower bill.

The correction needs no inference call merely to establish cache mode, interpreter identity or source generation. End-to-end task quality and model cost remain empirical questions.

## Proposed evaluation and acceptance

All evaluations here are **unrun**. Keep the correctness obligation, candidate/test corpus, interpreter and starting cache states controlled when comparing strategies; the cache strategy is the experimental variable. Do not make a candidate implementation look faster by selecting fewer tests or omitting imported source.

First use a small host-execution fixture with a pinned CPython and pytest version, no third-party plugins, temporary source/cache directories and explicit timestamps. Create a valid timestamp cache from the before bytes, replace the source with the specified after bytes, and independently inspect the source timestamp, size, cache header and digest. Do not rely on whole-second sleeps. Record the executable path/version, argv, relevant environment, import location, test count, output, raw exits and final source bytes. A content digest of the candidate is useful evidence even though it is not itself a bytecode-validity fix.

Then exercise actual `gates.Detect` → `gates.Run` → recurse measurement and the real policy adapter, with an owned scripted candidate instead of a live model. Reach the ordinary simplify angle with earlier attempts leaving source unchanged. Assert the retained/reverted source, Git commit contents, measured metrics, test observation and journal outcome. Finally qualify the ordinary CLI/chat callback path with its normal world/CodeDOM and permission setup; a helper-only fixture does not prove that production carries the cache policy and receipt correctly.

| Case | Required observation after a correction |
|---|---|
| Same timestamp word, same size, old timestamp cache, wrong candidate | Current implementation is exercised; unchanged assertion fails; candidate is not kept. |
| Identical wrong candidate with no cache | Same failing behavior; cache presence must not change the verified source generation. |
| Correct simplification with a matching old cache | Independently observe loaded-code provenance or compare with a fresh-compiled control; successful behavior alone cannot prove the new code ran. Valid improvement can still be kept. |
| Different timestamp, then different size | Ordinary invalidation still works; no unnecessary rejection of valid runs. |
| Matching timestamp but changed-size syntax error | Build must not claim current-source compilation merely from the old header. |
| Existing timestamp cache plus requested checked-hash preparation | Verify actual migration, not only argv or configured mode. |
| Checked-hash, unchecked-hash and runtime hash-policy variants | Supported modes have explicit results; unsupported or unverified associations are not represented as current coverage. |
| `-B` or no-write environment with an existing old cache | Demonstrate why no-write alone is not the fix and ensure the chosen contract still holds. |
| Imported project sibling outside the selected node | Preparation/cache isolation covers that relevant module or explicitly declines the stronger claim. |
| Source changes after preparation or while testing | Reject stale association and preserve the normal source-stability requirement. |
| Unavailable interpreter/pytest, no collected tests, timeout or cancellation | Existing unverified/no-tests/cancellation semantics remain intact. |
| Permission error, full cache storage or interrupted preparation | No fabricated pass; owned artifacts are retained or cleaned under a documented recovery rule. |
| Repeated candidates and concurrent sessions | No cross-candidate cache association; no deletion of another owner's artifacts. |
| Owner-declared commands and compatible alternate interpreter | Preserve the owner's contract; qualify support without silently replacing its environment or command. |

Existing source tests verify Python detector selection, passing/no-tests exit codes and acceptance-code handoff. The simplify test verifies that lower source lines can be kept without reducing tests, using Go fixtures. Those assertions are valuable controls but do not establish the old-Python-bytecode case. No test body located in this review exercised the specific source-time/size collision through the Python keep path. This is a bounded coverage assessment, not a claim that every historical test was examined. [Detector tests][detect-tests] [Python handoff test][python-test] [Simplify test][simplify-test]

**Checkable exit:** within the declared supported Python verification profile, the same candidate and unchanged test have the same semantic outcome whether an older timestamp cache is present or absent; a bytecode artifact from a different source generation cannot satisfy the candidate's mandatory test obligation; preparation failure cannot produce a keep; ordinary valid cache use, owner commands, source stability, cancellation and ownership remain intact. Publish measured evidence and implementation-status updates only after those checks actually run.

## Overlap and priority

The recursive main-tree inventory and nearby reports were screened. Targeted GitHub code searches under `Docs/audits` found no `bytecode`, `PYTHONDONTWRITEBYTECODE` or `SOURCE_DATE_EPOCH` report; `compileall` matched three neighboring reports whose relevant passages were inspected. Search indexing and historical branches are not an exhaustive novelty proof.

| Earlier report | Boundary already covered | Distinction here |
|---|---|---|
| [Test gate memoization identity][prior-memo] | CodeNERD can reuse a verdict with an incomplete input key. | Both Python processes actually rerun; reuse occurs inside the interpreter's executable artifact cache. |
| [No-tests evidence continuity][prior-no-tests] | No collected tests must not satisfy an actual test obligation. | The unchanged test executes and passes, but calls an older implementation. |
| [Workspace test witness][prior-witness] | The outer improvement needs a fresh applicable test producer. | That producer runs; the missing association is source bytes to imported code. |
| [Cargo default-member evidence][prior-cargo] | A command may test the wrong selected package set. | The intended Python module and test are selected, but the module's code generation is wrong. |
| [Source metric acquisition][prior-metrics] | Partial file reads can masquerade as an improvement. | Source reads are complete and the line decrease is real; the conflicting observation is execution of old bytecode. |
| [Post-verification effects][prior-effects] | Check-created effects can fall outside settlement's changed-path census. | No check mutates candidate source or adds an unaccounted source path. An existing cache is read. |
| [Go JSON package pass admission][prior-go] | A positive package verdict can be contradicted by parsed output. | Process verdicts are faithfully retained here; they concern the wrong implementation generation. |

Prioritize a narrow Python cache/source conformance fixture before adding new policy or deliberation behavior. Its purpose is specific: real host execution should establish the behavior of the source recurse will retain, rather than merely establish that an older executable artifact passes the selected test.

## Sources
 
[pin]: https://github.com/theRebelliousNerd/codenerd/commit/cfe155e28bf5865920da16e716fb4bb9586e3f67
[root]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/agents.md#L1-L20
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/agents.md#L1-L15
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/Docs/journeys/10-forever-loop.md#L75-L129
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/gates/detect.go#L134-L164
[override]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/gates/detect.go#L15-L41
[gate-command]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/gates/run.go#L83-L99
[interpreter]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/gates/detect.go#L166-L172
[command]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/build/workspace_command.go#L30-L61
[environment]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/build/workspace_command.go#L18-L28
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_workspace.go#L461-L480
[git]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_git.go#L16-L19
[git-census]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_git.go#L79-L125
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle.go#L1052-L1118
[improve]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle.go#L1184-L1205
[after]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle.go#L1288-L1359
[tested]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle.go#L1414-L1447
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle.go#L1577-L1606
[metric-reader]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/gates/metrics.go#L88-L171
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/gates/run.go#L48-L99
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/gates/findings.go#L112-L119
[no-tests]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle.go#L232-L265
[fact-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_policy.go#L182-L255
[keep-rule]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/core/defaults/policy/recurse.mg#L259-L329
[metric-rules]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/core/defaults/policy/recurse.mg#L339-L398
[adopt]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle.go#L1481-L1507
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/cmd/nerd/cmd_campaign_recurse.go#L219-L271
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/cmd/nerd/chat/campaign_recurse.go#L218-L266
[attempt-contract]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_attempt.go#L33-L53
[detect-tests]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/gates/detect_test.go#L92-L105
[python-test]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle_e2e_test.go#L437-L502
[simplify-test]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/internal/campaign/recurse_cycle_test.go#L467-L510
[py-pin]: https://github.com/python/cpython/commit/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7
[py-skip]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/compileall.py#L212-L249
[py-validate]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/importlib/_bootstrap_external.py#L726-L753
[py-load]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/importlib/_bootstrap_external.py#L1090-L1172
[py-default]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/py_compile.py#L72-L76
[py-write]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/py_compile.py#L124-L172
[pep552]: https://peps.python.org/pep-0552/
[compileall-doc]: https://docs.python.org/3.13/library/compileall.html
[import-doc]: https://docs.python.org/3.13/reference/import.html#cached-bytecode-invalidation
[python-cli]: https://docs.python.org/3.13/using/cmdline.html#cmdoption-B
[pytest-rewrite]: https://docs.pytest.org/en/stable/how-to/assert.html#assertion-introspection-details
[prior-memo]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/Docs/audits/test-gate-memoization-identity.md
[prior-no-tests]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/Docs/audits/recurse-no-tests-evidence-continuity.md
[prior-witness]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/Docs/audits/recurse-workspace-test-witness.md
[prior-cargo]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/Docs/audits/recurse-cargo-default-member-evidence.md
[prior-metrics]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/Docs/audits/recurse-source-metric-acquisition-completeness.md
[prior-effects]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/Docs/audits/recurse-post-verification-effects.md
[prior-go]: https://github.com/theRebelliousNerd/codenerd/blob/cfe155e28bf5865920da16e716fb4bb9586e3f67/Docs/audits/recurse-go-json-package-pass-admission.md

