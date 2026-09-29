---
doc-class: north-star
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 07 — North Star Derivation — Non-Interactive Vision Synthesis & Role Transduction

This capability specification details the non-interactive extraction, classification, and synthesis of a project's North Star directly from repository documentation into `internal/northstar`, owned by Lane `I2b`.

---

## 1. The North Star Ingestion Deficit

In the current codebase, `nerd init` never initializes a North Star:
1. `internal/init/initializer.go:796-804` opens `northstar.NewStore` solely to run SQLite DDL migrations and immediately closes it.
2. `internal/init/initializer.go:1447` instructs the operator to manually run `/northstar`.
3. The only LLM drafting path lives in the Bubbletea interactive chat UI (`cmd/nerd/chat/northstar_llm.go:185-242`), which amputates documentation by truncating files exceeding 10,000 characters (`northstar_llm.go:201-203`).
4. In Phase 7b (`internal/init/strategic_knowledge.go:25, 68-120`), an LLM call synthesizes a vision string, but commits it exclusively as a generic knowledge atom in `.nerd/knowledge.db` (`strategic_knowledge.go:616-663`), completely bypassing the North Star store.

The North Star Derivation engine resolves this deficit by providing an autonomous, non-interactive library pipeline that classifies read-candidate documents, derives a complete relational North Star specification, writes directly to `internal/northstar` authority, and emits durable orientation artifacts.

---

## 2. Document Role Transduction

The transduction library (`internal/northstar/derive.go`) provides:

```go
func ClassifyDocuments(ctx context.Context, client perception.LLMClient, docs []Document) ([]types.Fact, error)
```

### Sensory Contracts & Invariants
1. **Lossless Paging**: Documents are read in their entirety. If a document exceeds the model request context budget, it is paged sequentially in chunks with a structured running summary carried across boundaries. Not a single character is discarded.
2. **Small Document Batching**: Small documentation files are concatenated into single request batches up to a configured byte ceiling (`orient.classify_batch_bytes`).
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
4. **Evidence Line Tracking**: For every role claim, the model returns an exact textual evidence line grounding its assessment.
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
func DraftVision(ctx context.Context, client perception.LLMClient, o *OrientationContext) (*northstar.WizardDocument, error)
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

---

## 5. Storage, Authority, and Persistence

The derivation flow integrates into `internal/init/phase_orient.go`:
1. **Single Authority Write**:
   Writes the derived `WizardDocument` directly to `.nerd/northstar_knowledge.db` via `Store.SaveVision` (`internal/northstar/store.go:197`). Calls `WriteVisionJSON` and `WriteVisionMangle` so that Guardian's `SyncVisionAuthority` (`internal/northstar/bridge.go:436-534`) completes cleanly with no-op status on first boot.
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
- Deletes the hardcoded 10,000-character truncation (`northstar_llm.go:201-203`).
- Deletes redundant inline prompt templates in favor of shared JIT prompt atoms (`internal/prompt/atoms/northstar/derive_classify.yaml`, `derive_vision.yaml`).
- Eliminates dual-path drift between CLI and chat North Star generation.

---

## 7. Verification Seams & Tests

1. `TestDerive_LosslessPaging`: Feeds a synthetic 60,000-character specification document to `ClassifyDocuments` with a scripted mock LLM client, asserting that every byte block is received and no truncation occurs.
2. `TestDerive_WizardDocumentLinkIntegrity`: Verifies that `DraftVision` outputs a `WizardDocument` whose `doc.ToVision()` execution prunes dangling persona, capability, and risk references without errors.
3. `TestPhaseOrient_PreservesExistingVision`: Runs `phase_orient` against a repository containing an existing `.nerd/northstar_knowledge.db`, verifying that the existing vision is unchanged and the derived vision is written to `.nerd/northstar.derived.json`.
