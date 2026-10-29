# Custom Agent Option Matrix

Required fields:

| Field | Purpose |
|---|---|
| `name` | Stable custom-agent identifier |
| `description` | Human-facing routing guidance |
| `developer_instructions` | Narrow behavioral contract |

Common options:

| Field | Guidance |
|---|---|
| `model` | `gpt-5.6` for demanding roles; `gpt-5.6-terra` for read-heavy support |
| `model_reasoning_effort` | medium, high, or xhigh according to judgment depth |
| `sandbox_mode` | read-only for auditors; workspace-write for artifact/code owners |
| `nickname_candidates` | optional display aliases |
| `skills.config` | attach the owning governed skill and narrow specialist skills |
| `mcp_servers` | attach only a required existing server |

codeNERD conventions:

- include `name` and `description` even when the config registry also supplies them
- keep recursive delegation disabled in specialist instructions
- preserve memory under `.claude/agent-memory/`
- register governed roles in `.codex/config.toml`
- verify every attached skill path
