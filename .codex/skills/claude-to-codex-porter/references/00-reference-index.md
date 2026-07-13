# Reference Index

Read this file first, then load only the reference files needed for the current
migration.

## repo-migration-pattern.md
Repo-local source and target mapping rules for this repository. Covers:

- `.claude` source surfaces
- mixed `.codex` and `.agents` target discovery in codeNERD
- when to port runtime artifact directories
- path-rewrite rules
- existing local examples and prompts

## source-parity-ledger.md

Records the initial source snapshot, every codeNERD addition, and the two
late-arriving NeuroLog fleet-maintenance scripts intentionally not copied as
runnable artifacts.

## migration-quality-rubric.md

Six-dimension scorecard for completeness, fidelity, activation, safety,
validation, and evidence honesty. Passing requires no score below 3/5 and an
average of at least 4/5.

## codex-surface-decision-tree.md
Official-surface decision tree for rehoming Claude workspace systems. Covers:

- `AGENTS.md` vs hooks vs `.rules` vs skills vs agents
- classification algorithm and surface ledger
- hard no mappings that prevent fake ports
- validation required for each target surface

## repo-codex-config-patterns.md
Repo-local `.codex/config.toml` surface map. Covers:

- top-level model and context settings
- `[features]` and `[agents]` runtime controls
- MCP server declarations
- explicit `[agents."<name>"]` registration blocks
- validation for config edits

## discovery-vs-registration-policy.md
Decision rule for whether the repo is filesystem-only, registration-first, or
hybrid. Covers:

- the three discovery models
- the repo finding for this workspace
- why agents in this repo should be migrated with config registration

## migration-surface-checklist.md
Anti-underdoing migration checklist. Covers:

- skill package surfaces
- runtime artifact surfaces
- agent TOML surfaces
- config surfaces
- journal surfaces
- evidence-preservation surfaces
- validation and return surfaces

## references/phases/
Step-level migration pipeline inspired by the durable phase-machine pattern:

- `01_scope/phase.md` -- surface ledger before edits
- `02_inventory/phase.md` -- source/target comparison and evidence boundaries
- `03_transform/phase.md` -- target-specific transformation rules
- `04_validate/phase.md` -- target-specific validation gates
- `05_report/phase.md` -- compact report and learning update

## custom-agent-option-matrix.md
Decision matrix for Codex custom-agent sophistication. Covers:

- required TOML fields
- optional fields such as `sandbox_mode`, `mcp_servers`, and `skills.config`
- sandbox selection guidance
- repo-local conventions like source-provenance comments and journal protocols

## repo-agent-exemplars.md
Concrete local TOML examples by archetype. Covers:

- read-only explorers
- journaled writers with skill attachments
- deep implementation workers
- source-provenance ports
- config registration patterns

## workspace-system-surface-map.md
Root-level Claude-to-Codex workspace mapping rules. Covers:

- `.claude/settings*.json` to `.codex/config.toml` or plugin surfaces
- `.claude/mcp.json` to Codex MCP declarations
- `.claude/hooks/` to Python Codex hooks or plugin hook bundles
- `.claude/commands/`, `.claude/rules/`, and `.claude/prompts/` rehoming rules,
  including command-to-skill routing and deprecated prompt exceptions
- `.claude/plugins/` to repo-local plugin bundles and marketplace registration
- unsupported-gap classification when no faithful Codex destination exists

## agent-conversion-checklist.md
Detailed `.claude/agents/*.md -> .codex/agents/*.toml` checklist. Covers:

- dependency discovery
- TOML skeleton
- provenance comments
- model-selection guidance
- shared memory root handling
- config registration
- validation commands

## openai-agent-skills.md
Heading-aligned notes from the official OpenAI Codex skills docs. Use for:

- required skill package shape
- metadata expectations
- official optional `agents/openai.yaml`
- official repo/user/admin/system skill locations

## openai-subagents.md
Heading-aligned notes from the official OpenAI Codex subagents docs. Use for:

- custom agent TOML requirements
- optional fields
- sandbox inheritance
- custom-agent naming and scope

## scripts/inventory_workspace.py

Deterministic pre-edit inventory for Claude/Codex root surfaces, parsed skill
configuration, duplicate skill names, and divergent root copies. Run its unit
tests before relying on it in a migration.

## evals/evals.json

Machine-readable positive, boundary, safety, collision, and negative-routing
cases for the porter activation and output contract.

## journal.md
Preserved legacy working history. Add new run notes as dated files under
`references/journal/`.

