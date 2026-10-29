# Claude Agent to Codex Checklist

## Inventory

- [ ] Read the full Claude agent frontmatter and body.
- [ ] Identify behavior, write ownership, memory, hooks, skills, model label, and
      forbidden tools.
- [ ] Separate identity from reusable workflow.
- [ ] Find an existing target collision.

## TOML conversion

- [ ] Add `name`, `description`, and `developer_instructions`.
- [ ] Select a documented Codex model and reasoning effort.
- [ ] Select read-only or workspace-write deliberately.
- [ ] Replace Claude tool syntax with Codex workflow language.
- [ ] State whether the agent may delegate.
- [ ] Attach only required skills with valid paths.
- [ ] Preserve source provenance when useful.
- [ ] Keep shared memory at `.claude/agent-memory/`.

## Registration

- [ ] Add or update `[agents.<name>]` in `.codex/config.toml`.
- [ ] Point `config_file` at the project-relative TOML.
- [ ] Avoid duplicate keys and built-in-name collisions.

## Validation

- [ ] Parse every TOML with `tomllib`.
- [ ] Check required fields.
- [ ] Check config-file and skill paths.
- [ ] Scan for Claude model labels, Claude-only tool names, stale source paths,
      and fabricated journal roots.
- [ ] Probe discovery from the intended launch directory.
