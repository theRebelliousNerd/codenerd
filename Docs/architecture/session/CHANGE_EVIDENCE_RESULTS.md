# Local acceptance experiment, 2026-09-07

One private Go fixture reproduced the Windows drive-letter parsing failure.
`TestWindowsPath` failed before editing; `TestUnixPath` passed. All modes began
with the same content snapshot (`e4e9a6a77197e6a1a41517431c0e30761388f99078641aaafe186fc93e40948d`).
Model: `muse-spark-1.3-contributor`; limits: 10 model calls, 16 tool calls,
2,048 output tokens per call, five-minute invocation deadline. No fixture was
manually repaired during a run.

| Run | Tokens | Execution seconds | Independent acceptance | Host completion |
|---|---:|---:|---|---|
| Minimal loop | 28,174 | 41.33 | verified | complete |
| Ordinary codeNERD, final | 170,196 | 96.19 | verified | checks passed; acceptance unspecified |
| Contracted codeNERD | 164,045 | 84.79 | verified | `/done`, current witnesses persisted |

Execution time includes production boot, model work and in-path verification;
it excludes the common external baseline/grader. These are single observations
on a shared Windows workstation, not statistically controlled latency estimates
or general completion-rate claims. The contracted path spent substantially more
tokens than the minimal loop; this experiment does not establish an efficiency win.

Two earlier ordinary runs remain part of the development cost: 239,996 tokens
for a correct edit rejected by inconsistent newline validation, and 61,132 for
a deadline failure during concurrent compilation. Neither received host `/done`.
Across all five invocations: 663,543 tokens. The earlier verifier also exposed
unstable environment identity, fixed by pinning the process environment and
normalizing Go's generated temporary prefix-map path.

The 48,000-fact production-corpus delta benchmark returned the expected derived
answers in both modes: full evaluation 0.984 s, bounded differential selection
0.998 s (one iteration each). This establishes a local regression witness for
avoiding the reported 91-second first delta, not a universal latency bound.

A separate smoke run of the actual `nerd fix --acceptance` CLI also passed,
including task cloning and the normal JIT/Piggyback route. It persisted verified
acceptance at snapshot `a12fa941450e`, executed eight tools and recorded 323,307
session tokens. This run is outside the controlled comparison above.

Remaining uncertainty: broader/private task distributions, false-completion and
human-rescue rates, noisy-machine latency, and task-specific test relevance need
more observations. The tests witness the requested parsing behavior; they do
not prove every new branch correct.

## Self-upgrade marathon checkpoint, 2026-09-07

Persistent token-efficiency specialists were created through init, consulted on
local source guides, and exercised in fresh processes after their own methodology
was persisted. This establishes reuse of prompt knowledge, not automatic success
of proposed optimizations. The broader 54-task campaign remains in progress.

A production-state CPU profile found predicate scans dominating kernel evaluation.
Large full-fixpoint evaluations now use argument-indexed storage. The comparison
in [kernel observations](token_efficiency_kernel_observations.json) ran indexed
first and the scan baseline afterward against the same workspace:

| Observation | Scan | Indexed |
|---|---:|---:|
| Boot | 232.60 s | 15.50 s |
| First next_action query | 32.51 s | 1.36 s |
| Heap allocated at boot sample | 292.33 MB | 330.04 MB |
| Cumulative allocation through boot | 4.58 GB | 5.25 GB |

Both returned identical hashes for all 4,167 file-topology and file-existence
facts. Total EDB counts were about 55,500 and differed by 19 transient facts.
A separate regression compares the entire production-corpus closure with scan
storage before and after retraction. A bound-lookup microbenchmark measured
361,376 ns/op versus 378 ns/op, with 24 B/op in both cases; index construction is
outside that microbenchmark and its memory cost is included in boot observations.

These are shared-workstation observations, with sampled heap rather than peak
RSS. They establish neither a token-saving result nor a 10M-LOC scale result.
Campaign host checks, native Windows verification, and TDD now distinguish actual
execution outcomes from model prose. The full marathon and matched coding-task
comparisons remain open; completed-turn counts are not acceptance evidence.

### Persistent knowledge and measured learning

A subsequent force-init live run preserved all 11 curated expert prompt files
byte-for-byte and all 398 grounded knowledge records; 116 enrichment calls
succeeded. Five token-efficiency experts were consulted and their methodology
reused in fresh processes.

Live probes then exposed three retrieval gaps: expert databases were absent from
knowledge lookup, delegated task text was absent from the retrieval query, and
ephemeral lookup results were discarded unless present in the prompt-vector
index. The repair carries bounded expert/project lookups through a distinct
`retrieved_context` witness into Mangle selection, preserving context, conflict,
dependency and budget checks. A production-factory test fails against the prior
source and passes with the repair, including a sibling-isolation control.

New learnings now receive sanitized lexical handles immediately; legacy records
remain recallable before embedding. Recall preserves source, confidence and
timestamp, honors cancellation, and reinforcement preserves descriptor/embedding
identity. Fresh live experts retrieved a source-only control with its exact
concept and confidence, then reused measured kernel latency and memory costs.
An initially overlong learning lost its trailing limitations at the existing
600-character descriptor boundary; replacing it with a bounded capsule preserved
both measurements and uncertainty. Raw records and earlier failed probes remain
in the local marathon artifacts.

### Matched coding trials: overhead remains substantial

[Twelve trial results](token_efficiency_prompt_trials.json) use three small Go
regressions, the same model and call limits, fresh workspaces, and caller-owned
acceptance tests whose contents must remain unchanged.

| Condition | Accepted | Total tokens, including failed attempts |
|---|---:|---:|
| Minimal tool loop | 3/3 | 114,630 |
| Earlier production executor | 2/3 | 512,743 |
| Updated production executor | 3/3 | 597,110 |
| Updated executor with acceptance contract | 2/3 | 547,540 |

No false completion was observed in these twelve runs. This is one invocation
per task/condition, with provider and host variability; it does not establish
causal token savings. Setup, expert consultation and earlier repair costs are
separate and must enter whole-marathon totals. One successful tiny-task prompt
contained 78 atoms and about 30,000 tokens, including guidance for tools absent
from its five-tool catalog. The experts identified capability-aware prompt
selection as the next controlled improvement. The broader campaign remains open.
