---
doc-class: shipped-with-future
subsystem: orient
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472 plus C2b working tree; runtime validation pending
supersedes: []
---

# 07 — North Star Derivation — Non-Interactive Vision Synthesis & Role Transduction

This capability specification details the non-interactive extraction, classification, and synthesis of a project's North Star directly from repository documentation into `internal/northstar`, owned by Lane `I2b`.

---

## 1. The North Star Ingestion Deficit

At the original `e056692c` baseline, `nerd init` did not initialize a North Star:
1. `internal/init/initializer.go:796-804` opens `northstar.NewStore` solely to run SQLite DDL migrations and immediately closes it.
2. `internal/init/initializer.go:1447` instructs the operator to manually run `/northstar`.
3. The previous interactive drafting path capped document text at 10,000 bytes. C2b replaces that path with library callers (`cmd/nerd/chat/northstar_llm.go:15`, `generateRequirementsWithLLM`; `cmd/nerd/chat/northstar_llm.go:39`, `analyzeNorthstarDocs`).
4. In Phase 7b (`internal/init/strategic_knowledge.go:25, 68-120`), an LLM call synthesizes a vision string, but commits it exclusively as a generic knowledge atom in `.nerd/knowledge.db` (`strategic_knowledge.go:616-663`), completely bypassing the North Star store.

The intended pipeline classifies read candidates, drafts the relational vision and installs it without replacing existing authority. C2b authors the library and TUI callers; C2a owns init execution, reports and orientation fact persistence. Runtime validation has not been executed by this author-only lane.

### C2b implementation witnesses (authored, validation pending)

| Contract | Source witness | Authored regression witness |
|---|---|---|
| Full document reading and small-document batching | `ClassifyDocuments`, `internal/northstar/derive.go:215`; bounded UTF-8 `nextFrame`, `internal/northstar/derive_page.go:19`. | `TestDerive_LosslessPaging`, `internal/northstar/derive_test.go:95`; `TestDerive_BatchBudgetBoundary`, `internal/northstar/derive_test.go:390`. |
| Role/evidence validation and fact projection | `applyClassification`, `internal/northstar/derive_parse.go:115`; `Classification.Facts` in `internal/northstar/derive.go:85`. | `TestDerive_RoleParsing`, `internal/northstar/derive_test.go:160`. |
| Full ordered brief, grounded fields, retained draft status and link pruning | `DraftVision`, `internal/northstar/derive.go:343`; `validateDraft`, `internal/northstar/derive_parse.go:477`; `pruneDraftLinks`, `internal/northstar/derive_parse.go:555`. | `TestDerive_WizardDocumentLinkIntegrity`, `internal/northstar/derive_test.go:214`; `TestDerive_DraftRejectsMissingOrInventedProvenance`, `internal/northstar/derive_test.go:263`; `TestDerive_DraftMarkerCannotBeEscapedByFieldCitation`, `internal/northstar/derive_test.go:537`. |
| Requirements share paging and retain every returned object | `DeriveRequirements`, `internal/northstar/derive.go:381`. | `TestDerive_RequirementsKeepAllFieldsAndObjects`, `internal/northstar/derive_test.go:277`; `TestNorthstarTUI_RequirementsUseSharedJSONParser`, `cmd/nerd/chat/northstar_llm_test.go:84`. |
| Existing authority is never updated by derivation | `InstallDerivedVision`, `internal/northstar/derive.go:545`; conditional transaction `Store.installDerivedVisionIfAbsent`, `internal/northstar/derive.go:597`. | `TestInstallDerivedVision_PreservesExistingAuthority`, `internal/northstar/derive_test.go:316`; `TestInstallDerivedVision_ConcurrentDraftsDoNotOverwrite`, `internal/northstar/derive_test.go:463`. |
| Phase-specific prompts are JIT atoms | `CompilePhasePrompt`, `internal/northstar/derive_prompt.go:129`; atom IDs at `internal/prompt/atoms/northstar/derive_classify.yaml:4`, `internal/prompt/atoms/northstar/derive_vision.yaml:4`, `internal/prompt/atoms/northstar/derive_requirements.yaml:4`. | `TestCompilePhasePrompt_DeriveAtomsOnly`, `internal/northstar/derive_test.go:378`. |
| Request bytes use typed defaults and validation | `DefaultNorthstarDeriveConfig`, `internal/config/northstar_derive.go:19`; `NorthstarDeriveConfig.Check`, `internal/config/northstar_derive.go:23`; `ParseNorthstarDeriveConfig`, `internal/config/northstar_derive.go:38`. | `TestParseNorthstarDeriveConfig`, `internal/config/northstar_derive_test.go:5`. |

The config owner still needs to embed `NorthstarDeriveConfig` into `OrientConfig`, initialize its default and call its `Check` from shared config validation. No Mangle declaration changes are made by C2b: the existing `doc_role_claim` and `doc_theme` contract is retained.

---

## 2. Document Role Transduction

The transduction library (`internal/northstar/derive.go`) provides:

```go
func ClassifyDocuments(ctx context.Context, client Completer, prompt PhasePrompt, budget DeriveBudget, docs []SourceDocument) (*Classification, error)
```

### Sensory Contracts & Invariants
1. **Lossless Paging**: Documents are read in their entirety. If a document exceeds the model request context budget, it is paged sequentially in chunks with a structured running summary carried across boundaries. Not a single character is discarded.
2. **Small Document Batching**: Small documentation files are framed into requests up to `orient.derive_request_bytes`. This bounds the complete user message, including framing and carry-forward summaries. The typed section is owned by `internal/config/northstar_derive.go`; absent values use its default, explicit invalid values fail. A summary that leaves no room for another complete UTF-8 rune fails visibly rather than being clipped.
3. **Structured Role Vocabulary**: The LLM assigns each read candidate to one or more formal roles:
   - `/vision`: explicit high-level project vision or north star.
   - `/north_star_draft`: emerging, unfinalized, or in-progress vision notes.
   - `/origin_design`: foundational inception architecture.
   - `/spec`: technical specification of a subsystem.
   - `/plan`: roadmaps, sprint plans, or task backlogs.
   - `/report`: audit findings, performance measurements, or meeting notes.
   - `/standard`: governance rules, coding standards, or style guides.
   - `/guide`: developer tutorials or onboarding manuals.
   - `/journal`: chronologically ordered engineering logs.
   - `/readme`: high-level directory or project overview.
   - `/instructions`: agent prompt instructions or rules.
   - `/reference`: API reference documentation.
   - `/archive`: obsolete or deprecated documentation.
   - `/generated`: machine-generated documentation.
4. **Evidence Line Tracking**: For every role claim, the model returns a single evidence line present in the full document. Invalid roles, confidence values, evidence and unrequested paths are recorded as rejected statements, never asserted as facts. Omitted documents are reported explicitly.
5. **EDB Predicates Emitted**:
   - `doc_role_claim(Path, Role, ConfidencePct)`
   - `doc_theme(Path, Theme)`

---

## 3. Deductive Vision Weighting in Mangle

Facts emitted by `ClassifyDocuments` are asserted into the orientation engine alongside timeline facts (`repo_era`, `doc_generation`, `doc_burst`, `doc_live`).

Policy file `internal/orient/lineage.mg` derives:
- `vision_source(Path, WeightPct, Why)`:
  Computes the authoritative sources of project vision. The calculation weights role claims (`/vision` or `/north_star_draft`) against structural evidence:
  - Higher weight for documents confirmed as live (`doc_live`).
  - Higher weight for recent commit bursts (`doc_burst`), recognizing that an active burst of architectural drafting represents current human focus.
  - Lower weight for superseded predecessors (`doc_superseded`).
  - Retention of draft status: if the highest-weighted source is `/north_star_draft`, the derived project vision is explicitly tagged as a draft in its statement.

---

## 4. Non-Interactive North Star Synthesis

Once Mangle derives `vision_source`, `origin_source`, and `repo_era`, `derive.go` executes:

```go
func DraftVision(ctx context.Context, client Completer, prompt PhasePrompt, budget DeriveBudget, brief OrientationBrief) (*Draft, error)
```

### Context Staging Order
The synthesis prompt assembles inputs in strict logical hierarchy:
1. **Development Timeline**: Major eras, active waves, lulls, and representative commit subjects.
2. **Origin Documents**: Foundational inception designs that explain architectural origins.
3. **Evolution Chains**: Traces of superseded designs and active successors.
4. **Vision Sources**: Full, unclipped text of winning vision sources in descending weight order.

### Output Domain Model
The model synthesizes a complete `WizardDocument` conforming to `internal/northstar/types.go:27-38`:
- `Mission`: Core objective and purpose.
- `Problem`: Concrete problem space being solved.
- `VisionStmt`: Long-term end state (retaining draft indicators if derived from draft sources).
- `Personas`: Target users with structured `PainPoints` and `Needs`.
- `Capabilities`: Core system capabilities categorized by timeline (`now`, `next`, `later`) based on what recent commit eras built versus long-term aspirational claims. Each capability links to personas via `Serves`.
- `Risks`: Technical and operational risks with `Likelihood`, `Impact`, and `Mitigation`.
- `Requirements`: Functional and non-functional requirements with strict relational links (`Supports` referencing capabilities, `Addresses` referencing risks).
- `Constraints`: Architectural and environmental limitations.
- **Source Citations**: Every generated field cites the repo-relative source path from which it was derived.

`Draft.Document` is the `WizardDocument`; `Draft.FieldSources` preserves scalar and indexed item provenance. Sources must occur among the origin or vision documents actually supplied. Unsourced populated fields fail parsing. Dangling relational links are pruned before `ToVision` and named in `Draft.Notes`. A policy-selected vision source marked draft keeps the statement marked, even if the model cites another supplied path. `DraftVision` performs no persistence; the caller invokes `InstallDerivedVision` after deciding whether to install.

Requirements generation shares the same library and paging contract through `DeriveRequirements`. Its input includes existing requirement IDs; new IDs never replace explicit model IDs, and duplicates fail visibly. The complete wizard input is JSON, including persona pain points, capability/risk IDs and all research insights.

---

## 5. Storage, Authority, and Persistence

The derivation flow integrates into `internal/init/phase_orient.go`:
1. **Single Authority Write**:
   The caller requests installation through `InstallDerivedVision` (`internal/northstar/derive.go:545`). Its conditional insert (`Store.installDerivedVisionIfAbsent`, `internal/northstar/derive.go:597`) performs no update on conflict, including concurrent installers. It then calls the existing JSON and Mangle writers so Guardian reconciliation can be a no-op on first boot. `TestInstallDerivedVision_FreshAuthorityBootsWithoutReimport` (`internal/northstar/derive_test.go:430`) is authored but unexecuted. Export failures return the installed authority and an error that says the row is already installed; files and SQLite are not one atomic transaction.
2. **Existing Vision Protection**:
   If a valid vision already exists in the store (e.g. from an earlier configuration or manual authoring), `init` **never overwrites it**. Instead, it writes the newly derived vision to `.nerd/northstar.derived.json` and logs an advisory note.
3. **Orientation Report Emission**:
   Generates a comprehensive markdown report at `.nerd/orientation/README.md` containing:
   - Development timeline and historical eras.
   - Origin documents and evolutionary descendants.
   - Active versus superseded specifications.
   - Vision sources with associated evidence quotes.
   - Summary of the derived North Star.
4. **Runtime Fact Persistence**:
   Emits ground EDB facts to `.nerd/orientation/orientation.mg` with header `# generated by nerd init, do not hand-edit`. This file is loaded into the real kernel on Guardian boot beside `northstar.mg`, providing permanent contextual grounding.
5. **Next Steps Clean-Up**:
   Removes the obsolete `Use '/northstar' to define your project vision` message from final init diagnostics (`internal/init/initializer.go:1447`).

---

## 6. Thinning the Interactive TUI Wizard

The Bubbletea TUI wizard (`cmd/nerd/chat/northstar_llm.go`) is refactored into a thin wrapper around `internal/northstar/derive.go`:
- Reads full files and calls `ClassifyDocuments` (`cmd/nerd/chat/northstar_llm.go:39`).
- Deletes redundant inline prompt templates in favor of shared JIT prompt atoms (`internal/prompt/atoms/northstar/derive_classify.yaml`, `derive_vision.yaml`).
- Eliminates dual-path drift between CLI and chat North Star generation.

The existing `NorthstarRequirement` DTO (`cmd/nerd/chat/northstar_types.go:57`) lacks relational fields. Until its owner adds `Supports` and `Addresses` and the adapter can retain them, `generateRequirementsWithLLM` rejects linked results with a visible error instead of dropping links (`cmd/nerd/chat/northstar_llm.go:30`). `TestNorthstarTUI_RequirementsRejectUnrepresentableLinks` (`cmd/nerd/chat/northstar_llm_test.go:135`) is authored. The old prose parser (`parseGeneratedRequirements`, `cmd/nerd/chat/northstar_types.go:78`) is now test-only and must be deleted by that file's owner.

---

## 7. Verification Seams & Tests

1. `TestDerive_LosslessPaging`: Feeds a large UTF-8 specification document to `ClassifyDocuments` with a scripted fake at the completion-client boundary, asserting exact byte reconstruction, ordered offsets, carried summaries and bounded requests.
2. `TestDerive_WizardDocumentLinkIntegrity`: Verifies that `DraftVision` outputs a `WizardDocument` whose `doc.ToVision()` execution prunes dangling persona, capability, and risk references without errors.
3. `TestPhaseOrient_PreservesExistingVision`: Runs `phase_orient` against a repository containing an existing `.nerd/northstar_knowledge.db`, verifying that the existing vision is unchanged and the derived vision is written to `.nerd/northstar.derived.json`.
