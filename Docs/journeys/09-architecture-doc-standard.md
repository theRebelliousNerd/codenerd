# The standard the architecture corpus has to meet (R6)

> Rewritten 2026-09-21. The 2026-09-20 revision of this file asked for a corpus "SHORTER than
> what it replaced" and held up a three-file package (`Docs/architecture/diff`) as the model.
> That was an overcorrection and it is withdrawn. Steve, 2026-09-21: "rewrite every single arch
> doc so it doesn't just document what exists, but serves as a plan on what we are building
> toward -- the arch docs are the dream." The exemplar he named is a sibling repository's corpus,
> `C:\CodeProjects\Vectryx\docs\architecture\wormhole`. This file is the standard read off that
> exemplar, joined to the citation rules the first revision measured and got right.

## What the corpus is for

An architecture directory answers four questions about its package, and keeps them apart:

1. **What are we building toward?** The dream: the behaviour the finished package exhibits.
2. **What runs today?** The code as it is, every claim cited.
3. **What is the distance between the two?** Named gaps, each with an exit criterion.
4. **Why is it shaped this way?** Decisions, principles, risks -- with their evidence.

The July 2026 corpus mixed all four, unmarked, so no sentence in it could be trusted: a reader
could not tell a description of the code from a wish. The 2026-09-20 rewrites fixed that by
deleting questions 1, 3 and 4 and keeping only 2. Neither is the standard. The standard is all
four, **each file labelled with which one it answers**.

## The label every file carries

Every markdown file opens with front-matter. The vocabulary is the exemplar's:

    ---
    doc-class: north-star | shipped | shipped-with-future | governance | inventory | deep-dive | cross-cutting
    subsystem: <package directory name>
    implementation-status: planned | target-state | partial | shipped | accepted-not-implemented | not-applicable
    last-verified: <YYYY-MM-DD>
    verified-against: <commit>
    supersedes: []
    ---

`implementation-status` is the load-bearing field. It is what lets a vision document and a
current-state document sit in one directory without corrupting each other. A file whose status
is `shipped` makes only claims the code bears out today; a file whose status is `planned` or
`target-state` is allowed to describe what does not exist, because it says so at the top.

## The documents a package carries

Scale to the package. A one-file package does not need forty documents; a subsystem the north
star leans on needs more than three. But these slots are not optional for any package, because
each is one of the four questions:

| slot | doc-class | answers |
|---|---|---|
| `README.md` | governance | what this directory is, in a paragraph; points at the index |
| `00-INDEX.md` | governance | read order, one line per file saying what it answers and when to read it; a "grounded vs hypothesized" section naming which files are which |
| `01-VISION.md` | north-star | the behaviour the finished package exhibits and why it matters to codeNERD's north star (`CLAUDE.md`/`agents.md`, "North Star" and "The Vision"). May cite no code. |
| `02-CURRENT-STATE.md` | shipped | what is built and reachable today, file by file, every claim cited |
| `03-GAP-ANALYSIS.md` | shipped-with-future | the gap matrix (below) |
| `04-PRINCIPLES-AND-CONSTRAINTS.md` | governance | numbered principles any change to this package must respect, each with the code or ruling it comes from |
| `05-…` onward, one per capability | north-star or shipped-with-future | the spec: a capability in implementable detail -- data shapes, predicates, the Go/Mangle split, failure modes, tests that would prove it |
| `IMPLEMENTED_SPEC.md` | shipped | the authoritative record of shipped behaviour. On any disagreement with another file, this one wins. |
| `WIRING-AND-NOT-BUILT.md` | shipped | what is wired and reachable, what exists but nothing calls, what the design assumes that the code does not do -- with citations |
| `adr/ADR-NNN-<slug>.md` | governance | one decision each: context, decision, consequences, and a **witness** (below) |
| `RISK-REGISTER-AND-DECISION-LOG.md` | governance | risks with likelihood, consequence, and what would retire them |
| `OPEN-QUESTIONS.md` | governance | unresolved design questions and standing invariants ("tripwires") a future author must preserve |
| `TODO.md` | governance | the build queue: leaf work only, each item traceable to a gap ID |

### The gap matrix

`03-GAP-ANALYSIS.md` is the document that turns description into plan. One row per gap:

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|

- **Current state** is a `shipped` claim and is cited like one.
- **Target state** points at the spec file that details it.
- **Exit criteria** is something a command can check: a test that passes, a predicate that
  derives, a gate that reaches zero. "Improved", "robust" and "complete" are not exit criteria.
- IDs are stable (`GAP-<PKG>-01`). A closed gap stays in the table, marked closed with the
  commit that closed it; it is not deleted.

A gap with an ID, a dependency and a checkable exit is a unit of work a campaign can pick up.
That is the point of it: R7 (recursion) chooses its next item from these tables.

### An ADR's status is derived, never asserted

The exemplar's ADR-014 is adopted whole. Each ADR names a **witness** -- a test, a symbol, a
predicate, a file -- and its status follows from whether the witness resolves. An ADR that
reads "Accepted" over a type that a repository-wide search does not find is the defect that rule
exists for. If the decision's subject was never built, the status is
`accepted-not-implemented` and the ADR says so.

## The citation rules (unchanged from 2026-09-20, because they were measured)

These govern every claim in a `shipped`, `shipped-with-future`, `inventory` or `deep-dive`
file, and the "current state" half of anything else.

### 1. Every claim about the code cites a path, and the path resolves

Not "the compiler selects atoms by score" but "the compiler selects atoms by score
(`internal/prompt/selector.go`)". A sentence about behaviour with no citation is an opinion.

**The citation is repo-relative, and the symbol has to be there too.** Measured across the
first eight rewrites (2026-09-20): 1,223 citations, of which **856 were bare filenames**
(`registry.go:127`) rather than repo-relative paths. A bare filename is not a citation -- this
tree has four `registry.go` and three `compiler.go`, so the reader cannot tell which file is
meant, and neither can the checker. Write `internal/tools/registry.go:127`.

Worse than unqualified is **abbreviated**. One 2026-09-20 rewrite cites `registry.go` and
`compiler.go` throughout a package that contains neither; it means
`internal/autopoiesis/runtime_registry.go` and `internal/autopoiesis/tool_compiler.go`, with the
distinguishing half of each name dropped. A shortened filename reads exactly like a real one
and is the single hardest citation defect to see.

Resolving the file is the floor, not the bar. In that same document `RuntimeRegistry.Register`
and `Restore` are real symbols cited at lines 127 and 207; they are at 39 and 77. So the check
is: open the file, find the symbol, confirm the line. A path-resolver sees none of this, and
all of it survived a `/done` verdict.

**Do not quote a fabricated identifier in this file.** An earlier revision named two
non-existent functions verbatim as the example. Every run is briefed to read this standard
first, and it is injected whole, so within three minutes the invented names were in the context
of every subsequent run, indistinguishable by string match from a symbol the repo actually has.
Describe the defect; do not spell the artifact. A document the harness injects is not a place
to write things that are not true.

### 1a. Every file you name carries a symbol and a line, or you do not name it

Symbol-level grading of the first fourteen rewrites: 580 citations checked, **468 correct**
(80.7%), and the errors concentrate in one shape. `core/README.md` opens with a file-map table
whose rows group files by theme. The rows that name a symbol and a line are precise. The rows
that list filenames with only a phrase beside them name **13 files that do not exist** out of
24 cited in the package, five of them consecutive in a single row.

Asked for a symbol and a line, a run goes and looks. Asked to characterise an area, it produces
a plausible file list, and plausible is exactly what a Go package's filenames are. A grouping
row is prose wearing a table's clothes. So: no file is named anywhere in these documents without
at least one symbol and line drawn from inside it. A file you cannot cite a symbol from is a
file you did not open.

### 2. The plan layer is held to a different rule, not a weaker one

A `north-star`, `planned` or `target-state` file may describe what does not exist. It must:

- say so in its front-matter, and never state a planned behaviour in the present tense as
  though the code does it;
- anchor to what does exist -- the seam in today's code where the capability would attach,
  cited under rule 1;
- trace to codeNERD's north star: which sentence of the vision this capability serves. A spec
  that serves none of it is a spec for a different project;
- end in something buildable: the gap IDs it closes and their exit criteria.

Invention is wanted here. Vagueness is not: "the kernel should be smarter about context" is not
a spec; a named predicate, the facts that feed it, the Go seam that asserts them and the test
that would fail today is.

### 3. One question per file, and no two files answer the same one

Two templates were overlaid in July and neither retired, so within one package two files claimed
the same slot. This is the no-shims rule applied to prose: when a document is replaced the old
one is deleted, not left pointing at its successor. Nothing in the corpus is a forwarding stub,
and nothing anywhere in the repository -- markdown, Go comments, test messages, `corpus.toml`,
`portfolio.toml` -- still points at a file that was removed.

### 4. Written from the code, never from the previous docs -- for the shipped layer

The July corpus was generated one package at a time by a weak model, and its second generation
inherited the first's claims rather than re-deriving them. A current-state document that reads
the old file first launders its errors forward. Read the Go.

The old documents' *ambitions* are a different matter. Where an old VISION or spec states an
intent for the package, it is an input to the plan layer -- to be judged against the north star
and the code, kept where it is right, and never treated as evidence of anything being built.

### 5. Cover the packages that exist

50 of 91 Go packages under `internal/` and `cmd/` had no documentation on 2026-09-20. A corpus
that documents 45% of the tree and says nothing about the omission implies the rest does not
exist.

### 6. Cross-cutting truths live in exactly one place

The kernel's evaluation model, the Mangle engine's verified behaviours, the safety gate's
contract: each is one document (`doc-class: cross-cutting`), cited from the packages that
depend on it. Repeating it per package is how forty copies drift into forty different claims.

## What "impresses" means, concretely

A reviewer picks any package at random, reads its documentation, then reads the package. The
corpus passes when, for every sample:

1. every path cited exists and every symbol named is in the file it is attributed to;
2. nothing a `shipped` document claims is contradicted by the code;
3. nothing load-bearing in the package is missing -- a reader who had only the documents would
   not be surprised by anything important in the code;
4. the documents say which parts do not run, and are right about that too;
5. a reader who had only the documents would know **what to build next and how they would know
   it was done**: the vision is specific to this package, the gaps have IDs and checkable exit
   criteria, and the specs are detailed enough to start work from;
6. every ADR's status matches what its witness shows;
7. no file leaves the reader unsure whether it describes the present or the plan.

The deterministic half is machine-checked: citations resolve and symbols sit at their lines
(`scripts/r6_symbolcheck.py`, `scripts/doc_citation_check.py` -- `scripts/` is gitignored; this
line is their record), every file has valid front-matter, every required slot is present, every
gap row has an exit criterion, every ADR has a witness that was looked up. The judgement half
-- is the claim true, is the dream worth building, is anything important missing -- is the
reviewer's. A run cannot pass by writing confident prose, and it cannot pass by writing
citations around nothing.

## Status of the corpus against this standard (2026-09-21)

- 16 package directories were rewritten on 2026-09-20 to the withdrawn three-file model. Their
  current-state claims grade well at symbol level (699 checked, 576 correct; three packages
  perfect) and they carry no vision, gap matrix, specs or ADRs. They are a good
  `02-CURRENT-STATE` / `WIRING-AND-NOT-BUILT` layer and nothing else, and are redone.
- 25 package directories still hold the July corpus.
- 50 packages have no directory.
