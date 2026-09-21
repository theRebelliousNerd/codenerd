# The standard the architecture corpus has to meet (R6)

The ladder's R6 says codeNERD rewrites `Docs/architecture` "to a standard that impresses."
That is not a criterion until the standard is written down, so this is it: nine rules, each
one answering a defect measured in the corpus on 2026-09-20, and each one either checkable by
a script or checkable by a reader who knows what to look for.

The measurements this is built on, all from the current corpus:

| | |
|---|---|
| markdown files | 929 (~4.8 MB) across 41 directories |
| real Go packages under `internal/` + `cmd/` | 91 |
| **packages with no documentation at all** | **50 of 91** |
| files that are only a forwarding pointer | 189 (20%) |
| **files citing no repo path at all** | **550 of 929 (59%)** |
| path citations | 3,576, of which 96.1% resolve |
| cited line numbers past end of file | 0 |

The last two lines matter more than they look. **The corpus is not mostly wrong -- it is mostly
unfalsifiable.** Its citations are nearly all correct; there are just far too few of them, and
the majority of its files make no claim a reader could check. That is why it reads as
substantial and why citing it has repeatedly turned out to be citing nothing.

## The nine rules

### 1. Every claim about the code cites a path, and the path resolves

Not "the compiler selects atoms by score" but "the compiler selects atoms by score
(`internal/prompt/selector.go`)". A sentence about behaviour with no citation is an opinion.

**The citation is repo-relative, and the symbol has to be there too.** Measured across the
first eight rewrites (2026-09-20): 1,223 citations, of which **856 were bare filenames**
(`registry.go:127`) rather than repo-relative paths. A bare filename is not a citation — this
tree has four `registry.go` and three `compiler.go`, so the reader cannot tell which file is
meant, and neither can the checker. Write `internal/tools/registry.go:127`.

Resolving the file is the floor, not the bar. `autopoiesis/WIRING-AND-NOT-BUILT.md` cites
"dual validators (`ValidateGoCode` vs `validateGoCodeOffline`, `compiler.go:531` vs `:594`)";
neither symbol exists anywhere in the repo and `internal/autopoiesis/` has no `compiler.go`.
In the same file `RuntimeRegistry.Register` is real but lives in `runtime_registry.go`, cited
as `registry.go:127`. A plausible filename is the exact shape a fabricated citation takes, so
the check is: open the file, find the symbol. Both failures are invisible to a path-resolver
and both survived a `/done` verdict.

Checkable: `scripts/doc_citation_check.py` resolves every `internal/…` and `cmd/…` path and
every `:line` against the working tree. Target: 100% resolve, 0 line numbers past EOF. The
corpus is at 96.1% and 0 today, so this rule is nearly met already and must not regress.

The largest current violation is instructive: `cmd/nerd/chat/session_boot.go` is cited 65 times
and was deleted by `5bcd12f8` ("delete the dead legacy boot path"). The docs describe a boot
path the repo removed on purpose. One citation is `internal/foo.go`.

### 2. A file that cites nothing must justify its existence in its first paragraph

A VISION file may legitimately cite no code. Everything else is suspect, and two slots are
outright violations today:

| slot | files citing code | verdict |
|---|---|---|
| `IMPLEMENTED_SPEC.md` | 98% | the model to copy |
| `08-WIRING-AND-INTEGRATION.md` | 100% | the model to copy |
| `05-INTERNAL-ARCHITECTURE.md` | **28%** | 6 KB of prose about code it never points at |
| `09-SAFETY-AND-INVARIANTS.md` | **28%** | an invariant nobody can locate cannot be checked |
| `12-FAILURE-MODES.md` | 13% | a failure mode that names no site is not a failure mode |

### 3. One question per file, and no two files answer the same one

`01-VISION.md` appears in 39 directories and `01-DOMAIN-MODEL.md` in 25; `12-FAILURE-MODES.md`
in 39 and `08-FAILURE-MODES.md` in 25. Two templates were overlaid and neither retired, so
within one package two files claim the same slot and a reader cannot tell which is live.

This is the no-shims rule applied to prose. When a document is replaced the old one is deleted,
not left pointing at its successor.

### 4. Say what is NOT built

The single most useful thing these documents can contain, and the thing the current corpus
buries. A package doc that describes the design without saying which parts of it run is how
`Docs/architecture` became untrustworthy: every statement was aspirational and none was marked
as such.

Every package doc carries a section naming, with citations: what is wired and reachable, what
exists but nothing calls, and what the design assumes that the code does not do. The starved
and undeclared baselines under `internal/core/defaults/testdata/` are the model for the tone --
they are lists of things that do not work, kept deliberately, with the reason.

### 5. Every claim is dated and pinned to a revision

`> Verified 2026-09-20 against a1b2c3d` at the top of each file. A claim with no date cannot be
audited for staleness, and a corpus whose staleness cannot be measured decays silently. The
existing files carry "Last verified: 2026-07-13" -- the form is right, the practice lapsed.

### 6. Written from the code, never from the previous docs

The corpus's own history is the argument: it was generated one package at a time by a weak
model, and the second generation inherited the first's claims rather than re-deriving them.
A rewrite that reads the old file first will launder its errors into the new one.

### 7. Cover the packages that exist

50 of 91 packages have no documentation. A corpus that documents 45% of the tree and says
nothing about the omission implies the rest does not exist.

### 8. Cross-cutting truths live in exactly one place

The kernel's evaluation model, the Mangle engine's verified behaviours, the safety gate's
contract: each is one document, cited from the packages that depend on it. Repeating it per
package is how 41 copies drift into 41 different claims.

### 9. Nothing in it is a forwarding stub

189 files today are a heading and a pointer. They go, and nothing links to what is gone.

## What "impresses" means, concretely

A reviewer picks any package at random, reads its documentation, then reads the package. The
corpus passes when, for every sample:

1. every path cited exists and every symbol named is in the file it is attributed to;
2. nothing the document claims is contradicted by the code;
3. nothing load-bearing in the package is missing from the document -- a reader who had only
   the document would not be surprised by anything important in the code;
4. the document says which parts do not run, and is right about that too;
5. it is shorter than what it replaced.

Rules 1 and 9 and the slot half of rule 3 are machine-checked. Rules 2 and 3 are the reviewer's.
That division is deliberate: the deterministic half means a run cannot pass by writing
confident prose, and the judgement half means it cannot pass by writing citations around
nothing.

### Writing the new files is half the task; removing the old ones is the other half

Measured over the first eight rewrites: six finished, two did not. `broker` wrote four correct
files and left all twenty originals on disk; `cli` wrote its three and kept two old ones. In
both cases every hard citation error in the package sits in a file the run did **not** write --
`01-VISION.md`, `02-CURRENT-STATE.md`, `TODO.md`, `IMPLEMENTED_SPEC.md`,
`05-COMMAND-ARCHITECTURE.md`. The new documents were sound; the residue was not, and a reader
landing in the directory cannot tell the two apart.

So a package is not done while a file the rewrite superseded is still in its directory. The run
ends by listing the directory and naming every file in it as either one it wrote or one it
deleted -- `list_files` and `delete_file` both exist, under those names. A report that says the
old files remain and hands the deletion to someone else has not finished; it has produced two
corpora where there was one.

## Why this is not yet briefed as a rung

R6 is the largest rung and the corpus is 929 files. It is attempted after the smaller removal
sweep (R5) lands, because that sweep is rule 9 alone and it has already failed four times --
most recently by deleting three real documents while removing 85 correct ones. A run that
cannot yet tell a stub from a document by reading it is not ready to be told to rewrite 929 of
them.
