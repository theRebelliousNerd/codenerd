# init/ - Workspace Initialization Atoms

Guidance for workspace setup and knowledge base initialization.

## Files

| File | Purpose |
|------|---------|
| `agents.yaml` | Agent initialization |
| `analysis.yaml` | Codebase analysis during init |
| `facts.yaml` | Initial fact generation |
| `profile.yaml` | User/project profile setup |
| `phases_encyclopedia.yaml` | Init phase definitions |
| `kb_core_shards.yaml` | Core shard knowledge |
| `kb_shared.yaml` | Shared knowledge atoms |
| `kb_codebase.yaml` | Codebase knowledge extraction |
| `kb_campaign.yaml` | Campaign knowledge |
| `kb_agent.yaml` | Agent knowledge |

## Init Phases

1. **Scan** - Discover project structure
2. **Analyze** - Extract patterns and conventions
3. **Profile** - Generate project profile
4. **Hydrate** - Load knowledge base

## Selection

Init atoms are selected via `init_phase` in CompilationContext.
