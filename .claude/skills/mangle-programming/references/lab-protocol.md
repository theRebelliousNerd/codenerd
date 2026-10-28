# Lab protocol: experiment first, then build

Every Mangle mission starts as an experiment that tries one Mangle-only
feature on the actual problem. The experiment is cheap and disposable, and
it decides the design. Production code comes second.

## Scaffold

```bash
.claude/skills/mangle-programming/scripts/new-lab.sh route_lattice
go test -v ./_mangle_lab/route_lattice/
```

- `_mangle_lab/` is gitignored. Go skips `_` directories in `./...`, so a
  lab never reaches the real build or CI.
- The script copies `scripts/lab/harness.go` into `_mangle_lab/harness/` on
  every run. To improve the harness, edit the skill's copy.
- Labs live inside the `codenerd` module, so a lab can import any
  `codenerd/internal/...` package. For example, it can boot `core.NewRealKernel`
  next to the bare engine and compare the two.

## The harness

`harness.Run(t, src, opts...)` parses, analyzes and evaluates with
everything switched on:

| Default | Why |
| --- | --- |
| Strict Decl bounds (`ErrorForBoundsMismatch`) | The kernel skips them; the lab shows what they would catch |
| Temporal store and `now` | Temporal facts and operators just work |
| Derivation recorder | `r.Why("goal(...)")` prints the proof; `r.ProofFacts` returns it as facts |
| Deterministic order, 100k fact limit | Reproducible runs and the kernel's circuit breaker |

Options: `LooseBounds()` (analyze the way the kernel does), `At(time)`
(pin `now`), `FactLimit(n)`, and `WithExternal(name, arity, fn)` (a Go
function behind a predicate declared `descr [external(), mode(...)]`).
Use `harness.Try` when the experiment is about an error.

## The loop

1. **Name the hypothesis** in the template header: which Mangle-only
   feature does this job better than Go code or flat Datalog, and why.
2. **Build the smallest program that could falsify it**: one expected row,
   one near miss, and the shape the production data will have.
3. **Run, then read `r.Why`** for the surprising rows. A surprise is the
   finding.
4. **Push once more.** Combine it with a second unique feature (a lattice
   over structs, temporal facts with proofs, an external with a merge) and
   measure what that changes.
5. **Write the verdict** in the header: worked, failed or surprised, plus
   the reason, and the cost (rows, strata, time from `r.Stats`).
6. **Record it.** Add one memory per real finding (see "What to remember"
   below), including dead ends. A failed idea with its reason is worth as
   much as a win.
7. **Promote or drop.** Promotion means Decls in `schemas_*.mg`, rules in
   `policy/`, Go wiring at the real boundary, and a kernel test with a
   positive case and a near miss. The lab directory itself never ships.
8. **Showcase it** once the promoted feature ships and would surprise
   someone who knows Datalog. Write `showcase/<slug>.md` in the skill
   (format in `showcase/README.md`) and link it from SKILL.md.

## What makes an experiment good

- It answers "what can Mangle do here that Go can't do as clearly?" It
  doesn't re-solve a solved problem in new syntax.
- It runs at the scale production will see. Generate 10k–1M rows in Go
  inside the test when the question is cost.
- It compares against the current implementation when one exists: same
  inputs, both answers, and a diff.
- It ends in a decision, not a demo.

## What to remember (agent memory)

Record what took a run to learn:
- an engine behavior the docs get wrong;
- a feature combination that worked, with its cost;
- an idea that failed, with the reason;
- a codeNERD seam where a unique feature fits.

Don't record syntax the skill already states, or anything visible in the
code.
