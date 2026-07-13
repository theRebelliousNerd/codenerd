# OpenAI Codex Skills Notes

Source: https://developers.openai.com/codex/skills
Accessed: 2026-07-09

This file is a heading-aligned summary of the official Codex skills docs, adapted for
local reference use in this repository.

## Agent Skills

- A Codex skill is a directory with a required `SKILL.md` file plus optional
  `scripts/`, `references/`, `assets/`, and `agents/openai.yaml`.
- Codex uses progressive disclosure: it sees skill metadata first and loads the full
  `SKILL.md` only when the skill is chosen.
- `SKILL.md` must include at least `name` and `description`.
- Codex detects skill changes automatically, though a restart may be needed if
  the update does not appear.
- Skills are the current shared surface for reusable workflows. Custom prompts
  are deprecated and user-local, so shared slash-command imports should become
  skills or skill references.

## How Codex Uses Skills

- Skills can activate explicitly when the user names them.
- Skills can activate implicitly when the request matches the `description`.
- Clear scope and negative boundaries in the description improve activation quality.

## Create a Skill

- The docs recommend using the built-in `$skill-creator` first.
- Manual skills are also supported by creating a folder with a `SKILL.md` file.
- The minimal frontmatter shown in the docs contains `name` and `description`.

## Where to Save Skills

Official Codex docs describe these locations:

- repo-scoped skills under `.agents/skills`
- user-scoped skills under `$HOME/.agents/skills`
- admin-scoped skills under `/etc/codex/skills`
- built-in system skills shipped with Codex

Important repo-local note:

- The official docs use `.agents/skills` for repository scope.
- codeNERD's governed packages and explicit attachments still use
  `.codex/skills`; preserve the existing owner and use that root for new
  governed packages until the repository deliberately consolidates.
- Treat `scripts/`, `references/`, `assets/`, and `agents/openai.yaml` as part
  of a skill package when present.
- Duplicate skill names across roots are not merged and may both appear in the
  selector; validate with `codex debug prompt-input`.

## Install Skills

- The docs point to `$skill-installer` for installing additional skills.
- Codex detects newly installed skills automatically, though a restart may be needed
  if a change does not appear.

## Enable or Disable Skills

- Docs show `[[skills.config]]` entries in `config.toml` for per-skill
  enablement overrides.
- Official examples have used both the `SKILL.md` path and skill-directory path
  with an `enabled` flag. Preserve a proven repository representation and verify
  it against the current runtime/schema.

## Optional Metadata

- The docs describe optional `agents/openai.yaml` metadata for appearance, invocation
  policy, and tool dependencies.
- Relevant examples include display name, brand color, default prompt, and
  `allow_implicit_invocation`.
- Tool dependencies can declare MCP requirements.

## Best Practices

The docs emphasize:

- keep each skill focused on one job
- prefer instructions over scripts unless deterministic behavior is needed
- write imperative steps with explicit inputs and outputs
- test prompts against the description to verify trigger behavior

## Plugin Distribution

- Direct skill folders are best for local authoring and repo-scoped workflows.
- Build a plugin when the workflow should be installable, bundled with app
  integrations, bundled with MCP configuration, or shipped with lifecycle hooks.

