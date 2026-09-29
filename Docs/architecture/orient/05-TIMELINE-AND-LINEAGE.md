---
doc-class: north-star
subsystem: orient
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 05 — Timeline and Lineage — Development Chronology & Document Evolution

This capability specification details the design of codeNERD's development timeline, document evolution, and read-candidate attention selection subsystem, owned by Lane `I2a`.

---

## 1. The Problem Space and Purpose

Current initialization logic attempts to identify strategic documentation using hardcoded priority tables (`internal/init/strategic_knowledge.go:246-260`) and batch LLM relevance queries (`strategic_knowledge.go:414-450`). This approach exhibits three critical failure modes:
1. **Blindness to Evolution**: It cannot distinguish a 2-year-old foundational architecture document from a 2-day-old temporary patch note.
2. **Blindness to Bursts**: It fails to recognize that a comprehensive set of documents committed simultaneously on a single day often represents the primary architectural thesis of the project.
3. **Cognitive Overload**: In a repository with thousands of documentation files, issuing batch LLM relevance calls is cost-prohibitive, while naive directory scanning drowns the model in ephemera.

The Timeline and Lineage engine resolves this by deriving a chronological fact graph from git commit history, document linkage networks, and semantic embedding clusters, computing exact read candidates before any LLM is invoked.

---

## 2. Go Sensory Pipeline: Whole-History Git Streaming

The Go sensor (`internal/orient/history.go`) executes a single, streaming git command over the repository:

```bash
git log --name-status -M --format=@@@COMMIT@@@%H|%ct
```

### Sensory Contracts & Invariants
1. **Zero Repeated Git Calls**: The scanner executes one streaming process for the entire repository, reading stdout line by line. It never invokes git on a per-file basis.
2. **Rename Tracking (`-M`)**: File renames and moves are tracked so that an evolving specification retains its complete historical lineage across path changes.
3. **Locking Isolation**: The process runs with `GIT_OPTIONAL_LOCKS=0` to ensure non-blocking read-only safety.
4. **Shallow Clone Safeguard**: The engine executes `git rev-parse --is-shallow-repository`. If the repository is shallow, it asserts `repo_span(..., /yes)`. Mangle policies recognize this flag and refuse to derive development eras from an amputated commit graph.
5. **Committer Timestamp Truth**: Chronology is computed strictly from the committer epoch timestamp (`%ct`). Filesystem `mtime` is never consulted.

### Ground EDB Predicates Emitted by Go
- `repo_file_history(Path, FirstUnix, LastUnix, Commits, ActiveDays)`:
  - `Path`: string, repo-relative path.
  - `FirstUnix`, `LastUnix`: int64 epoch seconds of first and last commit.
  - `Commits`: total number of commits touching this file.
  - `ActiveDays`: distinct UTC calendar days on which commits touched this file.
- `repo_month(MonthIndex, Label, Commits, FilesAdded, DocsAdded)`:
  - `MonthIndex`: 0-indexed month from repository inception.
  - `Label`: format `"YYYY-MM"`.
  - `Commits`: commit count within the month.
  - `FilesAdded`, `DocsAdded`: new files and documentation files introduced.
- `repo_span(FirstUnix, LastUnix, TotalCommits, Shallow)`:
  - Overall repository commit boundaries and shallow clone status (`/yes` or `/no`).
- `doc_file(Path, Dir, Bytes, Headings)`:
  - Discovered markdown, RST, and text documentation files with size and heading counts.
- `doc_link(FromPath, ToPath)`:
  - Resolved relative cross-references between documentation files.
- `doc_similar(A, B, Permille)`:
  - Semantic vector similarity between document text centroids, scaled to integers $0..1000$, emitted only for pairs above the configured similarity floor.

---

## 3. Semantic Embedding Similarity and Centroids

To establish semantic relationship edges without saturating memory, `internal/orient/docs.go`:
1. Chunks each document into bounded byte slices (configured by `orient.embedding_chunk_bytes`).
2. Computes chunk embeddings via the configured local embedding engine (`internal/embedding`).
3. Computes the document's central embedding vector (centroid).
4. Calculates top-$k$ nearest neighbors for each document above a configurable per-mille threshold (e.g. 750 / 1000).
5. Caches vectors under `.nerd/cache/embeddings/` keyed by SHA-256 content hashes to avoid re-embedding unchanged documents during re-orientation.

---

## 4. Deductive Mangle Policy: Lineage, Eras, and Attention

Policy files `internal/orient/timeline.mg` and `internal/orient/lineage.mg` evaluate the asserted facts to fixpoint.

### Derived IDB Predicates

1. **Development Eras** (`repo_era/4`):
   Partitions repository history into discrete temporal eras classified as `/wave` (sustained development activity exceeding commit and file creation thresholds) or `/lull` (periods of low activity).
2. **Document Generations** (`doc_generation/2`):
   Assigns generation labels based on the document's `FirstUnix` relative to repository eras:
   - `/origin`: introduced during the foundational inception era.
   - `/early`, `/middle`, `/recent`: corresponding to subsequent developmental phases.
3. **Burst Identification** (`doc_burst/1`):
   Identifies documents created or heavily revised during a compact burst of commits (`Commits >= min_burst_commits` with `ActiveDays <= max_burst_days`).
4. **Evolution and Supersession** (`doc_evolved_into/2`, `doc_superseded/2`):
   - `doc_evolved_into(Old, New)`: derives when two documents share high semantic similarity (`doc_similar`), `New` was first committed after `Old`, and shared structural themes or links exist.
   - `doc_superseded(Old, By)`: derives when `Old` evolved into `By`, `By` is active in the recent era (`doc_live`), and `Old` has received no commits across recent eras.
5. **Living Documents** (`doc_live/1`):
   Derives when a document was committed within the recent era or occupies a central position in the cross-link graph, and is not superseded.
6. **Origin Sources** (`origin_source/2`):
   Derives foundational origin documents that explain the core architectural lineage of the system.
7. **Read-Candidate Attention Selection** (`orient_read_candidate/2`):
   The central attention mechanism. Selects the bounded set of documents that the LLM must read in full:
   - All foundational `/origin` documents.
   - Central hubs of the document link graph (high in-degree and out-degree).
   - Embedding cluster representatives (documents closest to the cluster centroid).
   - High-density commit burst documents.
   - Root and subtree instruction files (`agent_source` with Kind `/instructions`).
   - Active tips of document evolution chains (`doc_live` successors).
   The derivation is strictly bounded by `config_param(/orient_read_candidate_budget, Budget)`.

---

## 5. Failure Modes and Recovery

| Failure Mode | Root Cause | Engine Behavior & Mitigation |
|---|---|---|
| Shallow git clone | User executed `git clone --depth 1` | `repo_span(..., /yes)` asserted. Policy disables historical era derivation and treats all files as recent, emitting a diagnostic warning. |
| Inception burst | Repository initialized via massive code dump | High initial commit count marked as Era 0 origin burst; generation correctly assigns `/origin` to the initial import. |
| Volatile mtimes | Clone or checkout touched file timestamps | Ignored entirely; committer timestamps from git log govern all chronological sorting. |
| Document graph cycle | Bidirectional links between two specifications | Mangle stratified evaluation reaches clean fixpoint over cyclic relations without infinite loops. |
| Oversized doc set | Monorepo containing 20,000 generated markdown files | Structural attention prunes unlinked, peripheral documents; `orient_read_candidate` caps total candidates at configured budget. |

---

## 6. Verification Seams & Tests

The timeline and lineage capability is verified by deterministic integration tests:
1. `TestGitHistoryStreaming_ControlledRepo`: Constructs a temporary git repository with controlled `GIT_COMMITTER_DATE` values, verifying correct extraction of `repo_file_history`, rename tracking, and monthly aggregation.
2. `TestMangleLineage_EvolutionAndSupersession`: Asserts small synthetic fact fixtures to verify that `doc_evolved_into` and `doc_superseded` correctly derive without circularity.
3. `TestAttentionSelection_BoundsRespected`: Seeds 500 document facts and asserts that `orient_read_candidate` never exceeds the configured ceiling and never consults file-name priority lists.
