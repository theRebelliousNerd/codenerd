# Global Memory Continuity

## Context

The user requested one-way synchronization from Claude's global agent memories
and project memory corpora into Codex's user-level local memory system. The
source was `~/.claude/agent-memory/` plus `~/.claude/projects/*/memory/`; the
target was the native Codex memory intake under `~/.codex/memories/`.

## Evidence

- Claude exposed 425 detailed memory records: 423 across 12 project roots and
  two agent-scoped records.
- Codex documents its main memory files as generated state and its ad-hoc notes
  extension as the authoritative memory add/edit intake.
- Codex supports user-level `SessionStart` hooks, but changed command hooks need
  a one-time trust review.

## Decisions

- Classified the memory records as `REHOME` into Codex's native ad-hoc intake.
- Classified the automatic pull as `DIRECT_TRANSLATION` to a user-level
  `SessionStart` command hook.
- Preserved project and agent boundaries through native-looking `scope: cwd=...`
  and `scope: agent_type=...` fields.
- Used a private hash manifest for idempotency and stripped source-specific
  frontmatter and migration labels from destination memory text.

## Intentional Non-Parity

- Structural `MEMORY.md` indexes are omitted when detailed sibling records
  exist, avoiding duplicate facts and broken relative links.
- Deleted source records are retained and reported because Codex's authoritative
  ad-hoc note intake is append-only.
- Claude transcripts, settings, credentials, and `CLAUDE.md` are outside the
  memory-sync boundary.

## Validation

- Python compile and three isolated unit tests passed.
- Real dry run found 425 records; initial sync created 425; the second sync
  reported 425 unchanged.
- Destination audit found zero Claude-import labels and zero unstripped source
  frontmatter blocks.

## Reusable Lessons

User-level memory migration should target Codex's user-level memory intake and
hook layer. Repository agent-memory trees remain a separate subagent concern.
