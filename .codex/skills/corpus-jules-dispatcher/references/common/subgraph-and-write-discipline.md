<!-- SYNCED from corpus-build/references/common/subgraph-and-write-discipline.md sha256:09457adecb42 -- DO NOT EDIT -->

# Fact-Space and Write Discipline

codeNERD's shared state is a logic program plus runtime stores. Protect it with
these invariants.

## Mangle ownership

- Declare every predicate before use.
- Match arity and type bounds exactly.
- Bind variables positively before negation.
- Keep learned rules from granting core-owned permissions.
- Preserve stratification and bounded recursion.
- Add facts through the owning store or kernel path, not an ad-hoc side channel.

## Runtime writes

- Identify the owning subsystem before persisting state.
- Check for an existing logical equivalent when duplicates would change
  derivation or replay behavior.
- Make retries idempotent where a lifecycle can replay.
- Preserve cancellation and cleanup.
- Keep shared registration edits serial.

## LLM-facing behavior

- Add prompt atoms first.
- Wire selection through the JIT compiler.
- Preserve piggyback/control-packet semantics.
- Do not hardcode new behavioral prose in a shard when an atom is the owning
  surface.

## Safety

- New actions must pass the constitutional `permitted(...)` path.
- Hooks and prompts are guardrails, not authorization boundaries.
- Record intentional global state explicitly.

