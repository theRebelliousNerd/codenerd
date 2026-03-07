---
name: spec-doc-sprint
description: Fill in the codeNERD product specification scaffold templates for any subsystem. This skill should be used when the user asks to document a subsystem, fill in spec docs, run the documentation sprint, or complete the product specification for any package in cmd/ or internal/. Enforces template preservation, source-code grounding, natural language only, and per-file structural requirements.
---

# Spec Doc Sprint — Deterministic Workflow

This is a step-by-step workflow for filling in the 18 scaffold template files for any codeNERD subsystem. Follow these steps in exact order. Do not skip steps. Do not improvise.

> [!CAUTION]
> **You are FILLING IN templates, not rewriting them.** The scaffold templates have specific headers, sections, tables, and HTML comment placeholders. Replace ONLY the `<!-- comment -->` placeholders with real content. Preserve every header, table structure, and section. If you overwrite the template structure, you have FAILED.

---

## Phase 1: Research (Before Writing Anything)

### Step 1.1 — Directory Scan

Use `Glob` (e.g., `internal/campaign/**/*.go`) and `Bash` with `ls -la` on the target source directory.

Record:

- Total file count (determines quality tier — see Quality Tiers below)
- File sizes (large files = complexity hot spots)
- Test file count vs source file count
- Non-Go artifacts (.mg, .md, .bak, .json)

### Step 1.2 — Read Key Source Files

Use `Read` on the 5-8 most important files in this priority:

1. Main type definition file (types.go, *_types.go)
2. Constructor / initialization file (init.go, new.go, *_init.go)
3. The largest file
4. Interface definitions (public API surface)
5. Test files (what's tested tells you what matters)

Build a mental model of: core types, public interfaces, data flow, concurrency model, external dependencies.

### Step 1.3 — Investigate Callers and Consumers

Use `Grep` to find who imports this package (reverse dependency graph). Identify the public API surface by reading exported types and functions. Trace how this subsystem is wired into the rest of the system by examining constructor callers and method invocations.

> **DO NOT rely on GEMINI.md, AGENTS.md, CLAUDE.md, SKILL.md, or any other configuration/spec markdown files as sources of truth.** All content in spec docs must come from reading the actual Go source code, test files, and build artifacts. The code IS the spec.

### Step 1.4 — Read the Target Template Files

Open each of the 18 scaffold files in `Docs/Spec/<subsystem>/`. Confirm they exist and contain the template structure. If any are missing, re-run `scaffold.ps1` first.

---

## Quality Tiers

Before writing, classify the subsystem by its source file count (non-test .go files). This determines the minimum content depth per spec document.

| Tier | Source Files | Multiplier | Example Min Lines/Doc | Example |
|------|-------------|------------|----------------------|---------|
| Small | 1-10 files | file_count * 5 | 5 files = 25 lines/doc | internal/types, internal/diff |
| Large | 11+ files | file_count * 10 | 30 files = 300 lines/doc | internal/core, internal/campaign |

Floor: 15 lines per doc minimum regardless of package size.

Detailed specs are expected. Thin, superficial documents that meet the minimum structure but lack substance will be flagged by `quality-scan.ps1`. Every document should be thorough enough that a senior engineer joining the project can understand the subsystem from the spec alone.

---

## The Narrative Arc: Current State → Gaps → North Star

Three documents form the backbone of every spec. They must tell a coherent story:

1. **current-state.md** — The honest reality. What exists TODAY, file by file, warts and all.
2. **gap-analysis.md** — The delta. What's missing, what's partial, what's undocumented. Every gap must map to a north-star goal.
3. **north-star.md** — The destination AND the route. Vision, goals, principles, AND a phased uplift roadmap showing how to close the gaps.

If a reader cannot trace a gap backward to a current-state limitation AND forward to a north-star goal with a roadmap phase, the narrative is broken. Fix it before marking complete.

---

## Phase 2: Write Documents (In This Exact Order)

Each document has a corresponding reference guide in `references/`. **Read the reference guide BEFORE writing each document.** The reference guide tells you exactly what to put in each section.

### Step 2.1 — README.md

📖 **Read first:** `references/readme.md`

Fill in: What Is This (one paragraph), Quick Facts table, Key Concepts (3-5 bullets). Leave the navigation table as-is.

Update `Spec Status` from `🔴 Not Started` to `🟢 Complete`. Update `Last Updated` date.

---

### Step 2.2 — current-state.md

📖 **Read first:** `references/current-state.md`

This is the most important document. Fill in: What's Built and Working, What's Built but Incomplete, What's Stubbed, File Inventory table (EVERY significant file with name, purpose, line count, status), Key Types, Current Behavior (step-by-step trace), Known Limitations, Technical Debt.

Update status header.

---

### Step 2.3 — north-star.md

📖 **Read first:** `references/north-star.md`

This is the VISION document. It must paint a detailed, concrete picture of where this subsystem is heading and HOW to get there. Thin north-stars (under 60 lines for medium+ subsystems) indicate insufficient vision work.

Fill in ALL of:

1. Vision Statement (2-3 sentences — concrete end state, not platitude)
2. 3-5 Strategic Goals (each with ### header, 4-6 sentences each covering: what, why, what success looks like, which current-state limitation it addresses)
3. Guiding Principles (3-5 numbered principles with concrete examples)
4. Non-Goals (at least 3 explicit things this subsystem should NOT try to do)
5. Success Metrics table (every metric must have Current, Target, and How Measured columns)
6. Relationship to Overall Architecture (1-2 paragraphs connecting to codeNERD mission)
7. **Uplift Roadmap** (NEW — 3-4 phased plan showing how to close gaps toward the vision, each phase referencing specific gap-analysis items)
8. **North-Star Alignment Matrix** (NEW — table mapping each goal to its supporting gaps and roadmap phase)

QUALITY CHECK: Every Strategic Goal must address at least one gap from gap-analysis.md or one limitation from current-state.md. Orphaned goals that don't trace to reality are speculative.

Update status header.

---

### Step 2.4 — gap-analysis.md

📖 **Read first:** `references/gap-analysis.md`

This is the BRIDGE between current-state (what IS) and north-star (what SHOULD BE). Every gap must explicitly identify which north-star goal it blocks.

Fill in ALL of:

1. Spec vs Reality Matrix (use Yes/Partial/No indicators)
2. Built But Not Spec'd (undocumented capabilities — things in code with no spec reference)
3. Spec'd But Not Built (features in specs with zero code)
4. Partially Implemented table (What Works / What's Missing columns)
5. **North-Star Alignment Map** (NEW — table mapping each gap to the north-star goal it blocks)
6. Priority Assessment (Critical/Important/Nice-to-Have — each item references its north-star goal)
7. Recommendations (Immediate / Short-term / Strategic, with effort estimates)

COMPLETENESS CHECK: You MUST include ALL seven sections. Missing sections is an automatic FAIL. Cross-reference gaps against north-star goals — every critical gap needs a goal, every goal needs at least one gap.

Update status header.

---

### Step 2.5 — data-flow.md

📖 **Read first:** `references/data-flow.md`

Fill in: Overview (Mermaid diagrams welcome here), Input table, Processing steps (add as many ### Step N headers as needed), Output table, State Management, State Transitions, Concurrency Model, Data Invariants.

Update status header.

---

### Step 2.6 — dependencies.md

📖 **Read first:** `references/dependencies.md`

Fill in: Upstream Dependencies table (from import statements), Downstream Dependents table (from `Grep`), External Dependencies table, Dependency Health, Circular Dependencies, Decoupling Opportunities.

Update status header.

---

### Step 2.7 — wiring.md

📖 **Read first:** `references/wiring.md`

Fill in: Entry Points table, Exit Points table, Protocol Boundaries table, Initialization & Lifecycle, Shutdown & Cleanup, Current Wiring Gaps, Wiring Diagram (prose narrative).

Update status header.

---

### Step 2.8 — api-contract.md

📖 **Read first:** `references/api-contract.md`

Fill in: Each public contract as `### Contract N: Name` with What it does / Preconditions / Postconditions / Invariants / Error conditions. Internal Interfaces. Versioning & Stability table. Breaking Change Policy.

Update status header.

---

### Step 2.9 — test-strategy.md

📖 **Read first:** `references/test-strategy.md`

Fill in: Testing Philosophy, Current Test Coverage table (Unit/Integration/Fuzz/Benchmark/Race rows), Test Gaps (Critical and Important sub-sections), Test Infrastructure, Recommended Test Additions table, Testing Challenges.

Update status header.

---

### Step 2.10 — failure-modes.md

📖 **Read first:** `references/failure-modes.md`

Fill in: Overview, each failure mode as `### Failure Mode N: Name` with ALL fields (Trigger, Symptoms, Impact, Likelihood, Severity, Current Mitigation, Recommended Fix). Inefficiencies table. Single Points of Failure. Cascading Failure Risks. Resilience Recommendations.

Update status header.

---

### Step 2.11 — error-taxonomy.md

📖 **Read first:** `references/error-taxonomy.md`

Fill in: Error Categories (grouped as ### Category N: Name with tables), Error Handling Strategy, Error Propagation Map, Sentinel Errors, Error Reporting, Missing Error Handling.

Update status header.

---

### Step 2.12 — safety-model.md

📖 **Read first:** `references/safety-model.md`

Fill in: Safety Scope, Constitutional Safety Integration, Trust Boundaries table, Permission Model, Dangerous Operations table, Audit Trail, Safety Gaps.

Update status header.

---

### Step 2.13 — performance-profile.md

📖 **Read first:** `references/performance-profile.md`

Fill in: Hot Paths, Latency Characteristics table, Memory Behavior, Concurrency Profile, Scaling Characteristics table, Benchmark Results, Optimization Opportunities table.

Update status header.

---

### Step 2.14 — observability.md

📖 **Read first:** `references/observability.md`

Fill in: Debugging Playbook (### Step N headers), Logging table, Metrics table, Tracing, Common Failure Signatures table, Observability Gaps.

Update status header.

---

### Step 2.15 — configuration.md

📖 **Read first:** `references/configuration.md`

Fill in: Configuration Options table (Option/Type/Default/Valid Range/Effect), Environment Variables table, Configuration Sources, Configuration Validation, Configuration Interdependencies, Hardcoded Values That Should Be Configurable.

Update status header.

---

### Step 2.16 — design-decisions.md

📖 **Read first:** `references/design-decisions.md`

Fill in: ADRs as `### ADR-NNN: Title` with ALL fields (Date, Status, Context, Decision, Alternatives Considered, Consequences). Open Questions. Revisit Candidates.

Update status header.

---

### Step 2.17 — todos.md

📖 **Read first:** `references/todos.md`

Keep the Priority Legend as-is. Fill in TODO Items using `- [ ]` checkboxes under each priority tier (🔴 P0, 🟠 P1, 🟡 P2, 🟢 P3). Source TODOs from: gap-analysis Critical Gaps → P0, failure-modes Recommended Fixes → P0/P1, test-strategy gaps → P1/P2, performance opportunities → P2/P3, observability gaps → P2, configuration hardcoded values → P3, safety gaps → P1.

Update status header.

---

### Step 2.18 — glossary.md

📖 **Read first:** `references/glossary.md`

Fill in: Terms table (Term/Definition/Context), Abbreviations table, Related Glossaries. Source terms from all 17 previous docs and from type/constant names in source code.

Update status header.

---

## Phase 3: Validate

### Step 3.1 — Run Validation Script

Run `scripts/validate-spec.ps1 -Subsystem "<subsystem>"` from the skill directory.

Fix any issues it reports: missing headers, unfilled placeholders, missing checkboxes, code blocks, etc.

### Step 3.1b — Run Quality Scan

Run `scripts/quality-scan.ps1 -Subsystem "<subsystem>"` from the skill directory.

This checks substance, not just structure. It grades each document A-F based on:

- **Minimum line thresholds** (dynamic, based on source file count -- see Quality Tiers)
- **Placeholder evasion** ("varies", "TBD", "N/A" in table cells)
- **Filler phrase density** (word salad detection)
- **Code file reference validity** (do cited .go files actually exist?)
- **Cross-document consistency** (narrative arc alignment between pillar docs)
- **Table completeness** (empty or single-word cells flagged)
- **GIBBERISH detection** (adversarial adverb padding -- hard fail)

If any documents score below A, the script emits a **SOURCE MANDATE** block listing every `.go` file, test file, and non-Go artifact in the mapped source directory, plus a prioritized list of failing docs. **You MUST read ALL listed source files BEFORE rewriting any spec doc.** Docs written without reading the source will be caught by the GIBBERISH and GROUNDING checks.

A subsystem is not "Complete" until it achieves grade **A** across all 18 documents. Fix any issues before proceeding.


### Step 3.2 — Update INDEX.md

Open `Docs/Spec/INDEX.md`. Find the row for the subsystem you just completed. Replace the `<!-- one-line description -->` placeholder with a one-sentence description pulled from what you wrote in README.md's "What Is This?" section.

For example, change:
`| [campaign](internal/campaign/README.md) | <!-- one-line description --> |`
to:
`| [campaign](internal/campaign/README.md) | Multi-phase autonomous goal orchestration with decomposition, execution, and verification |`

### Step 3.3 — Run Completion Report

Run `scripts/completion-report.ps1` to verify the subsystem shows as complete.

### Step 3.4 — Update Tracking

Update `task.md` in conversation artifacts to mark this subsystem as `[x]` complete.

---

## Golden Rules (Never Violate)

1. **PRESERVE the template structure.** Replace `<!-- comment -->` placeholders only. Keep every header, table, and section skeleton.
2. **NEVER delete template sections.** If inapplicable, write "Not applicable — [reason]" under the header.
3. **Natural language ONLY.** No code snippets, no function signatures, no Go/Mangle syntax. Prose, tables, lists. Mermaid diagrams allowed in data-flow.md only.
4. **Ground every claim in source code.** Reference specific filenames.
5. **Tables for lists, prose for explanations.** Follow the format the template gives you.
6. **Update the Spec Status** from `🔴 Not Started` to `🟢 Complete` and update `Last Updated`.
7. **Maintain the narrative arc.** current-state → gap-analysis → north-star must tell a coherent story. Every gap must trace backward to a current-state limitation AND forward to a north-star goal. If the thread breaks, the docs are disconnected.

## Common Mistakes (Ranked by Severity)

1. **OVERWRITING templates** — Replacing the template structure with your own format
2. **Word salad** — Vague prose that sounds professional but conveys nothing concrete
3. **Speculative claims** — Describing what code "should" do instead of what it does
4. **Skipping sections** — Deleting headers instead of writing "Not applicable"
5. **Code snippets** — Inserting Go, Mangle, or JSON
6. **Missing file references** — Not naming specific source files

## Validation Scripts

- **`scripts/validate-spec.ps1`** — Template compliance and structural checker. Run with `-Subsystem "internal/campaign"` or `-All`. Add `-Summary` for compact output. Add `-Strict` to enable anti-cheating checks (placeholder evasion, word salad, code file grounding).
- **`scripts/quality-scan.ps1`** — Substance and quality grader. Grades each document A-F and produces an overall subsystem grade. Checks filler density, specificity, cross-doc consistency, and pillar doc depth. A subsystem needs grade B+ to be considered truly complete.
- **`scripts/completion-report.ps1`** — Sprint progress with dynamic subsystem discovery from source directories.

## External References

- **Scaffold Templates:** `Docs/Spec/scaffold.ps1`
- **Methodology:** `Docs/Spec/METHODOLOGY.md`
- **Spec Root:** `Docs/Spec/` (git-ignored on purpose)
