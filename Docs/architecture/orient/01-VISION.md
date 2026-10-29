---
doc-class: north-star
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 01 — Vision — The Autonomous Orientation Engine

## The North Star of Orientation

When an engineer runs `nerd init` in any repository — whether an established polyglot enterprise monolith, an emerging prototype, or an open-source codebase — codeNERD must fully configure itself by **deriving reality from first principles**, rather than demanding manual configuration or guessing through fragile heuristics.

The executive model of codeNERD establishes that the model is the creative center, but logic is the executive. Truth is computed by deterministic deductive reasoning over grounded evidence. Orientation is the foundational transduction phase of this contract: it ingests the raw, messy artifacts of a repository's history, file trees, documentation, and preexisting agent configurations, and derives an accurate, durable world model.

### Finished Behavioral Profile

In the finished target state, running `nerd init` executes an autonomous, comprehensive orientation sequence:

1. **Autonomous Development Chronology**:
   Orientation streams git history across the entire commit graph in a single linear pass. It constructs an authentic timeline of development eras — identifying bursts of intense development, sustained waves, and transitional lulls. It grounds chronological reasoning in immutable git committer timestamps rather than fragile filesystem modification times that reset on checkout or clone.

2. **Document Evolution and Lineage**:
   Orientation discerns the genealogy of the project's documentation. It distinguishes foundational origin documents from intermediate revisions, distinguishes superseded specifications from active live contracts, and traces how core designs mutated over time. It recognizes that recency does not equate to quality: a foundational architectural thesis committed in a single burst eighteen months ago remains the governing truth if no successor has superseded it, while a routine changelog updated yesterday is merely operational ephemera.

3. **Detection of Living Vision**:
   Orientation locates where the project's vision, philosophy, and strategic architecture are being written, even when that vision is incomplete, evolving, or explicitly labelled as a work in progress. It does not demand that a file be named `VISION.md` or `CLAUDE.md`. It evaluates role claims, thematic clustering, and structural centrality to identify the genuine creative intent of the human authors.

4. **Non-Interactive North Star Derivation**:
   Orientation extracts and synthesizes the project's mission, problem space, vision statement, user personas, core capabilities, architectural risks, and requirements directly into codeNERD's formal North Star store. It completes this synthesis without forcing the operator through multi-step interactive chat wizards, while respecting existing vision stores and never overwriting prior commitments. It produces a clear, human-readable summary in `.nerd/orientation/README.md` and persists ground facts in `.nerd/orientation/orientation.mg` for instant injection into runtime reasoning.

5. **Exhaustive Multi-Agent Ecosystem Ingest**:
   Orientation inventories the collective intelligence left behind by other developer tools and coding-agent ecosystems. It parses skills, rules, subagent definitions, memories, journals, and instructions across Claude Code, OpenAI Codex, Google Antigravity / Gemini, Grok, Roo Code, Jules, Cursor, and shared `.agents/` directories. It identifies semantic duplicates across tools, uses evidence-backed logic to select the authoritative version, and dynamically materializes domain specialist shard agents and SQLite knowledge bases. Imperative hardcoded agent switches are entirely replaced by declarative deduction.

6. **Discernment of Nebulous Trees**:
   A repository's tracked files are rarely pure application source code. Orientation analyzes file counts, byte densities, commit cadences, and structural attributes to discern what each tree actually represents: production source, test suites, integration fixtures, seed data, validation answer keys, generated stubs, vendored third-party clients, archives, or disposable experiments. It assigns appropriate operational treatments so that massive seed data or answer keys are indexed by name but never erroneously ingested as code context or target edit surfaces.

7. **Recognition of Private Systems**:
   Orientation identifies proprietary, internal, or enterprise dependencies that cannot be queried on the public web. When codeNERD encounters proprietary frameworks, internal database engines, or enterprise services, it recognizes their private status from manifest inspection and local agent skills. It strictly suppresses futile public web research that would leak internal names or generate hallucinations, and instead routes knowledge synthesis exclusively through the repository's own embedded skills, documentation, and client source code.

8. **Consented MCP Integration**:
   Orientation identifies external Model Context Protocol (MCP) server definitions declared in local or user-scoped agent configurations. It recognizes servers that match the project's active dependencies and skills. Rather than silently copying credentials or ignoring external tooling, it stages import candidates, requests explicit operator approval, and executes byte-preserving, non-destructive configuration merges.

9. **Targeted First-Boot Clarification**:
   Orientation does not paralyze `nerd init` by asking dozens of interrogative questions in the terminal. Instead, it formulates best guesses for any residual ambiguities (conflicting tree roles, uncertain private dependencies, proposed MCP imports). On the first interactive TUI launch, codeNERD presents these concise, high-value questions with pre-selected defaults via the kernel clarification path. The operator's decisions are permanently persisted, immediately overriding initial hypotheses across all Mangle policies.

---

## Why Orientation Matters to the codeNERD Vision

Every capability of the orientation engine directly realizes the core architectural tenets articulated in `CLAUDE.md` ("The Vision (Steve, 2026-09-18)"):

### 1. "It knows the codebase better than any other coding agent because it never has to grep around"
Standard coding agents enter a new repository blind. They issue ad-hoc directory listings, execute scattered grep searches, read arbitrary README files, and hallucinate structural assumptions. codeNERD eliminates grep-driven orientation. The Mangle kernel computes a complete, relational fact graph over history, documents, agent configurations, and directory topologies before the first user prompt is processed. When an agent shard is activated, the kernel immediately delivers the precise blend of domain knowledge, architectural lineage, and structural boundaries required for the task.

### 2. "The harness decides, not the model's discretion"
In standard agents, the LLM decides which files to inspect, what constitutes an agent skill, and whether a document is relevant. When left to model discretion, context windows fill with irrelevant seed data, stale documentation overrides live design, and duplicate skills fight for attention. In codeNERD, the harness decides. Deductive Mangle rules compute which documents are read candidates, which skills supersede their competitors, and which trees must be excluded from code search. The model's creative power is applied exclusively to comprehension and synthesis within boundaries forced by logic.

### 3. "Domain knowledge is pushed into the context window when the harness decides it is needed"
Orientation does not dump an entire documentation tree or twenty foreign skill files into an unbounded LLM context window. Instead, orientation digests foreign skills, documentation lineage, and project vision into structured SQLite knowledge bases and discrete prompt atoms. The JIT prompt compiler and working-set activation engine then project these atoms into the model's window at the exact turn, task, and lifecycle phase where they are required.

### 4. "Clean fixpoint, not clean loop"
Orientation rejects brittle imperative scripts, cascading heuristics, and complex procedural loops for executive decisions. Go acts as the sensory organ: it streams git logs, parses file trees, extracts AST elements, and reads document bytes. The executive decisions — what an era is, which document evolved into which, which agent must be spawned, which tree is seed data, and what question must be asked — are the fixpoint of deductive Mangle rules evaluated over asserted facts.

### 5. "The north star is a final state, not a goal to be hit; the harness continuously derives distance"
An agent cannot align its work with a project's north star if that north star does not exist or must be manually typed into a form. By autonomously deriving a comprehensive, structured North Star from the repository's living documentation, orientation provides the immutable measuring stick against which the Guardian and background observers derive the distance between current code evidence and ultimate architectural intent.
