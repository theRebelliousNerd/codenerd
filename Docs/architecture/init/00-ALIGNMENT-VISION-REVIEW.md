# init — Alignment & Vision Review

> Last verified: 2026-07-13  
> Evidence base: `internal/init/*.go`, `cmd/nerd/cmd_init_scan.go`

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully aligned and wired end-to-end |
| 4 | Strong alignment; minor gaps or stubs |
| 3 | Partial: intent clear, important holes |
| 2 | Thin or mostly aspirational in code |
| 1 | Conflicts with north star or missing |

## Dimension scores

| Dimension | Score | Evidence |
|-----------|------:|----------|
| **LLM as creative center** | 4 | Strategic knowledge, doc relevance filtering, optional Gemini grounding (`strategic_knowledge.go`, `initializer.go` grounding helper). Deep analysis phase intentionally deferred to JIT session path rather than embedding a Researcher domain shard. |
| **Mangle kernel as executive** | 3 | Init creates `profile.mg`, loads scan facts via `kernel.LoadFacts`, templates `mangle/extensions.mg` + `policy_overrides.mg`. Does **not** run a full OODA/permitted loop during init; uses kernel as fact sink + doc_ingestion asserts. |
| **Constitutional safety / default deny** | 2 | Policy template comments show `permitted(...)` examples; no runtime `permitted` checks inside init itself. File writes are local to `.nerd/`. Permissions on agents are metadata for later spawn, not enforced here. |
| **JIT prompt atoms** | 4 | `jit_integration.go` builds phase-scoped atoms (`/analysis`, `/profile`, `/facts`, `/kb_agent`, `/agents`); `populateProjectAtoms` + `initializePromptDatabase` + `prompt.ReloadAllPrompts` sync YAML→DB. Comment in `profile.go` notes project atoms may land only in `knowledge.db` unless also ingested into corpus.db. |
| **Wiring completeness** | 3 | CLI `nerd init` / `scan` live; chat uses `IsInitialized`, session load/save, status tools. Interactive agent selection and Type-U flags are implemented as library APIs; CLI wiring for interactive curation / `--define-agent` is partial (see gap analysis). Tool generation phase is **stubbed** (Ouroboros/VirtualStore deferred). |
| **Inversion of control** | 4 | Detection is rule/file-based (deterministic); LLM used for strategic “soul” synthesis and optional research, not for executive control of what directories exist or whether init succeeds. Agent recommendation is heuristic switch tables, not free-form LLM inventing agents as system of record. |
| **Durable memory substrate** | 5 | Core job: materialize `.nerd/` stores (knowledge.db, shard KBs, northstar DB, corpus.db, agents.json, session.json, tools catalog). Upgrade/migrate paths via `store.MigrateAllAgentDBs` and validation. |
| **Observability** | 4 | `logging.CategoryBoot` / `CategoryStore`; progress channel + ETA tracker; stdout phase banners; post-init validation summary. |
| **Test grounding** | 4 | Broad unit coverage on pure functions (parsers, prefs, ETA, validation helpers, Type-U parsing). Full `Initialize` end-to-end less dense (embedding/LLM dependent). |

**Composite (mean): ~3.7** — production cold-start package with intentional JIT refactor stubs and incomplete interactive/CLI surface.

## North-star narrative

codeNERD’s slogan is *logic determines reality; the model merely describes it*. Init’s job is to **describe the project into durable structure** so the kernel and specialists have facts and knowledge to reason over later. The package correctly:

- Prefer **file-system detection** over LLM guessing for language/deps/entry points.
- Emit **Mangle profile facts** and **knowledge atoms**, not only free text.
- Push **on-demand research** into JIT clean loop (researcher domain shard removed from init path).

Remaining tension: strategic knowledge and Context7 research still use LLM during init when a client is available — creative description, not executive control — which is aligned if failures degrade to warnings rather than blocking a usable `.nerd/` (mostly true: many phases append `Warnings` and continue).

## Verdict

**Aligned enough to ship; not fully finished.** Treat researcher-shard removal and tool-generation stub as **intentional** partials, not accidental dead code — but finish wiring interactive selection and Type-U agents into the CLI if those remain product goals.
