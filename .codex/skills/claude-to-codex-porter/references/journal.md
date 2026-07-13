# Claude to Codex Porter Journal

## 2026-07-13 (Vectryx to codeNERD ecosystem port)

Observed failure mode:

- Copying the Vectryx corpus and architecture fleets verbatim carried Storyworld, REST/RBAC, campaign-narrative, private model alias, and nonexistent test-remediation path assumptions into codeNERD.
- Directory parity alone would not activate the roles: codeNERD also needed custom-agent TOMLs, config registration, skill attachments, SubagentStart/Stop hooks, memory context routing, and registry updates.

Correction:

- Classified Vectryx as the read-only source and codeNERD as the target, copied reusable packages, then rewrote active orchestration around codeNERD's perception-to-Mangle-to-VirtualStore fact flow, JIT prompt atoms, constitutional permissions, current directories, and official current model names.
- Registered dedicated arch-propose and corpus-build fleets with non-recursive worker instructions, shared subagent memory injection, and explicit lifecycle telemetry that never infers unavailable token usage.
- Preserved historical scenario/interrogation evidence and deliberately declined stale enforcement hooks whose repository assumptions conflicted with codeNERD's current build and test contract.

Evidence:

- Parsed all new agent TOMLs and `.codex/config.toml`, validated hooks JSON and Python syntax/behavior, ran porter inventory tests, and checked fleet registration/skill attachment parity.
- Upgraded the stress-tester package as a second live port-quality probe, replacing stale paths and unverifiable counts with bounded profiles and measured receipts.

## 2026-06-28 (Skill support closure for scripts and assets)

Observed failure mode:

- A skill migration can appear complete after `SKILL.md` and references are
  merged while active `scripts/` and `assets/` remain source-only or are skipped
  as incidental files. This is especially risky for verifier/oracle skills where
  the entrypoint delegates real behavior to helpers and package-local assets.
- After `.codex/skills` was retired, the old `.agents` guard also blocked the
  new canonical `.agents/skills` target while telling agents to use it.

Correction:

- Treat `scripts/`, `assets/`, metadata, `agents/openai.yaml`, and active
  skill-local runtime dirs as first-class skill package support surfaces in
  scope, inventory, transform, validation, and reporting.
- Retarget the guard to block only the retired `.codex/skills` root; do not
  block `.agents/skills`.
- Remove obsolete `.codex/config.toml` skill-disable blocks that targeted
  `.agents/skills`.

Evidence:

- Migration repair: 2026-06-28 porter v2.3.1 update.
- Validation target: stale porter guidance scan for `.agents` disable/block
  language; `.codex/config.toml` parse after removing 350 obsolete disable
  blocks; `py_compile` and JSON-stdin probes for
  `.codex/hooks/block-agents-directory-access.py`; package metadata read-back.

## 2026-06-28 (Canonical skill root consolidation)

Observed failure mode:

- Once `.agents/skills` is the canonical repo-local skill root, deleting
  `.codex/skills` can still leave active `.codex/agents`, `.codex/rules`, and
  `.codex/hooks` instructions pointing at the retired package root. Skill
  package parity checks alone do not catch those downstream policy surfaces.

Correction:

- During root-consolidation migrations, audit active skill, agent, hook, and
  rule surfaces for stale `.codex/skills` references after the package merge.
  Preserve historical references only in journals, changelogs, brainstorms,
  peer reviews, and metrics evidence.
- Treat `.skills/` as shared runtime/work output unless the user explicitly
  asks to migrate it.

Evidence:

- Migration run: 2026-06-28 `.codex/skills` to `.agents/skills` manual merge
  and `.codex/skills` deletion.
- Validation: scoped stale-reference audit over `.agents/skills` and `.codex`
  active surfaces returned no matches; `.codex/agents/*.toml` parsed with
  `tomllib`; `.codex/hooks/block-agents-directory-access.py` passed
  `python -m py_compile`.

## 2026-06-27 (Copied Markdown line-ending hygiene)

Observed failure mode:

- Copying Claude skill support Markdown into `.codex/skills` can preserve CRLF
  line endings or trailing whitespace in generated journal/reference files.
  The content reads correctly, but `git diff --check` flags the copied lines.

Correction:

- After mechanical skill-directory copies, normalize touched Markdown line
  endings/trailing whitespace before declaring the slice validated.

Evidence:

- GLM scout companion skill sync: `git diff --check -- .codex\skills\glm-formulate*`
  initially flagged four copied `references/journal.md` files; LF normalization
  cleared the check.

## 2026-06-27 (GLM-formulate and current Codex manual refresh)

Observed failure mode:

- The porter still described hooks as Windows-disabled and routed shared Claude
  slash-command content toward checked-in prompt surfaces, but the current Codex
  manual says hooks are enabled by default and slash commands import to skills.
- A real GLM-formulate slice required coordinated skill packages, custom-agent
  TOMLs, config registration, and a command hook. Treating any one surface as
  sufficient would have under-ported the ecosystem.

Correction:

- Updated hook guidance to use `[features].hooks`, command hook handlers,
  `commandWindows`/`command_windows` for Windows overrides, JSON-stdin probes,
  and explicit unsupported-gap notes for prompt, agent, or async handlers.
- Updated command/prompt routing so shared reusable commands become skills by
  default, while deprecated custom prompts require an explicit user-local
  exception.
- Added current skill/subagent model and schema notes, including `gpt-5.5` as
  the default demanding-agent choice and repo-local `gpt-5.4-mini` handling.

Evidence:

- Official manual refresh: `node C:\Users\smoor\.codex\skills\.system\openai-docs\scripts\fetch-codex-manual.mjs`
- GLM validation: `python -m py_compile` for the new hook; `hooks.json` parse;
  TOML parse for GLM agents and `.codex/config.toml`; hook behavior probe
  emitted routing context for a KILL-bearing GLM experiment file and no output
  for an unrelated file.

## 2026-06-04 (Codex-only skill target and .agents guard)

Observed failure mode:

- The porter still treated `.agents/skills` as the active Codex target, which can
  mutate the shared external skill tree when the requested deliverable is a
  `.codex`-only skill and agent port.

Correction:

- Codex-native skill ports in this repository target `.codex/skills/<name>/`.
- `.codex/config.toml` disables every `.agents/skills/<name>` folder path.
- `.codex/hooks/block-agents-directory-access.py` is wired in `.codex/hooks.json`
  and denies tool calls that read from or write to `.agents/`.

Evidence:

- Migration repair run: 2026-06-04 Socratic Innovation Partner Codex-only import.
- Validation: TOML parse for `.codex/config.toml` and touched agent TOML;
  `python -m py_compile` for the guard hook; JSON-stdin behavior probe for a
  blocked `.agents` command.

## 2026-03-23

Initial skill creation.

Observations:

- This repository already uses `.agents/skills` as its active local skill surface even
  though the current official Codex docs describe repo-scoped skills under
  `.agents/skills`.
- Existing `.codex/agents/*.toml` files preserve source provenance in comments and
  map source agent bodies into `developer_instructions`.
- The checked-in `CLAUDE_METHODOLOGY_SKILL_PORT_PROMPT.md` is a strong repo-local
  exemplar for how the user expects migrations to be scoped, rewritten, and validated.

## 2026-03-23 (follow-up)

The first cut underfit the actual sophistication of this repo's Codex surface.

Corrections:

- Treat `.codex/config.toml` as a first-class part of agent migration because this
  repo explicitly registers many agents there.
- Capture local agent archetypes, not just the official OpenAI schema, so future
  ports can match read-only explorers, journaled writers, skill-attached agents,
  and deep workers.

## 2026-03-23 (second follow-up)

The repo is registry-complete for custom agents, not merely registry-aware.

Corrections:

- Add an explicit discovery-vs-registration policy so the migration skill knows this
  workspace is hybrid and should update both TOML files and config blocks.
- Add a migration surface checklist so future ports cannot stop after copying the
  obvious files while missing runtime dirs, journal roots, config entries, or
  preserved-evidence decisions.

## 2026-04-22

Observed failure mode:

- The previous workflow still behaved like a path rewrite tool. It allowed
  Claude-style path-scoped Markdown rules to collect under `.codex/rules/`, even
  though Codex `.rules` files are Starlark command execution policies.
- Hook migration also needed sharper validation: a migrated hook is not real
  unless the Python script exists, `.codex/hooks.json` points at it, and the
  relevant event surface is supported or explicitly marked as a gap.

Correction:

- Added `codex-surface-decision-tree.md`.
- Refactored `SKILL.md` into a roadmap-grinder-style phase router.
- Added `references/phases/**/phase.md` files with explicit gates for scope,
  inventory, transformation, validation, and reporting.
- Added validation requirements for `.rules`, hooks, `AGENTS.md`, and stale
  Claude-style Markdown under `.codex/rules/`.

## 2026-04-22 (app pipeline handoff migration)

Observed failure mode:

- Copying app pipeline skills from Claude can preserve stale plugin-surface
  assumptions even after `.claude/skills` and `.claude/agent-memory` paths are
  correctly repointed. The app-implement package still referenced
  `.claude/commands`, `.claude/settings.json`, `CLAUDE_PLUGIN_ROOT`, and
  `plugin-dev:create-plugin`, which are not Codex-native handoff targets.

Correction:

- During app-pipeline ports, scan active skill and agent bodies for old plugin
  surfaces separately from source-provenance comments. Rehome plugin assets to
  `plugin/` / `.codex-plugin/plugin.json` language and replace plugin scaffolding
  delegation with `plugin-creator`.

Evidence:

- Migration run: `2026-04-22_sachs_benchmark`.
- Validation: `tomllib` parse of 21 app-discover/app-implement agent TOMLs plus
  `.codex/config.toml`; `bash -n` for copied shell scripts; stale-reference
  `rg` showed remaining `.claude` hits only in source-provenance comments.

## 2026-05-17 (bulk skill/agent uplift)

Observed failure mode:

- Bulk package ports can accidentally copy `references/` updates while skipping
  the root package files (`SKILL.md`, `CHANGELOG.md`, `Claude.md`) if the source
  path filter treats `.claude/skills/<name>/SKILL.md` as a one-part relative path.
  This leaves fresh deep references behind stale routers.
- Active Codex skill docs can inherit Claude Markdown rule dependencies such as
  `.claude/rules/r2s-depth-and-readiness.md`. These are not Codex `.rules`
  targets; they must be rehomed into owning skill references or `AGENTS.md`.

Correction:

- Treat package-root files as first-class active skill surfaces during inventory
  and compare them alongside `references/`, `scripts/`, and `assets/`.
- For Claude rule content that is operational policy for a skill, create a
  Codex-native reference under the owning skill and repoint active docs there.
  Preserve `.claude` paths only in changelogs, journals, and source-provenance
  comments.

Evidence:

- Migration run: 2026-05-17 `.claude` skills/agents to `.codex` parity pass.
- Validation: `tomllib` parsed `.codex/config.toml` and all 116 agent TOMLs;
  changed/new Python scripts passed `py_compile`; touched shell scripts passed
  `bash -n`; stale-reference audit reported zero active `.claude`/agent-memory
  hits after preserved evidence was excluded.

## 2026-05-18 (R2S Claude update pull-in)

Observed failure mode:

- R2S skill packages can be fully translated from `.claude/skills` to
  `.agents/skills` while still retaining legacy `.skills/` runtime paths for
  manifests, metrics, and journals. These paths are active instructions, not
  preserved evidence, so they must be translated during Codex-side sync.

Correction:

- Add `.skills/` and `.skills\` to the R2S migration stale-reference audit and
  rewrite them to `.agents/skills/` for active skill, agent, and journal surfaces.
- Treat `.claude/agents/...` in generated TOML `# source:` comments and old
  agent-journal incident notes as preserved evidence, not active paths.

Evidence:

- Migration run: 2026-05-18 R2S update sync from Claude to Codex.
- Validation: `tomllib` parsed 14 R2S agent TOMLs plus `.codex/config.toml`;
  R2S helper scripts passed `python -m py_compile`; scoped stale-path audit
  found no active `.skills/` references and only preserved `.claude` evidence.

## 2026-05-18 (Codex agent-schema correction)

Observed failure mode:

- A Claude-shaped migration can produce TOML that parses but is not Codex-native:
  `[[skills.config]] name = "..."` is not a valid skill attachment, and any
  following `developer_instructions` remains scoped inside that array table.
- Several custom-agent files also lacked the required top-level `name` or
  `description` fields even though registry entries existed.

Correction:

- Agent and skill edits must cross-check the current OpenAI Codex docs or schema
  before writing. Treat Claude frontmatter and local exemplars as source behavior,
  not target-format authority.
- Codex custom agents require top-level `name`, `description`, and
  `developer_instructions`; `[[skills.config]]` uses `path` to the skill folder
  plus `enabled = true`.

Evidence:

- Official docs checked: `https://developers.openai.com/codex/subagents` and
  `https://developers.openai.com/codex/config-reference#configtoml`.
- Validation added: required-field audit, `skills.config` shape audit, missing
  skill-folder audit, TOML parse, and scoped `git diff --check`.

## 2026-06-27 (Proof Forge verifier skill port)

Observed failure mode:

- A copied skill can contain a package-local verifier script with a hardcoded
  source skill root, so `py_compile` alone is not enough to prove the migrated
  package will read its Codex-side assets.
- Rehoming active Claude rule dependencies into a skill-local `references/rules/`
  copy may change `.nl` oracle comments and therefore change generated honesty
  card hashes under shared `.skills/` runtime output.

Correction:

- For skill packages with verifier scripts, patch package-root constants and
  command examples to `.codex/skills/<name>/`, then run one side-effect-bounded
  helper command that actually reads the migrated assets.
- Keep `.skills/<name>/` runtime metrics/cards as shared run output when the
  source workflow uses them, but refresh generated cards when the migrated
  oracle source hash changes.

Evidence:

- Migration run: 2026-06-27 `proof-forge-discovery-run` direct skill package port.
- Validation: `python -m py_compile` for the distiller; `nlc check` for both
  honesty `.nl` assets; `nlc saturate` derived `falsifier_passed(yes)`; the
  migrated distiller rendered the shared honesty card from the `.codex` package.

## 2026-06-27 (Frontier Discernment verifier skill refresh)

Observed failure mode:

- Verifier skills may already have a tracked Codex copy, but the working tree
  can still need a runtime refresh because shared `.skills/<name>/` cards embed
  the migrated oracle hash.
- Skill-evolution docs and distillers must agree on the current metrics surface;
  for frontier-discernment that surface is the append-only
  `.skills/frontier-discernment/metrics-log.jsonl`, not the retired
  `.last-metrics.json` snapshot.

Correction:

- Treat Codex verifier package checks as source-plus-runtime: verify the
  `.codex/skills/<name>/` script root, run the `.nl` falsifier, and run a real
  sample or render command that refreshes the shared card from the Codex assets.
- Keep `.skills/<name>/` as shared runtime output unless the owning workflow has
  explicitly migrated those telemetry artifacts.

Evidence:

- Migration run: 2026-06-27 `frontier-discernment` Codex package refresh.
- Validation: `python -m py_compile` for the distiller; `nlc check` for the
  oracle and falsifier; `nlc saturate` derived `falsifier_passed(yes)`; the
  migrated distiller returned `gate_verdict: green` on the real
  `experience_metabolism_executive_loop` sample and refreshed the shared card.

## 2026-06-27 (Shared-memory guard canonical text)

Observed failure mode:

- The Codex agent shared-memory guard may immediately rewrite agent TOMLs after
  an edit. Its canonical block points agents at `.claude/agent-memory/<agent>/`
  and uses Markdown-link `MEMORY.md` indexes, while also naming the retired
  `agentJournals` root only in "supersedes" / "never create" negative prose.

Correction:

- Do not fight the guard by hand-editing that canonical block away. During
  validation, classify the guard's negative old-root mentions as intentional
  exception text, while still treating `.Codex/agent-memory`, active
  Codex-only memory roots, or non-guard `agentJournals` instructions as stale.

Evidence:

- Migration run: 2026-06-27 `frontier-discernment` companion-agent refresh.
- Validation: `tomllib` parsed `.codex/agents/frontier-discernment.toml`; the
  guard-normalized block retained only shared `.claude/agent-memory/...`
  instructions and the old root appeared only in negative guard prose.

## 2026-06-27 (Spec-to-Team orchestrator skill port)

Observed failure mode:

- Orchestrator skills are especially prone to "copied but not cloned" drift:
  the source package can look like a normal skill copy while its active routing
  still names Claude-only agent Markdown, Claude slash-command syntax, and
  Claude hook paths.
- Runtime `.skills/<name>/...` paths can be active and correct, while
  `.claude/agents` / `Skill(...)` / Claude hook references in the same package
  are stale target-surface instructions. They must be separated instead of
  blanket-rewritten.

Correction:

- For orchestrator/router skills, translate the dispatch vocabulary and roster
  verifier together: `.codex/agents/<name>.toml` for agents, `.codex/skills` for
  skill existence checks, `$skill` for skill handoffs, and Codex built-in
  subagent names (`default`, `worker`, `explorer`) for built-ins.
- Validate by checking declared spawned agents resolve to existing Codex TOML
  files, not just by reading the copied Markdown.

Evidence:

- Migration run: 2026-06-27 `spec-to-team` direct skill package port.
- Validation: exact stale-token scan over active spec-to-team docs; YAML parse
  of `assets/team_plan.template.yaml`; spawned-agent resolver check reported
  `spawned_agents 9` and `missing []`; touched agent TOMLs and config parsed.

## 2026-06-27 (VectryxDB ontology skill port)

Observed failure mode:

- Domain skills can carry newer live-verification evidence in their own
  journals while the entrypoint still teaches the older workflow. A literal copy
  would preserve stale target behavior even though the source package already
  contains the correction.
- Legacy skill identities can differ between directory and frontmatter names;
  Codex repo-local packages should use the package directory name as the
  activation identity and keep old aliases only as trigger text.

Correction:

- During port, reconcile source-internal journal evidence into the Codex copy
  when it affects active workflow instructions. For `vectryxdb-ontology`, that
  meant `POST /packs`, proposal-based candidate preview/review, and explicit
  `/simulate` fixture boundaries.
- Check top-level routing tables for missing Claude-era sibling aliases and
  replace them with repo-local or currently available Codex skill names.

Evidence:

- Migration run: 2026-06-27 `vectryxdb-ontology` Codex skill package port.
- Validation target: exact stale-token/API scan over the Codex package; Markdown
  parse and `git diff --check` over the migrated skill plus porter journal.

## 2026-06-27 (VectryxDB developer skill port)

Observed failure mode:

- Reference-heavy guide skills can appear structurally copied while their
  progressive-disclosure table leaves large reference files unreachable.
- Legacy Vectryx skill aliases (`vectryx-clients`, `vectryx-mcp-agent`,
  `vectryx-ai-frameworks`) no longer match the Codex skill surface exposed in
  this workspace.

Correction:

- Port identity should follow the repo package directory (`vectryxdb-developer`)
  and the entrypoint's "load next" table should route every content reference,
  not only the headline subset.
- Translate sibling-skill redirects to current Codex skill names during port.

Evidence:

- Migration run: 2026-06-27 `vectryxdb-developer` Codex skill package port.
- Validation target: stale-alias scan, frontmatter directory-name check,
  reference-route coverage check, and `git diff --check`.

## 2026-06-27 (Requirements Interrogator crew fork port)

Observed failure mode:

- Crew forks can depend on shared source-skill oracle assets. Porting the fork
  alone leaves live instructions calling back into `.claude/skills/...`.
- Active project-guidance wording may mention `CLAUDE.md` as a source-preamble
  gate even when Codex's current governing file is `AGENTS.md`.

Correction:

- Port dependent shared oracle assets/scripts into the Codex source skill and
  patch scripts to read `.codex/skills/...` paths.
- Keep shared `.skills/<name>/...` runtime outputs as shared outputs, but
  translate active invocation syntax to `$skill` and target self-improvement at
  the ported crew fork when the fork owns the behavior.

Evidence:

- Migration run: 2026-06-27 `requirements-interrogator-crew` Codex skill
  package port plus shared `requirements-interrogator` oracle asset parity.
- Validation target: stale active-token scan, script py_compile/render-card,
  frontmatter/spawned-agent resolution, and `git diff --check`.

## 2026-06-27 (RSI mission crew port)

Observed failure mode:

- Claude Code experimental agent teams are not schema-equivalent to Codex custom
  agents. `TeamCreate`, `SendMessage`, teammate mailboxes, and
  `~/.claude/tasks/<team>/` cannot be copied as if they were active Codex
  runtime primitives.
- Crew packages also mix executable role bodies with historical self-review
  evidence. A blanket stale-token rewrite would erase useful provenance, while
  a blind copy would leave Claude-only primitives in active instructions.

Correction:

- Port the crew as a Codex skill plus registered `.codex/agents/rsi-*.toml`
  role wrappers. The parent Codex session owns dispatch, peer relay, synthesis,
  and durable notes in `docs/mission-progress/`.
- Patch active role bodies to use `$skill` activation and Codex parent relay,
  while labeling Claude team matrix / peer-review files as preserved evidence.
- State unsupported persistent-team/mailbox semantics explicitly instead of
  implying feature parity.

Evidence:

- Migration run: 2026-06-27 `rsi-mission-crew` Codex skill package port with
  six custom role agents (`rsi-analyst`, `rsi-cross-machine-scout`,
  `rsi-fluid`, `rsi-crystal`, `rsi-ideator`, `rsi-interrogator`).
- Validation target: TOML parse for new role agents plus config, frontmatter
  check, active stale-token scan limited to intentional unsupported-gap notes,
  registry resolver check, and whitespace/line-ending scan.

## 2026-06-27 (Socratic Innovation Partner crew fork port)

Observed failure mode:

- Crew forks can look like ordinary skill copies but carry active team-channel
  assumptions (`SendMessage`, slash-command routes, `.claude/skills/...` path
  anchors) inside their entrypoint and reference library.
- Reference-library path rewrites can create dangling Codex links if the support
  rules were not also cloned.

Correction:

- Translate active fork instructions to Codex `$skill` recommendations and
  parent-session relay, while preserving copied brainstorms and historical
  changelog entries as source evidence.
- Port the immediate support-rule closure (`showcase-northstar`,
  `syntax-is-the-point`, `dalio-corpus-discipline`,
  `app-discover-phase6-output-shape`, `llm-amplifier-design-posture`) plus the
  app-implement manifest consumer contract referenced by those rules.
- Validate translated `.codex/rules/...` references resolve instead of only
  checking for stale `.claude/...` strings.

Evidence:

- Migration run: 2026-06-27 `socratic-innovation-partner-crew` Codex skill
  package port and referenced support-rule cascade.
- Validation target: frontmatter check, active stale-token scan over entrypoint
  / references / assets / copied support rules, `.codex/rules/*.md` resolver,
  whitespace/line-ending scan, and `git diff --check`.

## 2026-06-27 (Rules and hook parity sweep)

Observed failure mode:

- Skill package ports can pass while the project policy surfaces they cite
  remain missing or stale. The audit found `.claude/rules` much larger than
  `.codex/rules`, and several Claude hook basenames with no Codex Python port.
- Claude shell hooks cannot be copied directly into `.codex/hooks/`; the local
  hook policy says Codex keeps Python equivalents only.

Correction:

- Mirror the full `.claude/rules/*.md` set into `.codex/rules/*.md` and
  normalize active `.claude/skills`, `.claude/rules`, and `.claude/hooks` path
  references to `.codex/...`.
- Add Python ports for the missing Claude hook basenames and keep them unwired
  unless/until `.codex/hooks.json` deliberately enables those event surfaces.
- Update `HOOKS_MIGRATION.md` with the new unwired ports and their conservative
  behavior where it differs from the old shell hook.

Evidence:

- Migration run: 2026-06-27 rules mirror plus Codex hook parity batch.
- Validation target: rule directory comparison, stale `.claude/...` path scan,
  `.codex/rules/*.md` reference resolver, hook basename resolver, `py_compile`
  over `.codex/hooks/*.py`, whitespace/line-ending scan, and `git diff --check`.

## 2026-06-27 (Post-port active-reference audit)

Observed failure mode:

- A skill can pass directory parity while active reference files still carry
  Claude-only invocation syntax (`/research`, `/experiment-methodology`) or a
  stale companion-agent path.
- Broad stale-token scans can over-report historical evidence files and ordinary
  path fragments like `specs/research`; final audits need scoped active-surface
  checks plus explicit allowlisting for unsupported-runtime notes.

Correction:

- Normalize active Socratic reference-library routes to Codex `$skill` syntax,
  `.codex/rules/...` paths, and `.codex/agents/AGENTS.md` guidance.
- Normalize the RSI ideator role body to Codex injection wording and
  `$research-methodology` example routing, while retaining the explicit
  `TeamCreate` / `SendMessage` unsupported-runtime warning in the RSI entrypoint.

Evidence:

- Migration run: 2026-06-27 post-port audit after `rsi-mission-crew` and
  `socratic-innovation-partner(-crew)` ports.
- Validation target: scoped active stale-instruction scan, intentional
  unsupported-runtime note scan, TOML parse, hook `py_compile`, frontmatter
  check, parity checks for agents/skills/hooks/rules, and `git diff --check`.

## 2026-06-27 (Claude command surface port)

Observed failure mode:

- `.claude/commands/*.md` are active user-facing workflows, but Codex has no
  repo-local `.codex/commands/` equivalent. Leaving them only in the Claude tree
  makes command behavior invisible to Codex even when matching skills exist in
  older or blocked roots.
- Claude command bodies may depend on `run_in_background`, `SendMessage`, or
  old `.claude/skills/...` paths that are not faithful Codex-native mechanics.

Correction:

- Port the five root commands (`status`, `topics`, `socratic-dialogue`,
  `implement-spec`, `disk-hygiene`) into repo-local
  `.codex/skills/source-command-*` packages.
- Translate slash-command activation to skill invocation, map disk hygiene to
  `.codex/hooks/disk-hygiene.py`, map implementation to `$spec-to-code`, and
  state the unsupported persistent background-message loop explicitly for the
  paused Socratic dialogue workflow.

Evidence:

- Migration run: 2026-06-27 root `.claude/commands` command-port batch.
- Validation target: command-name parity against `.codex/skills/source-command-*`,
  frontmatter check, active stale-instruction scan with unsupported-runtime notes
  treated explicitly, and `git diff --check`.
