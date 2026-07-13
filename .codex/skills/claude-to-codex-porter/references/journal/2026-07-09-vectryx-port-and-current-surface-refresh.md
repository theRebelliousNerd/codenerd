# Vectryx Port and Current-Surface Refresh

## Context

The source porter lived at
`C:\CodeProjects\neurolog\.agents\skills\claude-to-codex-porter`. The user asked
to add it to Vectryx and authorized focused improvements after the port.

## Evidence

- The source package contained 23 Markdown files and no scripts, assets, evals,
  metadata, runtime directories, or companion agents.
- Two untracked NeuroLog-only fleet helpers appeared after that snapshot. They
  hardcode model rewrites, role deletions, git baselines, and source/target roots;
  `references/source-parity-ledger.md` records why they were not copied.
- Vectryx's runtime exposes both `.codex/skills` and `.agents/skills`, but its
  governed config and agent attachments still enable `.codex/skills` packages
  and disable the mirrored `.agents/skills` entries.
- `codex debug prompt-input` showed that duplicate skill names can remain visible
  from both roots, so a blind dual copy would worsen selector ambiguity.
- The current Codex manual helper failed because the response omitted the
  required `x-content-sha256` header. Official OpenAI Developer Docs were used as
  the fallback authority.

## Decisions

1. Install this governed Vectryx package under `.codex/skills` and register its
   package and `SKILL.md` paths.
2. Teach future porter runs to inspect runtime roots and explicit attachments
   instead of treating either `.agents/skills` or `.codex/skills` as universally
   correct.
3. Preserve all source changelog and flat-journal entries verbatim as historical
   evidence while moving new lessons to dated journal files.
4. Preserve explicit Codex pins unless modernization is requested. For Claude
   labels, start with `opus` -> `gpt-5.6-sol`, `sonnet` -> `gpt-5.6-terra`, and
   `haiku` -> `gpt-5.6-luna`, then document any role-based override.
5. Support JSON, inline TOML, and plugin hook ownership; preserve helper language,
   require trust/activation tests, and never claim `PreToolUse` is complete
   enforcement.
6. Require plugin marketplace/install reachability for repo-distributed plugins.
7. Add deterministic inventory and executable eval surfaces so root and support
   closure are measured before edits.

## Validation Contract

- package validation and trigger-collision scan
- inventory helper unit tests and real-repo smoke run
- JSON/TOML parsing
- source-vs-destination support inventory
- `codex debug prompt-input` visibility check
- stale active-reference scan excluding preserved history
- `git diff --check`
