# init INTERNALS — how the cold-start pipeline decides and stores

Companion to README.md. Everything below is read from the code it cites.

## 1. Orientation first, and only what changed

`runOrientation` (`internal/init/phase_orient.go:130`) measures the repository
(`orient.Measure`), keeps the engine for every later phase, and asks the
planner what each read-candidate document is. A document whose body is
byte-identical to the last orientation (`oriented_document` digests in the
previous `snapshot.json`) keeps its role claims and is not sent again.
`deriveNorthStar` (`:261`) drafts the north star from the brief unless a
vision is stored and every oriented document is unchanged, then loads the
vision of record (drafted now, earlier, or written by the user) into
`orientedVision`. Derive prompts compile for the planner via
`northstar.PhasePromptFor`.

## 2. The roster is derived in Mangle, from evidence

Phase 6 (`integrateEcosystem`, `internal/init/phase_ecosystem.go:131`)
asserts profile signals (language, framework, dependencies, `/tests` when the
scan found test files) into the orientation engine, reads the stack roster,
then asks the planner for domain experts (`proposeDomainExperts`,
`internal/init/domain_experts.go:115`). The request carries the north star,
the oriented documents with their roles, the measured stack and the roster so
far; the reply is validated strictly: PascalCase names, a mission, a why
citing `north_star` or an oriented document path, bounded topics, failure
modes and research questions. Valid experts become `vision_agent*` facts that
`internal/orient/vision_agents.mg` merges into `orient_agent*`. The proposal
and its request digest are kept in `.nerd/orientation/domain_experts.json`;
an unchanged request reuses it.

`internal/orient/ecosystem_agents.mg` keeps SecurityAuditor and TestArchitect
when the north star supports them (`vision_core_support`) or the scan does
(a security surface: request-serving frameworks, data stores, browser
automation, credentials libraries; tests on disk). Imported subagents and
modes are bounded by `orient_imported_agent_max`, most recently maintained
first. Names that fold to the same letters are one agent
(`agentNameResolver`, `phase_ecosystem.go:389`). Recorded rejections from an
earlier interactive selection are honored on non-interactive runs.

Bounds are `orient_param` rows with defaults in `vision_agents.mg`
(`orient_domain_agent_max` 6, `orient_domain_agent_topic_max` 8,
`orient_imported_agent_max` 8, `orient_research_stale_days` 30); a
`config_param` of the same key overrides once `config.OrientConfig` carries it.

## 3. Agent knowledge bases

`createType3Agents` (`internal/init/agents.go:426`) builds each agent on a
bounded worker pool (`createAgent`, `:504`):

1. Corpus knowledge from winning imported sources (paragraphs, whole).
2. Stack topics through Context7 (`createAgentKnowledgeBase`,
   `internal/init/agents_knowledge.go:34`): no pass deadline (each fetch has
   `research.context7_timeout`), covered topics skipped on upgrade, and a
   topic two agents share is fetched once per run.
3. A vision expert's research questions by grounded web search on the main
   client (`researchDomainQuestions`, `internal/init/domain_research.go:90`).
   An answer with no source is not stored. Atoms carry `Question:`,
   `Sources:` and `Retrieved:`; a fast-moving answer older than the
   staleness bound is researched again and replaced only once a grounded
   answer is in hand.

`prompts.yaml` is generated on the planner when absent (on every run, so a
failed earlier attempt is retried) and never replaces a curated file.

### Dependency identity in acquisition subjects

Technology classification may group dependencies into a family such as `echo`;
knowledge acquisition and invalidation must retain the actual admitted Go module
path and manifest version. In addition to exact matches and vocabulary packages
inside a required parent module, an unversioned vocabulary family root must
recognize a valid semantic-major module path such as `github.com/labstack/echo/v4`.
That third relation emits the actual versioned module, not the family root.
Use the Go module parser's path/version rules and reject arbitrary descendants
or lookalike repositories. Keep distinct required majors as distinct subjects;
preserve npm, standard-library and package-inside-parent behavior.

Both init and background refresh use `KnowledgeSubjects`. Its exact subject key
must reach GoDoc/repository acquisition and the existing `dependency_pin` /
`knowledge_acquired_version` join. Mangle retains research admission, freshness
and budget decisions. Historical generic knowledge is not relabeled or deleted.
This contract does not establish effective versions under replacements or workspaces.

| Gap ID | Deficiency | Target and exit criterion | State |
|---|---|---|---|
| GAP-INIT-GO-MAJOR-SUBJECT | The former matcher lost `/vN` identity at the family boundary. | Subject tests preserve exact path, ecosystem and version for distinct majors and retain existing controls; real Cortex refresh derives `/version_changed` for a versioned dependency upgrade and does no work for an unchanged pin. | Implemented; focused verification passed. |

`pinnedPackages` (`internal/init/knowledge_sources.go:96`) keeps the existing
package match and separately checks semantic-major paths with `module.SplitPathVersion`,
`CheckPath` and `CheckPathMajor`. It emits each matched actual module once, with
its admitted version. `TestKnowledgeSubjects_GoMajorFamily` and
`TestKnowledgeSubjects_GoMajorRejectsUnrelatedPaths` (`knowledge_sources_test.go:67`)
cover the identity boundary. `TestKnowledgeRefreshResearchesVersionedGoModule`
(`internal/system/knowledge_refresh_version_test.go:84`) follows normal Cortex
refresh and checks the exact persisted `/v4` subject after an upgrade.

## 4. Strategic knowledge is replaced, not appended

`PersistStrategicKnowledge` (`internal/init/strategic_knowledge.go:144`)
owns the `strategic/*` concepts except `strategic/source/*`: an identical set
is left alone, a changed one replaces the old, empty values and "unknown" are
not knowledge. `syncStrategicSources` keeps `strategic/source/<path>` equal to
the current read set without re-embedding unchanged documents.
`/refresh-docs` extracts only new or changed admitted documents (all with
`--force`), retires the atoms of a changed or no-longer-admitted document,
and stores nothing for a reply that is not the requested JSON.

## 5. Tools

Phase 7e (`selectProjectTools`, `internal/init/project_tools.go:82`) reads the
commands the repository declares (root Makefile targets, package.json scripts
with the lockfile's runner) and asks the planner which an agent should run;
only declared commands are accepted. Phase 10 writes language tools for every
measured language (primary and per-module) plus those project tools; phase 11
limits agent tool grants to that set, because boot refuses an agent naming an
unregistered tool.

---
Verified 2026-10-27 against the working tree.
