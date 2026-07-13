# Vectryx Full Claude Delta Sync

## Context

The user requested a full pull of the upgraded Vectryx Claude workspace into Codex, with `corpus-build` and `arch-propose` called out as priorities. The source `.claude/` tree remained read-only.

## Evidence

- Repository inventory: 133 Claude agents, 68 `.agents/skills` packages, 67 `.codex/skills` packages, 58 duplicate names, and 19 divergent duplicate `SKILL.md` files.
- The prior successful porter baseline was the 2026-07-09 run. Git history after that baseline identified the operational delta in corpus-build, arch-templates, hooks, active evolution evidence, and shared agent memory.
- Current Codex documentation confirmed project-scoped skill, custom-agent, and command-hook contracts. The local runtime was `codex-cli 0.144.1`.

## Decisions

- Preserved existing `.codex/skills` ownership for governed Vectryx packages and left disabled `.agents/skills` compatibility duplicates untouched.
- Rehomed the Claude arch-propose command into `.codex/skills/arch-propose/references/orchestration.md` and converted its eight plugin agents into governed Codex TOMLs.
- Preserved `.claude/agent-memory/` as the shared Claude and Codex memory corpus.
- Ported corpus lifecycle telemetry without inferring token totals from unstable parent transcripts.

## Intentional Non-Parity

- Exact per-subagent token metering is unsupported because Codex does not expose a stable bounded subagent transcript path in the hook contract. The stop hook records explicit usage only when supplied by the runtime.
- The todo hook can migrate checked items after Codex apply-patch writes; ensuring `to-done.md` on a pure file read has no equivalent direct file-read event in the current hook contract.
- Claude agent-local enforcement hooks remain unactivated pending their required trust and policy checkpoint. `PreToolUse` would be a guardrail, not a complete security boundary.
- Unchanged commands, rules, prompts, plugin bundles, local settings, historical interrogations, and roadmap telemetry were not recopied.

## Validation

Validation results:

- Package support closure: `corpus-build` 20/20 paths, `arch-templates` 11/11, `vectryx-evolve` 160/160; `arch-propose` has the four source package paths plus the intentional Codex-owned `references/orchestration.md`.
- All 141 agent TOMLs and `.codex/config.toml` parsed; all eight `arch-propose-*` names resolve through governed registry entries to their real config files.
- `.codex/hooks.json` and `surfaces.yaml` parsed; all four configured hook command paths exist from the Git root.
- PowerShell syntax passed for all three migrated handlers. Redirected-stdin behavior probes passed for todo migration and corpus start/stop telemetry, including explicit `billable_total=21` verification.
- `quick_validate.py` passed for all four touched skill packages. Advisory warnings remain for pre-existing package hygiene only.
- `codex debug prompt-input` exposed `arch-propose` and `corpus-build` exactly once from `.codex/skills`, exposed neither disabled `.agents/skills` duplicate, and included the end of root `AGENTS.md`.
- `git diff --check` passed. Project config marks Vectryx trusted and keeps hooks enabled.

Rubric: surface completeness 5/5; behavioral fidelity 4/5; runtime activation 4/5; safety and ownership 5/5; validation quality 5/5; evidence honesty 5/5. Average: 4.67/5, with no dimension below 3.

## Reusable Lessons

When a Claude skill delegates agents from a plugin, the support closure includes both the plugin agent prompts and the command orchestration file even when neither lives under the source skill directory.
