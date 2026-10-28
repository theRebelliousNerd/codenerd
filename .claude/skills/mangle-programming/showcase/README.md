# Showcase: what Mangle can really do

codeNERD itself is the premier Mangle showcase. Burak, Mangle's author,
has forked it. The examples the Mangle world lacks should be real
codeNERD behavior carried by Mangle-only features, not demos beside the
product. This directory indexes those features. Each entry points at
production code and gives a short runnable distillation a stranger can
learn from.

## Entry format (`showcase/<slug>.md`, under 100 lines)

1. **The claim**, in one sentence: what Mangle does in codeNERD that Go
   or plain Datalog couldn't do as clearly.
2. **Where it lives**: the Decls, rules, Go wiring and tests in the repo,
   as paths.
3. **The distillation**: the smallest `.mg` and lab test that shows the
   idea, complete enough to paste into `new-lab.sh <slug>` and run.
4. **The output**: rows or a proof copied from a real run on the pinned
   engine.
5. **What it replaced or enabled** in codeNERD, with the before and after.
6. **Cost**: rows, strata and time at production scale.

## Rules

- Only shipped behavior with tests, plus a distillation that was really
  run. A lab result that hasn't shipped is a backlog item, not an entry.
- Every new entry gets a link in SKILL.md's Showcase section, in the same
  change.
- No entries yet. The first Mangle-only feature to ship goes here.
