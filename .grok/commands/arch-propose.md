---
name: arch-propose
description: Research-driven pre-implementation architecture corpus generator. Investigates a brand-new feature with no code via 4 parallel scouts, synthesizes candidates, stress-tests them, and emits a 21+ doc Pre-Implementation corpus under Docs/architecture/<feature>/.
argument-hint: <feature> [--expand] [--refresh] [--tier 1|2|3] [--sources path1,path2] [--force]
allowed-tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
  - Agent
  - AskUserQuestion
  - Skill
---

Run the /arch-propose pre-implementation architecture pipeline for a feature that has no implementation code yet.

This command is the research-first sibling of ``spec-doc-sprint`. Where ``spec-doc-sprint` audits source and writes docs, `/arch-propose` runs scouts, synthesizes candidates, stress-tests them, and emits a full corpus marked as Pre-Implementation.

## Argument Parsing

Parse the user's input:

- `<feature>` (required): subsystem name (e.g., `search`, `forge-agent-guided-training`, `ontology-mission-bridge`)
- `--expand` (optional): partial-corpus mode — existing `Docs/architecture/<feature>/` files are preserved and only gaps filled
- `--refresh` (optional): full-corpus mode — backup and regenerate every file (use with caution)
- `--tier 1|2|3` (optional): force tier classification (default: estimated from candidate scope)
- `--sources path1,path2` (optional): mark as virtual subsystem spanning multiple planned packages
- `--force` (optional): required to overwrite a corpus directory that has already been rebuilt to the doc standard (files carrying `implementation-status: shipped` front-matter)

If no `<feature>` was provided, list existing "Proposed Subsystems" from `Docs/architecture/INDEX.md` (if the section exists) and "Incomplete Directories" from that index, then ask which feature to target.

## Preflight

1. Read `Docs/journeys/09-architecture-doc-standard.md` for the corpus doc standard and front-matter vocabulary.
2. Load `arch-propose` skill via Skill tool to load Pre-Implementation rules.
3. Check if `internal/<feature>/` exists. If yes AND `--refresh` not set: refuse with message "Use ``spec-doc-sprint` for existing code. `/arch-propose` is pre-implementation only. Pass `--refresh` to force pre-implementation regeneration of docs over existing code (destructive)."
4. Check if `Docs/architecture/<feature>/` exists:
   - None → pure pre-implementation mode
   - Exists with files → require either `--expand` (gap-fill) or `--refresh` (regenerate)
   - Directory already rebuilt to the doc standard (any file with `implementation-status: shipped`) → require `--force`. Never overwrite a shipped-status file without it.
5. Create scratch dirs under `.arch-propose/{north-star,research/{internal,literature,convergent,divergent},candidates,collaborations,decision,diff,backups,journal}`. Add `.arch-propose/` to `.gitignore` if not present.

## Phase 0 — North-Star Interview

Ask the user 3–5 questions via AskUserQuestion to crystallize intent. At minimum:

1. **Problem statement**: what gap does this feature fill? What's broken or missing today? (free-text, pass as-is)
2. **Subsystem placement**: is this a new subsystem, or does it extend an existing one?
   - Options: "New subsystem", "Extends subsystem X (name it in 'Other')", "Genuinely not sure — let scouts decide"
3. **Success criteria**: how do we know this is "done"? What observable behavior proves it works?
4. **Scope boundaries**: what is explicitly OUT of scope for this proposal?
5. **Ownership**: which package owns the kernel predicates and store tables this feature adds? (suggest `internal/<feature>/` as default)

For `--expand` mode, add a 6th question: which existing files in `Docs/architecture/<feature>/` should be preserved vs. superseded?

Write `.arch-propose/north-star/<feature>.md` with all answers.

## Phase 1 — Parallel Research (blocking)

Dispatch 4 scouts in parallel. Each writes to its dimension-specific file. ALL must complete before Phase 2.

```
Agent(subagent_type="arch-propose-scout-internal", model="sonnet", run_in_background=true, prompt="<pass feature, north_star_path, subsystem_hint from Phase 0 question 2, expand_mode>")
Agent(subagent_type="arch-propose-scout-literature", model="sonnet", run_in_background=true, prompt="<pass feature, north_star_path, problem_class_hint if user supplied one>")
Agent(subagent_type="arch-propose-scout-convergent", model="sonnet", run_in_background=true, prompt="<pass feature, north_star_path, and tell it to poll internal/literature scout outputs as they become available>")
Agent(subagent_type="arch-propose-scout-divergent", model="opus", run_in_background=true, prompt="<pass feature, north_star_path, problem_class_hints>")
```

Wait for all 4 to finish. Verify each dossier file exists and meets the quality floors in `.agents/skills/arch-propose/references/pre-implementation-phase-checklist.md` (Phase 1 checklist). If any fails the floor, re-dispatch with specific feedback rather than proceeding with thin input.

## Phase 2 — Synthesis

Dispatch the synthesizer with all 5 input paths (north-star + 4 dossiers):

```
Agent(subagent_type="arch-propose-synthesizer", model="opus", prompt="<pass all 5 paths, expand_mode, diff_path if --expand>")
```

Verify `.arch-propose/candidates/<feature>.md` exists with 2–3 candidates each having mandatory fields (fact and store ownership, read-before-write strategy, integration surface with real file:line). Also verify `.arch-propose/collaborations/<feature>.md` has the initial `[SYNTHESIZER Round 1]` block.

### Phase 2.5 — Candidate Schema Validation (hard gate)

For EACH candidate in `.arch-propose/candidates/<feature>.md`, check the presence of these required fields. If ANY candidate is missing ANY field, HALT — do not proceed to interrogation.

Required per-candidate fields:
- `Subsystem placement` section naming host package path
- `Fact and store ownership:` — explicit owning package, non-empty
- `Read-before-write strategy:` — concrete description, not `TBD` / `TODO` / empty
- `Integration Surface` table with at least one row, each row with `file:line` citation resolving to a real file (spot-check via `grep -n` on the cited line)
- `Data Flow` ASCII diagram block (not empty, not placeholder `TODO`)
- `planned_go_file_inventory` — the list of planned `.go` files in `§Package Structure (Planned)` with non-zero count
- `estimated_tier` — 1, 2, or 3, consistent with inventory count (20+ files=1, 10-19=2, <10=3)
- `Invariants` — at least 1 item
- `Gating requirements` — at least 1 gate (no time estimates — ordering only)

On halt: write the validation failure to `.arch-propose/candidates/<feature>.md` under a new `## Validation Failures` section, notify the user via a concise message naming which candidate fails which field, and stop. The user can then re-dispatch the synthesizer with the specific feedback, or abort the run.

### Phase 2.6 — Tier Selection

If user passed `--tier N`, use that value (override).

Otherwise:
- Read the `estimated_tier` from the WINNING candidate (set in Phase 3.5 — for now, take the synthesizer's recommended candidate from the `[SYNTHESIZER Round 1]` bias block).
- If all candidates agree on tier, use that.
- If candidates disagree, defer tier selection to Phase 3.5 (the judge uses the winner's tier).
- NEVER fall back to "count .go files" for a pre-implementation feature — that yields 0 and would always produce Tier 3 regardless of scope. If no planned inventory exists (should never happen post-Phase 2.5), HALT with an error.

## Phase 3 — Socratic Interrogation

Dispatch `requirements-interrogator` on the candidates. It appends `[INTERROGATOR Round N]` blocks to the collaborations file.

**Hard limits (fail-closed)**:
- `max_interrogation_rounds = 3`
- `max_synthesizer_retries = 1`
- Accepted verdict regex: `^\*\*Verdict\*\*:\s*(READY|NEEDS_WORK|RESOLVED)\b` (case-sensitive, anchored at line start)
- If no line matching the verdict regex appears after `max_interrogation_rounds` rounds → treat as NEEDS_WORK with severity CRITICAL (fail-closed default).

```
Agent(subagent_type="requirements-interrogator", model="opus", prompt="
Interrogate the architectural proposal for <feature>.
Read the collaborations file at .arch-propose/collaborations/<feature>.md (the synthesizer's first block is there).
Apply the 20-dimension rubric selectively — focus on dimensions that bind for this feature's domain.
Produce at least 2 rounds; at most 3 rounds (HARD LIMIT — do not exceed).
Mirror the interrogation to .claude/interrogations/arch-<feature>.md as you go.
At the end of your final round, emit exactly one line matching:
  **Verdict**: READY
  OR
  **Verdict**: NEEDS_WORK
  OR
  **Verdict**: RESOLVED
No other verdict syntax is accepted. The orchestrator pattern-matches this line to decide flow.
")
```

After the interrogator completes, grep the collaborations file for the verdict regex. Behavior by verdict:

- `READY` or `RESOLVED` → proceed to Phase 3.5.
- `NEEDS_WORK` with any CRITICAL items → re-dispatch synthesizer with NEEDS_WORK items appended to its input. Counter: `synthesizer_retries += 1`. If retries exceed `max_synthesizer_retries` (1), stop retrying. Carry remaining CRITICAL items into TODO P0 and proceed to Phase 3.5. The flaws become the backlog, not blockers.
- No verdict line at all → treat as NEEDS_WORK CRITICAL per fail-closed default, apply same retry logic.

Record the retry count and final verdict in the journal (Phase 7).

## Phase 3.5 — Judge & Seal

Read the interrogation. Count candidates with Verdict ∈ {READY, RESOLVED}.

- 1 candidate viable → auto-select as winner
- 2–3 candidates viable → AskUserQuestion: present each candidate's Slot + one-line summary + biggest tradeoff; let the user pick
- 0 viable → halt, write failure journal, surface to user with the NEEDS_WORK list

Write `.arch-propose/decision/<feature>.md` with:
- Winner: Candidate {A/B/C}
- Rationale: 3–5 lines on why this candidate over the others
- Carry-forward items: every CRITICAL interrogator finding → TODO P0 line
- Carry-forward open questions: every unresolved thread → OPEN-QUESTIONS.md OQ-N

## Phase 4 — Synthetic Audit Construction

Dispatch the auditor:

```
Agent(subagent_type="arch-propose-auditor", model="sonnet", prompt="<pass feature, all 9 artifact paths, output_dir, target_tier, expand_mode>")
```

Verify `Docs/architecture/<feature>/.code-audit.md` exists with:
- The ⚠ Synthetic banner
- All 14+ sections
- §15 Candidate Provenance linking every artifact
- Spot-check 3 file:line citations by reading the cited files

## Phase 4.5 — Corpus Diff (`--expand` mode only)

Skip if pure pre-implementation.

Enumerate the 21+ expected files for the target tier (foundation 00-04, IMPLEMENTED_SPEC, deep-dives, 9 cross-cutting docs, governance docs, TESTING-STRATEGY, ECOSYSTEM-IMPACT). For each existing file in `Docs/architecture/<feature>/`, classify:

- **keep** — user-authored and still canonical (README, CLAUDE, AGENTS if present, any custom content outside the standard template)
- **supplement** — standard doc that exists but is thin; preserve and append, don't overwrite
- **supersede** — standard doc that was a stub or wrong-template; back up and regenerate

Write `.arch-propose/diff/<feature>.md` with the classification table.

### Phase 4.5a — Renumbering Map (mandatory when existing cross-cutting docs use old numbering)

The cross-cutting doc order is fixed: CLI-TUI-SURFACE, DEPENDENCY-MAP, CONSTITUTIONAL-SAFETY, TESTING-ALIGNMENT, CROSS-SYSTEM-WIRING-JOURNAL, TELEMETRY-OBSERVABILITY, TESTING-REMEDIATION-SURFACE, KERNEL-VIRTUALSTORE-SURFACE, CAMPAIGN-CONTROLLABILITY. Any partial corpus whose cross-cutting docs use an older order requires a rename + link-rewrite pass.

Build a renumbering map table in `.arch-propose/diff/<feature>.md`:

| Old file name | Content topic | New file name | Action |
|---|---|---|---|
| `07-TESTING-REMEDIATION-SURFACE.md` | testing-remediation | `{NN+6}-TESTING-REMEDIATION-SURFACE.md` | rename |
| `08-KERNEL-VIRTUALSTORE-SURFACE.md` | kernel-virtualstore-surface | `{NN+7}-KERNEL-VIRTUALSTORE-SURFACE.md` | rename |
| `09-CAMPAIGN-CONTROLLABILITY.md` | campaign-controllability | `{NN+8}-CAMPAIGN-CONTROLLABILITY.md` | rename |

Where `{NN}` is the target's first cross-cutting position (per tier + deep-dive count).

Renumber execution:
1. Back up the old-named file to `.arch-propose/backups/<feature>/<date>/` before renaming.
2. `git mv` the file if the repo is clean for that path, else `mv`.
3. Grep `Docs/architecture/<feature>/` for references to each old filename (e.g., `07-TESTING-REMEDIATION-SURFACE.md`). Rewrite every match to the new filename.
4. Grep `Docs/architecture/INDEX.md` and other cross-system docs for references to the renamed files; rewrite if found.
5. Record every renamed file + every rewritten reference in the diff artifact so the journal has the audit trail.

### Phase 4.5b — Backup Policy

Back up every `supersede` file AND every renamed file to `.arch-propose/backups/<feature>/<YYYY-MM-DD>/` before any regeneration or rename begins. Also back up the legacy `IMPLEMENTED_SPEC.md` if it exists and is being regenerated (it isn't in normal flow, but it might be in edge cases).

The backup directory must be byte-for-byte reconstructable via `diff -r`. If any backed-up file differs from the pre-run snapshot (per Phase 0 preflight), HALT — the run modified state before the backup completed.

## Phase 5 — Corpus Generation

### 5a. arch-writer dispatch

```
Agent(subagent_type="arch-writer", model="opus", prompt="Generate the foundation docs (00-04), IMPLEMENTED_SPEC.md, and deep-dives (05-NN) for the pre-implementation <feature> subsystem. The .code-audit.md at <audit_path> is SYNTHETIC — it carries a ⚠ Pre-Implementation banner. Suspend your file:line citation requirement for foundation docs 00-04 and IMPLEMENTED_SPEC §§3-4 per the auditor's preamble. ENFORCE citations for 02-CURRENT-STATE Section 2 (existing utilities), deep-dive adjacent-subsystem references, and any reused-utility mentions. Tier: <tier>. Output dir: Docs/architecture/<feature>/. Also respect the Pre-Implementation markers in the synthetic audit (0% status rows, target-state vision, gap analysis as implementation roadmap). NO TIME ESTIMATES anywhere.")
```

In --expand mode, pass the diff classification so arch-writer skips `keep` files and merges into `supplement` files instead of overwriting.

### 5b. cross-cutting-analyst dispatch (all 9 docs)

```
Agent(subagent_type="cross-cutting-analyst", model="sonnet", prompt="Generate ALL 9 cross-cutting docs for pre-implementation <feature>. The audit at <audit_path> is synthetic (has the ⚠ banner), so activate your Pre-Implementation Mode. Produce: CLI-TUI-SURFACE, DEPENDENCY-MAP, CONSTITUTIONAL-SAFETY, TESTING-ALIGNMENT, CROSS-SYSTEM-WIRING-JOURNAL, TELEMETRY-OBSERVABILITY, TESTING-REMEDIATION-SURFACE, KERNEL-VIRTUALSTORE-SURFACE, CAMPAIGN-CONTROLLABILITY. Fixed order; zero-padded NN numbering starting at <start_number> per tier rules. For each: be honest about 'no CLI surface yet', 'no tests yet', 'no log fields yet' — write recommendations and planned surfaces, not fabricated observations. Every adjacent-subsystem citation MUST have a real file:line (Pre-Implementation Mode suspends file:line only for the feature's own pre-implementation code, never for adjacent code it integrates with).")
```

Verify all 9 files exist after the agent completes. If any missing, re-dispatch with the specific gap listed — the agent's prompt includes all 9 so this should not occur unless the agent errors out.

### 5c. test-strategist dispatch (parallel with 5a/5b)

The cross-cutting `TESTING-ALIGNMENT.md` and `TESTING-REMEDIATION-SURFACE.md` are template fills. The test-strategist produces a deeper, actionable testing plan in `TESTING-STRATEGY.md`.

```
Agent(subagent_type="arch-propose-test-strategist", model="sonnet", run_in_background=true, prompt="<pass feature, audit_path, candidates_path, decision_path, internal_scout_path, north_star_path, output_dir>")
```

Verify `Docs/architecture/<feature>/TESTING-STRATEGY.md` exists with all 10 sections. The doc is un-numbered (sibling to TODO.md / OPEN-QUESTIONS.md), and is referenced by the cross-cutting test docs.

### 5d. ecosystem-mapper dispatch (parallel with 5a/5b/5c)

The cross-cutting docs cover DEPENDENCY-MAP, CROSS-SYSTEM-WIRING, KERNEL-VIRTUALSTORE-SURFACE. The ecosystem-mapper covers the BROADER ripple impact: kernel facts and policy, perception and routing, JIT prompts, tools and MCP, session and shards, campaigns, persistence, world model and context, autopoiesis, verification, CLI, config and boot wiring, observability, tests, and docs. Final section is the implementer checklist.

```
Agent(subagent_type="arch-propose-ecosystem-mapper", model="opus", run_in_background=true, prompt="<pass feature, audit_path, candidates_path, decision_path, internal_scout_path, north_star_path, output_dir>")
```

Verify `Docs/architecture/<feature>/ECOSYSTEM-IMPACT.md` exists with all 16 touchpoint sections + the implementer checklist. Halt if any section is silently skipped (each skip must include a one-sentence "Not applicable" justification).

### 5e. Parallel Join

5a (arch-writer), 5b (cross-cutting-analyst), 5c (test-strategist), 5d (ecosystem-mapper) all run in parallel where possible. Both 5c and 5d can start as soon as the synthetic audit is ready (they don't depend on arch-writer's foundation/spec output). 5b can start in parallel too. Wait for ALL FOUR to complete before Phase 6.

## Phase 6 — Governance + Index

### 6a. TODO.md

Derive T-NNN items from:
- Phase 3 CRITICAL/IMPORTANT interrogator findings → P0
- §14 Key Findings from audit marked "must appear in TODO" → P0
- Candidate's gating requirements → P1 (each gate becomes a T item)
- §12 Recommended Uplifts in IMPLEMENTED_SPEC (arch-writer's output) → P1/P2
- `--expand` legacy file reconciliation → P2 (e.g., "Reconcile legacy IMPLEMENTED_SPEC.md vs. regenerated version")

Follow the TODO format used in existing `Docs/architecture/*/TODO.md` files.

### 6b. OPEN-QUESTIONS.md

Every OQ-N from the candidates doc + every unresolved interrogation thread becomes an OQ entry with options table + recommendation. Follow the format used in existing `Docs/architecture/*/OPEN-QUESTIONS.md` files.

### 6c. agents.md

Subsystem-specific agent guidance. Include:
- Planned source location
- Adjacent subsystems the feature integrates with
- Relevant invariants from the winning candidate
- Owning package for the feature's predicates and tables
- Read-before-write pattern
- Any interactions with packages whose `agents.md` restricts changes (kernel, constitutional gate, articulation)

### 6d. README.md

Human-oriented overview. Short (50-100 lines). Reading order. Status banner matching the Pre-Implementation marker.

### 6e. _progress.md

All doc-generation checkboxes CHECKED (since this run produced them). All implementation checkboxes UNCHECKED (no code exists).

### 6f. Docs/architecture/INDEX.md patch

Find or create the "Proposed Subsystems (Pre-Implementation)" section. Insert this feature with: directory, doc count (count files after generation), generation date, target tier, one-line summary. If `--expand` moved the feature out of "Incomplete Directories," remove the old row there.

Do not add the feature to Tier 1/2/3 tables.

### 6g. ADR (Tier 1 candidates only)

Create `adr/0001-candidate-selection.md` documenting the Phase 3.5 decision: options considered, interrogation verdict, chosen candidate, why.

## Phase 7 — Journal + Compliance

### 7a. Journal

Write `.arch-propose/journal/<YYYY-MM-DD>_<feature>.md` containing:

- All input paths (north-star + 4 scouts + candidates + interrogation + decision + diff)
- All output paths (every generated doc)
- Interrogation verdict trace (list the [INTERROGATOR Round N] severity distributions)
- Compliance-check results (Rule 1–10 verification from pre-implementation-markers.md)
- `SEED:` cross-pollination markers — at least 1 per run:
  - `SEED:subsystem-X:<insight>` — findings other subsystems should absorb
  - `SEED:reuse:<pattern>` — reusable patterns for future `/arch-propose` runs
  - `SEED:gap:<capability>` — identified cross-system gaps worth their own run

Final verdict: COMPLETE / PARTIAL / FAILED.

### 7b. Compliance Grep Gate

Run the grep-checkable rules from `.agents/skills/arch-propose/references/pre-implementation-markers.md`, plus a no-time-estimates grep. Any failure halts and records in the journal.

```bash
# Rule 1: §3 all-zero
grep -c "100%" Docs/architecture/<feature>/IMPLEMENTED_SPEC.md   # expect 0
grep -cE "(Complete|Fully implemented|Done)" Docs/architecture/<feature>/IMPLEMENTED_SPEC.md  # expect 0 in status contexts (table rows)

# Rule 2: banner present
grep -l "⚠ Pre-Implementation" Docs/architecture/<feature>/IMPLEMENTED_SPEC.md  # must match

# Rule 3: 02-CURRENT-STATE verbatim honesty
grep "None. No code has been written" Docs/architecture/<feature>/02-CURRENT-STATE-*.md  # must match

# Rule 7: Docs/architecture/INDEX Proposed section contains feature; feature NOT in tier tables
grep -A2 "Proposed Subsystems" Docs/architecture/INDEX.md | grep "<feature>"  # must match
! grep -E "^\|\s*\[<feature>\]" Docs/architecture/INDEX.md | grep -E "Tier [123]"  # must NOT match

# Doc count
ls Docs/architecture/<feature>/*.md | wc -l   # expect 21+ for Tier 3, 22+ for Tier 2, 28+ for Tier 1

# NO TIME ESTIMATES
# This regex matches any common time-estimate pattern; allowed exceptions (benchmarked runtime citations, complexity classes) are grep-filtered.
# The pipeline should produce ZERO hits from this pattern across all generated docs.
grep -rEn "\b(weeks?|sprints?|story[- ]points?|person-weeks?|person-months?|~\s*[0-9]+\s*(hours?|days?|weeks?|months?)|Q[1-4]\s*20[0-9]{2}|by\s+end\s+of\s+(Q[1-4]|20[0-9]{2}))\b" Docs/architecture/<feature>/*.md \
  | grep -vE "\b(benchmark|complexity|O\()" \
  | head -20
# If any lines appear, the pipeline leaked a time estimate. Journal and remediate.
```

The time-estimate grep is permissive (allows "weeks" in benchmarked-runtime context; excludes algorithmic complexity class O(...) mentions). If it fires on a false positive, note the exclusion in the journal and proceed. If it fires on a real leak, halt and re-dispatch arch-writer with explicit removal instructions for the offending lines.

### 7c. Codex review offer

After successful run, offer (do not auto-dispatch): "Dispatch Codex adversarial review of the generated corpus? [/codex:adversarial-review]"

## Final Report to User

Surface to the user:
- Path: `Docs/architecture/<feature>/`
- Doc count: N files (foundation + spec + deep-dives + 9 cross-cutting + governance + TESTING-STRATEGY + ECOSYSTEM-IMPACT)
- Winning candidate: {A/B/C} ({Slot role}) — {one-line summary}
- Target tier on graduation: {1/2/3}
- CRITICAL TODOs: N items
- Open questions: N items
- **TESTING-STRATEGY.md highlights**: {tier counts — N unit tests, M integration tests, K cross-system tests planned}
- **ECOSYSTEM-IMPACT.md highlights**: {bullet list of the most important touchpoints flagged — e.g., "Requires new policy file under internal/core/defaults/policy/", "Adds new prompt atoms", "New .nerd/config.json key"}
- **Implementer Checklist**: ECOSYSTEM-IMPACT.md §3 — N items across 7 phases
- Journal: `.arch-propose/journal/<date>_<feature>.md`
- Suggested next step: implement once you decide to, or `/arch-propose <feature> --expand` to iterate further
