---
doc-class: north-star
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: working-tree-C2a
supersedes: []
---

# 06 — Ecosystem Ingest and Agents — Multi-Agent Configuration & Skill Ingest

This capability specification details the ingestion of foreign coding-agent configurations, deduplication of skills and memories across tools, and the declarative derivation of domain shard agents, owned by Lane `I1`.

---

## 1. The Multi-Agent Ecosystem Problem

Modern repositories frequently accumulate configuration files, prompt instructions, custom modes, and operational skills from multiple AI developer tools: Claude Code, OpenAI Codex, Google Antigravity / Gemini, Grok, Roo Code, Jules, and Cursor.

The pre-orientation baseline had two major failures in this domain. These
historical defects motivate the capability; they are not claims about the
current source integration recorded below:
1. **Total Blindness**: The scanner explicitly skips dot-directories (`internal/world/fs.go:241-263`), ignoring `.codex/`, `.agents/`, `.gemini/`, `.grok/`, `.jules/`, and `.cursor/`.
2. **Imperative Hardcoding**: Specialist agents are selected via an imperative Go `switch` statement over language strings (`internal/init/agents.go:373-626`). If a repository contains a rich library of specialized database, infrastructure, or compliance skills, codeNERD ignores them and spawns only generic language experts.

The Ecosystem Ingest engine parses all preexisting agent assets into uniform ground facts, uses Mangle logic to resolve cross-tool duplicates, and dynamically provisions shard agents and SQLite knowledge bases from repository reality.

---

## 2. Multi-Format Agent Registry

The parser (`internal/orient/ecosystem.go`) walks only registered, well-known agent configuration locations. It is a format registry, not a project-specific search:

| Tool Identifier | Target Paths & Formats | Kind Classified | Ingestion Semantics |
|---|---|---|---|
| `/claude` | `.claude/skills/<name>/SKILL.md`<br>`.claude/agents/<name>.md`<br>`.claude/agent-memory/<agent>/*.md`<br>`.claude/commands/*.md`<br>`~/.claude/projects/<sanitized_cwd>/memory/*.md` | `/skill`<br>`/subagent`<br>`/memory`<br>`/command`<br>`/memory` | Parses YAML frontmatter (name, description, tools). Extracts body without truncation. User-scoped memory read as `Tracked: /no`. |
| `/codex` | `.codex/skills/**/SKILL.md`<br>`.codex/agents/*.toml` | `/skill`<br>`/subagent` | Parses TOML agent configurations (name, description, developer instructions, model, tools). |
| `/agents` | `.agents/skills/**/SKILL.md`<br>`.agents/rules/*.md` | `/skill`<br>`/rule` | Standard shared agent skills and rule definitions. |
| `/gemini`, `/antigravity` | `.gemini/skills/**`<br>`GEMINI.md`<br>`.gemini/settings.json` | `/skill`<br>`/instructions`<br>`/mode` | Ingests skill markdown files and root instructions. |
| `/grok` | `.grok/**` | `/skill`, `/instructions` | Ingests Grok workspace rules and persona definitions. |
| `/roo` | `.roomodes` (JSON or YAML) | `/mode`, `/subagent` | Parses `customModes` array (slug, name, roleDefinition, customInstructions, groups). |
| `/jules` | `.jules/**` journals and plans | `/journal` | Ingests execution histories and architectural journals. |
| `/cursor` | `.cursor/rules/*.mdc` | `/rule` | Parses YAML frontmatter description and glob associations. |
| `/copilot` | `.github/copilot-instructions.md` | `/instructions` | Repository-wide developer instructions. |
| `/nerd` | `CLAUDE.md`, `AGENTS.md` (root and subtrees) | `/instructions` | Scoped developer guidance; subtree files carry `ScopeDir`. |

### Git Tracked vs. Local-Only
Files are examined against the git index (`git ls-files`). Files committed to version control are marked `Tracked: /yes`. Files existing only in the local checkout or user home directory are marked `Tracked: /no`.

---

## 3. Ground EDB Fact Representation

The parser asserts pure sensory facts into the Mangle engine:

- `agent_source(ID, Tool, Kind, Name, Path, Tracked)`:
  - `ID`: unique string hash of the source.
  - `Tool`: atom (`/claude`, `/codex`, `/agents`, `/gemini`, `/grok`, `/roo`, `/jules`, `/cursor`, `/copilot`, `/nerd`).
  - `Kind`: atom (`/skill`, `/subagent`, `/rule`, `/memory`, `/instructions`, `/command`, `/journal`, `/mode`).
  - `Name`: normalized identifier.
  - `Path`: repo-relative path (or absolute path for user-scoped memory).
  - `Tracked`: atom (`/yes` or `/no`).
- `agent_source_digest(ID, Digest)`:
  - SHA-256 content hash of the normalized body text.
- `agent_source_topic(ID, Topic)`:
  - Domain topics extracted from YAML frontmatter tags, descriptions, or headings.
- `agent_source_scope(ID, Dir)`:
  - Subtree scope for path-confined rules or instruction files.

---

## 4. Deductive Deduplication and Agent Derivation

Policy file `internal/orient/ecosystem_agents.mg` computes duplicate resolution and shard agent provisioning.

### Cross-Tool Duplicate Resolution
When multiple tools define overlapping skills (e.g. an identical deployment skill in both `.claude/skills/deploy` and `.codex/skills/deploy`):
- `agent_source_duplicate(A, B)`: derives when two sources share the same normalized `Name` or strongly overlapping `Topic` sets across different `Tool` definitions with differing `Digest` values.
- `agent_source_winner(ID, Why)`: selects the authoritative winning source based on grounded evidence:
  1. Commit recency when both sources have measured history.
  2. Git tracking precedence (`Tracked == /yes` beats local-only `/no`) when recency does not separate the pair.
  3. Body completeness and structural specificity.
  4. Inbound citation density from other specifications.
  *Ruling*: There is no hardcoded tool-precedence table. Evidence alone decides the winner.

### Dynamic Shard Agent Provisioning
- `orient_agent(Name, Why)`: defines the specialist shard agents codeNERD must construct. Derived from:
  1. Winning imported subagent definitions (`/subagent` or `/mode`).
  2. Clusters of winning skills sharing cohesive domain topics.
  3. Declarative language and framework mappings (the data previously buried in the Go switch).
- `orient_agent_knowledge(Name, SourceID)`: maps winning skills, rules, and memories directly into the target shard's knowledge pool.
- `orient_research_topic(Name, Topic)`: derives topics that require external Context7 research because no winning local source provides adequate coverage. If local skills provide complete coverage, external research is entirely suppressed.

---

## 5. Materialization Pipeline in `nerd init`

The orientation phase measures sources with `orient.Discover` alongside the
shared history/document census before profile generation. Agent materialization
(`internal/init/phase_ecosystem.go`) then consumes the retained engine:
1. Reads the measured sources from the retained snapshot.
2. Adds profile measurements to the same engine; it does not discover sources or run another history pass.
3. Evaluates the shared Mangle policy to fixpoint.
4. Queries `orient_agent`, `orient_agent_knowledge`, and `agent_source_winner`.
5. Materializes each derived agent via `createAgentKnowledgeBase`:
   - Seeds `.nerd/shards/{agent}_knowledge.db` with winning knowledge atoms (chunked for embedding, never truncated; each atom records source tool and path).
   - Generates `.nerd/agents/{agent}/prompts.yaml` using the winning prompt definition.
   - Registers the agent in `.nerd/agents.json`.
6. Emits an audit report at `.nerd/orientation/agents.md` documenting every provisioned agent, its knowledge sources, and every superseded duplicate with the rationale for its rejection.
7. Feeds the derived agent roster into interactive curation, allowing the operator to review and toggle agents on a TTY.

---

## 6. Verification Seams & Tests

1. `TestEcosystemParser_AllFormats`: Test table feeding fixture directory trees for each supported tool format, asserting exact parsing of frontmatter, bodies, scopes, and digest values.
2. `TestEcosystemDeduplication_EvidenceOverPrecedence`: Asserts duplicate sources across Claude and Codex with conflicting commit dates; verifies that the git-tracked, more recent file wins regardless of tool type.
3. `TestAgentMaterialization_EndToEnd`: Runs `phase_ecosystem` against a temporary workspace, verifying that `.nerd/shards/<name>_knowledge.db` is populated, `prompts.yaml` is written, and `.nerd/orientation/agents.md` records the lineage.

## C2a source integration (verification pending)

Init retains the measurement engine through profile and agent materialization
(`internal/init/phase_ecosystem.go:115`, `integrateEcosystem`). The embedded
loader includes ecosystem policy (`internal/orient/engine.go:111`, `policySource`).
Its two thresholds have typed defaults, validation and policy rows
(`internal/config/orient.go:182`, `WithDefaults`; `internal/config/orient.go:400`, `Params`).
Imported prompt-source selection is a derived row
(`internal/orient/ecosystem_agents.mg:109`, `orient_agent_prompt`). This is
source state, not passing integration evidence.
