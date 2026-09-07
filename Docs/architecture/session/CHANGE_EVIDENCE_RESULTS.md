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
