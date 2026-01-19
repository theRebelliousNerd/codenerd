---
trigger: always_on
description: Everything in its place
---

# Modularity and Structure

> **Everything has a place. Put everything in its place.**

## File Size Limits

| Type | Max Lines |
|------|-----------|
| Source code | 1500 |
| Test files | 2000 |
| Config | 500 |
| Docs | 1000 |

## codeNERD Project Structure

```
codeNERD/
├── cmd/                    # Entry points
├── internal/               # Private packages
│   ├── agents/permanent/   # Always-running agents  
│   ├── api/                # REST/gRPC handlers
│   ├── deductive/          # Mangle/Datalog engine
│   ├── storage/            # Database implementations
│   ├── vector/             # Vector operations
│   └── wormhole/           # Attention pipeline
├── pkg/                    # Public packages
├── ai_engine/              # Python AI components
├── docs/                   # Documentation
└── .agent/                 # Agent configuration
    ├── rules/              # Behavior rules
    ├── skills/             # Capabilities
    └── workflows/          # Automated workflows
```

## Placement Rules

| File Type | Location |
|-----------|----------|
| New handler | `internal/api/rest/` or `internal/api/grpc/` |
| New agent | `internal/agents/permanent/<name>/` |
| Shared types | `internal/core/` or appropriate `types.go` |
| Unit tests | Next to source as `*_test.go` |
| E2E tests | `tests/` folder |
| Analysis/audits | `docs/analysis/` |

> A dev should find any file based solely on what it does.
