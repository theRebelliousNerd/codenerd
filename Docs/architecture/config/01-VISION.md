# 01 — Vision

## Product outcome

**PROPOSED UPLIFT.** An operator edits or generates one versioned workspace
configuration. Before codeNERD opens a network client, store, kernel, MCP server,
or executor, a strict loader returns either one immutable validated snapshot or
one redacted diagnostic. Every consumer receives a projection with the same
snapshot ID.

The visible outcome is predictable boot: the intended provider and model are
used, budgets and execution bounds agree across chat/campaign/shared Cortex, and
invalid input cannot become ambient fallback behavior.

## Target architecture

```text
file / approved secret sources / schema version
                 |
                 v
 bounded decode -> migration -> normalization -> cross-field validation
                 |
       immutable workspace snapshot + redacted provenance receipt
                 |
      +----------+-----------+------------+-------------+
      v          v           v            v             v
  perception  scheduler   limits/JIT   execution/MCP  UX/features
      |                                      |
      +------ creative inputs/bounds --------+
                         |
                         v
             Mangle permitted/3 remains executive
```

Configuration projection is deterministic Go data. It does not ask an LLM to
guess intent, repair hostile input, or grant a capability.

## Required properties

1. Strict, versioned, bounded decode with compatibility migrations.
2. Cross-field validation for enums, URLs, paths, percentages, limits and
   engine/provider combinations.
3. Atomic merge-safe persistence and platform-honest secret protection.
4. One snapshot identity across all constructors and reload/rollback.
5. Exact source precedence with redacted field provenance.
6. Uniform execution projection plus independent Mangle default deny.
7. Raw prompts/responses off by default, redacted, bounded and retained only by
   explicit operator choice.
8. A fixed cross-surface conformance suite and no-effect migration laboratory.

## North-star alignment

- The LLM creative center receives configured model/capability/context inputs.
- The logic executive retains action and permission authority.
- Config strengthens transduction by making its inputs typed and attributable;
  it does not add prompt text or a second executive.
- Uplifts prefer immutable facts/projections and observable lifecycle gates over
  new mutable singletons.

## Non-goals

No automatic field-by-field hot reload, plaintext secret vault, prompt atom
library, Mangle policy authoring, full config dump in logs, capability-to-
permission shortcut, or model-driven validation.

**REJECTED.** A plugin framework that can mutate config during every turn would
destroy snapshot identity and reproducibility. Extension belongs in validated
schema namespaces with explicit lifecycle, not arbitrary callbacks.
