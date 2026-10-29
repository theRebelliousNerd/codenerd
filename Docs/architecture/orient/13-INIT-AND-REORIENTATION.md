---
doc-class: shipped-with-future
subsystem: orient
implementation-status: partial
last-verified: 2026-09-29
verified-against: working-tree-C2a
supersedes: []
---

# Init and re-orientation contract

GAP-ORIENT-23 replaces the duplicate init sensors with one `internal/orient`
engine. After setup, migrations, directories and the membership scan, init
asserts history, documents and ecosystem measurements, derives its read set,
calls northstar role transduction, then drafts a vision. Profile, agent
creation, prompt ingestion and knowledge bases follow this phase. Strategic
knowledge retains its existing storage categories, projected from the stored
vision and orientation evidence instead of a second document relevance pass.
An existing vision remains authoritative when a new draft is recorded beside
it. Init has no run deadline: its timeout bounds one provider request, and
caller cancellation is checked between phases.

An orientation snapshot records its measured Git HEAD and document digests.
Mangle derives staleness from current HEAD and current tracked document
digests. The refresh driver replaces affected measurements, removes obsolete
role claims, recomputes judgments, and writes the new snapshot atomically.
Operator `answers.json` is never rewritten. Fast-forward history uses a single
streaming delta; a rewritten history requires a full history measurement.
Changed document bodies are read again; unchanged bodies and classifications
are retained. A changed body requiring model transduction remains visibly
pending until an LLM-capable orientation pass supplies new claims.
Body digests, subtree witnesses and embedding outcomes are replaced with the
document census; old C3 measurement rows cannot remain current alongside new
ones. A refresh without an embedding client marks affected vectors unavailable.

Boot loads the generated orientation projection beside northstar facts.
The incremental scan must call Refresh and apply its exact fact delta through RefreshResult.Apply; its hook is outside C2a's
owned files. No orientation command or alternate engine loader is introduced.

Acceptance commands (authored, not run by this author-only lane):

- `go test -count=1 ./internal/init -run 'TestOrientation(PhaseOrder|InitArtifacts)'`
- `go test -count=1 ./internal/init -run TestOrientationCommitRefreshAnswersSurvive`
- `go test -count=1 ./internal/orient -run 'TestRefresh'`
- `go test -count=1 ./internal/config -run TestOrientEcosystemThresholds`
- `go test -count=1 ./internal/core/defaults/...`
- `go test -count=1 ./internal/init/... ./internal/orient/... ./internal/config/...`
- `go build -tags sqlite_vec ./...`

Source seams: `Initialize` (`internal/init/initializer.go:469`) invokes
`runOrientation` (`internal/init/phase_orient.go:38`); `Measure` (`internal/orient/snapshot.go:77`) collects the shared census; `Refresh` (`internal/orient/snapshot.go:281`) derives freshness and replaces measurements.
These are source witnesses. Runtime exits remain unexecuted.
