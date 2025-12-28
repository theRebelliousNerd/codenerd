---
description: "Start the codeNERD Perfection Loop - systematic stress testing and root-cause fixing"
argument-hint: "[--max-iterations N]"
allowed-tools: ["Bash(${CLAUDE_PLUGIN_ROOT}/scripts/setup-perfection-loop.sh:*)"]
hide-from-slash-command-tool: "true"
---

# codeNERD Perfection Loop

This launches a specialized Ralph Wiggum loop designed to bring codeNERD to 100% tested reliability.

Execute the setup script:

```!
"${CLAUDE_PLUGIN_ROOT}/scripts/setup-perfection-loop.sh" $ARGUMENTS
```

## What This Does

The perfection loop runs 14 systematic phases testing all 31+ codeNERD subsystems:

| Phase | System | Tests |
|-------|--------|-------|
| 1-5 | Core | Kernel, Perception, Session, Campaign, Safety |
| 6 | Integration | All logs clean |
| 7 | Demo App | Autonomous Mini-CRM creation |
| 8 | Ouroboros | Self-generating tools |
| 9 | Nemesis | Adversarial code review |
| 10 | Thunderdome | Attack arena sandbox |
| 11 | Dream State | Hypothetical exploration |
| 12 | Prompt Evolution | Self-learning |
| 13-14 | Integration | Full loop + verification |

## Root-Cause Mandate

When you find bugs, you MUST:
1. Trace to ROOT CAUSE using Five Whys
2. Fix the generation pipeline, not the artifact
3. Document in `.nerd/ralph/bugs/BUG-XXX.md`
4. Commit with `fix(<component>): <description>`

## Completion

Only output `<promise>CODENERD PERFECTION ACHIEVED</promise>` when:
- All 14 phases complete
- All logs clean (0 errors)
- Demo app builds and tests pass
- Ouroboros, Nemesis, Thunderdome, Dream State, Prompt Evolution all demonstrated
