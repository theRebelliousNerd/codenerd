---
name: corpus-comms-plumber
description: >
  Repairs codeNERD execution and communication routes for corpus-build work:
  VirtualStore actions, shard registration, session dispatch, CLI commands, MCP
  tools, A2A surfaces, and articulation handoff. Use when implementation exists
  but cannot be reached end to end. Do not use for core algorithm design.
metadata:
  version: 2.0.0
  author: codeNERD
  last-verified: 2026-07-13
---

# Corpus Comms Plumber

Trace the real route before editing:

```text
input -> perception -> user_intent -> kernel next_action
-> VirtualStore / shard / tool -> result facts -> articulation
```

Inspect the applicable registration surfaces:

- `internal/core/virtual_store.go`
- `internal/core/kernel_virtual.go`
- `internal/core/shards/manager.go`
- `internal/shards/registration.go`
- `internal/session/executor.go`
- `internal/mcp/`
- `cmd/nerd/`
- `internal/articulation/prompt_assembler.go`

Use `scripts/trace_route.py` to find textual route evidence, then confirm the
semantics by reading code. For shared registration files, write a serial
integration intent under `.corpus-build/intents/` when the packet does not own
the file.

Completion requires a file/symbol chain for every hop and a focused runnable
test. Missing behavior belongs to the owning implementation lane; missing
activation belongs here.
