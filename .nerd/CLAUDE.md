# .nerd Directory Structure

This directory contains codeNERD's runtime state, agent configurations, and knowledge databases.

## Prompt Atom Sync Architecture

Agent prompts are stored in YAML files and synced to SQLite knowledge databases for JIT prompt compilation.

### Directory Mapping

```
.nerd/agents/{name}/prompts.yaml  -->  .nerd/shards/{name}_knowledge.db
```

| Source | Target | Sync Trigger |
|--------|--------|--------------|
| `agents/bubbleteaexpert/prompts.yaml` | `shards/bubbleteaexpert_knowledge.db` | TUI boot, `/init` |
| `agents/cobraexpert/prompts.yaml` | `shards/cobraexpert_knowledge.db` | TUI boot, `/init` |
| `agents/goexpert/prompts.yaml` | `shards/goexpert_knowledge.db` | TUI boot, `/init` |

### Sync Triggers

1. **TUI Boot** - `prompt.ReloadAllPrompts()` called after JIT compiler init
2. **`/init`** - Phase 12 syncs all agent prompts after agent registry creation
3. **`/init --force`** - Same as `/init`, upsert overwrites existing atoms

### Upsert Semantics

The sync uses SQL `ON CONFLICT(atom_id) DO UPDATE SET`:

- **Same `atom_id`** - Content is **overwritten** with YAML values
- **New `atom_id`** - **Inserted** into database
- **Atoms in DB but not in YAML** - **Left alone** (not deleted)

### Editing Prompts

1. Edit `.nerd/agents/{name}/prompts.yaml`
2. Restart TUI or run `/init --force`
3. Changes are synced to `{name}_knowledge.db`
4. JIT compiler queries DB for fresh atoms

### YAML Format

```yaml
- id: "identity/agentname/mission"
  category: "identity"
  priority: 100
  is_mandatory: true
  content: |
    Your prompt content here...

- id: "methodology/agentname/patterns"
  category: "methodology"
  priority: 80
  depends_on: ["identity/agentname/mission"]
  content: |
    Additional prompt atoms...
```

### Key Fields

| Field | Purpose |
|-------|---------|
| `id` | Unique atom identifier (used for upsert matching) |
| `category` | Grouping: identity, methodology, domain_knowledge |
| `priority` | Selection priority (higher = more likely included) |
| `is_mandatory` | Always include in compiled prompt |
| `shard_types` | Which shards can use this atom |
| `depends_on` | Other atoms that must be included first |

## Directory Contents

```
.nerd/
├── agents/                    # Type B/U agent configurations
│   ├── {name}/
│   │   └── prompts.yaml       # Editable prompt atoms (SOURCE)
│   └── ...
├── shards/                    # Knowledge databases
│   ├── {name}_knowledge.db    # SQLite with prompt_atoms table (TARGET)
│   └── ...
├── config.json                # Global configuration
├── agents.json                # Agent registry
├── knowledge.db               # Project-level knowledge
└── session.json               # Session state
```

## Ephemeral Agents

Type A (ephemeral) agents do NOT use this architecture. Their prompts come from:
- Embedded corpus in the binary
- Build-time compiled prompt databases
- Static system prompts in shard code

---

**Remember: Push to GitHub regularly!**
