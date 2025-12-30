# .nerd Directory

This directory contains codeNERD workspace state, configuration, and runtime data.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Directory Structure

```
.nerd/
├── config.json           # Configuration file
├── sessions/             # Session history (quiescent boot)
├── prompts/
│   └── corpus.db         # Project-scoped prompt corpus
├── agents/               # Agent-scoped prompts and knowledge
│   └── <agent>/
│       └── prompts.yaml
├── campaigns/            # Campaign state and artifacts
│   └── <campaign>/
│       └── assault/      # Adversarial assault results
├── logs/                 # Structured logs (22 categories)
├── mangle/               # Hot-loadable Mangle rules
│   ├── extensions.mg     # Custom predicates
│   ├── learned.mg        # Learned patterns
│   └── policy_overrides.mg
├── tools/                # Generated tools (Ouroboros)
└── shards/               # Legacy shard knowledge DBs
```

## Sessions (Quiescent Boot)

Session management enables clean boot with fresh sessions:

- `.nerd/sessions/*.json` - Session history and state
- Use `/sessions` command to list/load previous sessions
- Use `/new-session` to start fresh

## Campaigns

Campaign state and run artifacts:

- `.nerd/campaigns/*.json` - Campaign plans/progress snapshots
- `.nerd/campaigns/<campaign>/assault/` - Adversarial assault artifacts

## Prompts (JIT)

JIT prompt atoms are stored in:

- `.nerd/prompts/corpus.db` - Shared/project-scoped prompt corpus (seeded from embedded defaults)
- `.nerd/agents/<agent>/prompts.yaml` - Agent-scoped prompt atoms

## Files

### review-rules.json

Custom review rules for the ReviewerShard. Define your own code review patterns here.

**Format:**
```json
{
  "version": "1.0",
  "rules": [
    {
      "id": "CUSTOM001",
      "category": "security",
      "severity": "critical",
      "pattern": "regex pattern",
      "message": "Description",
      "suggestion": "How to fix",
      "languages": ["go", "python"],
      "enabled": true
    }
  ]
}
```

**Documentation:** See `CUSTOM_RULES.md` for full documentation

**Example:** See `review-rules.example.json` for working examples

## Quick Start

1. Copy the example file:
   ```bash
   cp review-rules.example.json review-rules.json
   ```

2. Edit `review-rules.json` to add your custom rules

3. ReviewerShard will automatically load them on initialization

## Rule Categories

- `security` - Security vulnerabilities
- `style` - Code style and formatting
- `bug` - Potential bugs and errors
- `performance` - Performance issues
- `maintainability` - Code maintainability

## Rule Severities

- `critical` - Blocks commit, must be fixed
- `error` - Should be fixed before merge
- `warning` - Should be addressed
- `info` - Informational, FYI

## Testing Rules

Use regex101.com to test your patterns before adding them.

## Version Control

Commit `review-rules.json` to share rules across your team.

---

**Last Updated:** December 2024
**Architecture Version:** 2.0.0 (JIT-Driven)
