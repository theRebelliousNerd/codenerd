# cmd/tools/validate_prompt_atoms - Prompt Atom YAML Validator

This tool provides strict validation for prompt atom YAML files used by the JIT prompt compiler, catching typos and schema violations.

## Usage

```bash
go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms
```

## File Index

| File | Description |
|------|-------------|
| `main.go` | Strict YAML validator for prompt atom files catching unknown fields (typos like init_phase vs init_phases) and enforcing required fields (id/category/priority/is_mandatory/content). Reports errors and warnings with file paths and atom IDs. |

## Validation Rules

| Rule | Severity |
|------|----------|
| Unknown YAML field | Error |
| Missing required field | Error |
| Invalid category value | Error |
| Empty content | Warning |
| Missing description | Warning |

## Required Fields

- `id` - Unique atom identifier
- `category` - One of: identity, protocol, safety, methodology, language, framework, domain, campaign, context, exemplar
- `priority` - Integer priority (0-100)
- `is_mandatory` - Boolean skeleton/flesh flag
- `content` or `content_file` - Atom content

## Contextual Selectors (Optional)

Validates correct field names for all 11 selector dimensions:
- `operational_modes`, `campaign_phases`, `build_layers`
- `init_phases`, `northstar_phases`, `ouroboros_stages`
- `intent_verbs`, `shard_types`, `languages`, `frameworks`, `world_states`

## Flags

| Flag | Description |
|------|-------------|
| `-root` | Root directory for YAML files |
| `-fail-on-warn` | Exit non-zero on warnings |

## Dependencies

- `internal/prompt` - Atom category constants
- `gopkg.in/yaml.v3` - YAML parsing

## Building

```bash
go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms
```

---

**Remember: Push to GitHub regularly!**
