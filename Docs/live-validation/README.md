# Live validation evidence

In-repo home for **codeNERD live stress / campaign results**.

Earlier sessions dumped evidence only into `%TEMP%\codenerd-*`, which is invisible in the project and disappears across machines. That was a process mistake.

## Layout

| Path | What |
|------|------|
| [`polystack/`](polystack/) | Multi-language vehicle + marathon/CLI matrix evidence |
| `polystack/AGENTS.md` | How agents should treat that vehicle |
| `polystack/LIVE_TEST_RESULTS/` | Reports, stdout, subagent findings |

## Rule for agents

When you run live `nerd` stress or campaigns, **write evidence under `Docs/live-validation/`** (and optionally mirror next to the vehicle). Do not leave results only in OS temp.
