# Discovery vs Registration Policy

Use this reference to decide whether a migration should update only filesystem
surfaces, only config registration, or both.

## The Three Models

### 1. Filesystem Discovery

The tool discovers skill or agent files directly from known directories and does not
need extra registry entries for normal use.

Typical surfaces:

- `.codex/agents/*.toml`
- `.agents/skills/*/SKILL.md`

Use this model when:

- the repo has no explicit registry layer
- local behavior clearly depends only on directory presence

### 2. Explicit Registration

The repo treats a config file as the authoritative exposure surface.

Typical surface:

- `.codex/config.toml` with `[agents."<name>"]` blocks

Use this model when:

- the repo already registers custom agents centrally
- descriptions or visibility are curated in config

### 3. Hybrid

The repo uses filesystem presence plus explicit registration.

Use this model only for the subset of codeNERD roles governed by its curated
registry.

Implication:

- The `.toml` file is the executable agent definition.
- `.codex/config.toml` is the curated registry and description surface.
- A correct migration updates both.

## Repo Finding for `C:\\CodeProjects\\codeNERD`

As of 2026-07-09, this repository is **mixed**:

- `.codex/agents/*.toml` files exist
- `.codex/config.toml` curates many `[agents.<name>]` roles through `config_file`
- standalone agent files are also discoverable without registration
- registry keys and parsed TOML names use snake_case while filenames and
  `config_file` paths commonly use kebab-case

Migration rule for this repo:

- When a role joins or changes the governed codeNERD fleet, update its TOML,
  registry block, and architecture guidance together.
- When a standalone role is intentional, validate filesystem discovery and do
  not fabricate a duplicate registry block.
- Resolve each registry block's `config_file` and compare the parsed TOML `name`
  to the registry key.
- Report unrelated pre-existing mismatches as ambient drift.

## Skill Implication

Current Codex discovers `.agents/skills/`, while codeNERD's governed packages and
agent attachments still actively use `.codex/skills/`. Duplicate names are not
merged and may both appear in runtime selectors.

Migration rule:

- Select a skill root from current runtime and attachment evidence, then port the
  complete package support closure.
- Port custom agents by filesystem presence plus config registration.

## Validation

To check governed registrations without assuming filename/name identity:

```powershell
python -c "from pathlib import Path; import tomllib; cfg=tomllib.loads(Path('.codex/config.toml').read_text(encoding='utf-8')); bad=[]; [(lambda p,d: bad.append((key,str(p),d.get('name'))) if (not p.is_file() or d.get('name') != key) else None)(Path('.codex')/item['config_file'], tomllib.loads((Path('.codex')/item['config_file']).read_text(encoding='utf-8'))) for key,item in cfg.get('agents',{}).items() if isinstance(item,dict) and 'config_file' in item]; print(bad)"
```

An empty list proves the configured subset resolves consistently. It does not
prove that every standalone TOML must be registered.

