# capability/ - Tool Capability Atoms

Guidance for using specific tool capabilities.

## Files

| File | Capability | Purpose |
|------|------------|---------|
| `codedom_core.yaml` | CodeDOM | Semantic code manipulation primitives |
| `codedom_impact.yaml` | CodeDOM Impact | Impact analysis for code changes |
| `codedom_selection.yaml` | CodeDOM Selection | When to use CodeDOM vs text editing |
| `codedom_tools.yaml` | CodeDOM Tools | Tool schemas and usage |
| `knowledge_discovery.yaml` | Knowledge Discovery | Semantic knowledge retrieval |
| `tool_thinking.yaml` | Tool Selection | Meta-guidance for choosing tools |

## CodeDOM

CodeDOM provides semantic code editing via AST manipulation:
- `get_elements` - Query code structure
- `edit_lines` - Surgical line edits
- Language-agnostic operations

## Selection

Capability atoms are selected based on available tools and task requirements.
