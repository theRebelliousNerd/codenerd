# Schema, JIT, And Piggyback

This skill exists to reinforce the existing `codeNERD` architecture, not replace it.

## JIT remains standard

Do not turn the Codex skill into a giant static system prompt.

Keep JIT prompt assembly, prompt atoms, and context selection as the source of task framing.

The skill should define backend behavior and invariants, then let JIT provide task-specific context.

## Piggyback contract

When Piggyback is requested, preserve:

- `surface_response`
- `control_packet`
- any tool-request semantics already encoded by `codeNERD`

## Schema behavior

When the caller provides a schema:

- prefer the schema-capable path
- return only schema-valid JSON
- treat schema rejection as a first-class failure

## Skill invocation behavior

The repo-local skill should be invoked explicitly before the structured prompt body.

If the skill is missing at runtime, warn and fall back to the legacy prompt path so active sessions keep moving, but keep setup/auth probes strict so the missing skill is fixed quickly.
