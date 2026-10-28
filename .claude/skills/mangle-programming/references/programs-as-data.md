# Programs as data: Mangle that writes, checks and compares Mangle

A Mangle program is text that Go can parse, analyze, combine and evaluate
in milliseconds. That makes rules themselves a value Go can generate,
gate, version, diff and test. codeNERD already lets an LLM propose rules.
This file is about doing more with that.

## What exists in codeNERD

- The feedback loop (`internal/mangle/feedback`) sandbox-compiles
  LLM-proposed rules (`HotLoadRule`), and `checkCandidateBounds` type-checks
  each candidate strictly (`internal/core/kernel_rule_bounds.go`).
- The kernel puts schemas, then policy, then learned rules into one
  program on every rebuild. That order is part of the trust model.

## Ideas

- **Policy regression check (the "bisimulation" idea from the Vectryx
  skill)**: evaluate the old policy and the new policy over the same EDB,
  then ask Mangle "is anything permitted in v2 that v1 denied?" To share
  one program, prefix v1's predicates (`v1_permitted`) in Go before
  parsing. Every policy PR gets a semantic diff, not a text diff.
  Candidate: a test over the real corpus at HEAD versus HEAD~1.
- **Packages for provenance of rules**: `Package learned!` with
  `Use policy!` would put generated rules in their own namespace, so a
  learned rule can read policy but can't be mistaken for it. Probe how
  `Package`/`Use` resolve names on v0.7 before you design on it.
- **Rule templates from Go**: generate families of rules from typed Go
  data (one routing rule per registered shard) instead of hand-copying
  them. The generator owns the shape; the `.mg` holds the decisions.
  Check the output with the strict analyzer at build time.
- **Mutation testing for policy**: drop or negate one premise per rule,
  re-evaluate the fixture EDB, and see whether any test notices. A
  surviving mutant is a rule no test pins.
- **Learned-rule quarantine**: evaluate a candidate rule in a copy of the
  kernel over recent real EDB, and diff which decisions change. Promote it
  only if the diff matches the intent the LLM stated. Proofs
  (`proofs-as-data.md`) show which existing decisions it would take over.
- **Self-description**: `rule_source(RuleID, Text)` and `uses_rule` from
  provenance, plus `descr [doc(...)]`, let a program list its own rules,
  docs and usage. Dead-rule detection becomes a Mangle query over
  `uses_rule` tallies.
- **Minimal reproducers**: shrink a failing EDB fact by fact in Go,
  re-evaluating each time, until the wrong derivation disappears. The
  last fact removed is the cause.

## Guardrails

- Never splice untrusted text into a program by string concatenation
  without parsing it back and checking the predicate set it defines. A
  candidate must not define `permitted`, or anything else policy owns.
- Every generated program goes through parse, strict analysis and
  stratification before it reaches the kernel. A parse error anywhere is a
  boot failure everywhere.
- Rebuild cost: `rebuildProgram` parses the whole program each time.
  Measure before generating hundreds of rules.
