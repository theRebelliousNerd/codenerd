# Marathon init: optimizing the corpus for one model

> Corpus: `prompt` | Live owner: `internal/prompt/marathon`, `cmd/nerd/cmd_init_scan.go` | Added: 2026-08-17

## 1. What it is

`nerd init --marathon` runs after standard init and does one thing: it reads the
prompting documentation for the model that will actually serve this workspace,
rewrites the shipped atom corpus against that documentation, and emits the
results as **model-pinned variants** in the workspace overlay.

It is the consumer that makes provider/model pinning (`14-PROVIDER-MODEL-PINNING.md`)
worth having. Pinning gives atoms a way to be scoped to one model; the marathon
is what produces enough of them to matter.

```text
nerd init                      standard init: profile, agents, KB, corpus.db
  --marathon
    -> resolve serving identity        (hard fail if unknown)
    -> grounded research of THAT model (hard fail if unavailable / no docs)
    -> per-atom rewrite against the docs
    -> emit pinned, superseding variants into .nerd/prompts/corpus.db
```

## 2. Grounded or it does not run

Three hard failures, all before any atom is written:

| Failure | Sentinel | Why fatal |
|---|---|---|
| Serving provider/model unknown | `ErrNoModelIdentity` | Everything downstream is keyed on it: the research query, the pins, the checkpoint |
| Grounded search unavailable | `ErrNoGroundedSearch` | `grounded_web_search` is the only web route with a `safe_action` entry; `web_search`/`web_fetch` are routed but the constitution can never permit them |
| No documentation found | `ErrNoModelDocs` | Optimizing anyway writes unsourced rewrites into the agent's own instructions |

The third one carries the subtlety worth stating plainly. **Grounded search
answers happily from the model's own priors when retrieval finds nothing**, so a
non-empty response is *not* evidence that documentation was found. Citations
are. `ModelDocProfile.HasDocs` requires both prose and at least one citation, and
`NewOptimizer` re-checks it so no caller can route around the gate.

A degrading version of this feature — "research if you can, otherwise optimize
from general prompting knowledge" — was considered and rejected. Its output is
indistinguishable from the grounded version at the point of use, arrives with the
same authority, and lands in the file that tells the agent what to do. There is
no partial credit available here.

## 3. Additive, never destructive

`internal/prompt/atoms/**.yaml` is read-only to this package. Every result is a
**new** atom:

| Field | Value |
|---|---|
| `ID` | `<base-id>@<model-pin>`, e.g. `methodology/tdd@claude_opus_4` |
| `Providers` / `Models` | The pins, from `pinning.go` canonical tokens |
| `ConflictsWith` | `[<base-id>]` — one direction only |
| Everything else | Inherited verbatim from the base |

Selectors, category, priority and the mandatory flag are inherited **verbatim**
on purpose: the variant must be selected in exactly the situations the base
would have been, or it is not a replacement. The two exceptions are
`ContentConcise`/`ContentMin`, which are cleared — they were written for the old
text, and carrying them over would let the budget fall back to a rendering of an
atom that is no longer in the prompt.

The overlay lives in `.nerd/prompts/corpus.db`, which is workspace state that
`nerd init` regenerates from the shipped YAML. That is what makes the whole pass
revertible: `nerd init --force` discards it.

## 4. Supersession, and the kernel bug it exposed

For the overlay to mean anything, the variant must **replace** the base on the
model it was written for and **leave it alone** everywhere else.

The natural mechanism, exclusion groups, is inert: `jit_compiler.mg` — the live
path — never reads `atom_exclusion_group`. The only mechanism it does read is
`atom_conflicts`.

A probe against the real kernel showed that did not work either. Both atoms came
back in `selected_result`. The cause:

```prolog
prohibited(B) :- atom_conflicts(A, B), mandatory_selection(A).
```

derives `B` as prohibited, and **nothing downstream consults `prohibited` for a
mandatory atom**. It is joined only in `candidate_selection` (the vector/flesh
path) and the `atom_requires` pull-in. A mandatory atom reaches the output via
`mandatory_selection -> tentative -> final_valid`, and no step on that path reads
`prohibited`. The conflict was computed and discarded.

The direct fix — adding `!prohibited(Atom)` to `mandatory_selection` — is
unavailable: `prohibited` already depends on `mandatory_selection`, so that
closes a cycle through negation and the program stops being stratifiable. Hence
a separate predicate whose inputs are EDB plus `blocked_by_context`:

```prolog
mandatory_superseded(B) :-
    atom_conflicts(A, B),
    is_mandatory(A),
    !blocked_by_context(A).

mandatory_selection(Atom) :-
    is_mandatory(Atom),
    !blocked_by_context(Atom),
    !mandatory_superseded(Atom).
```

`!blocked_by_context(A)` is what makes supersession **conditional**, and that is
the property the overlay rests on: a variant blocked this compile (wrong model)
does not take its base down with it. Optimizing for one model therefore cannot
delete guidance for another.

The relation is **directional**. `atom_conflicts(A, B)` means "A supersedes B",
not "A and B are incompatible". A mutual pair cancels both atoms and deletes the
guidance outright; `TestSupersession_MutualConflictCancelsBoth` records that so
the consequence is documented rather than discovered in a prompt.

**Blast radius: zero on the shipped corpus.** No atom under
`internal/prompt/atoms` declares `conflicts_with`, so the rule derives nothing
until a producer opts in.

## 5. Resumability

A full pass is one LLM call per atom over 347 files. Progress is checkpointed to
`.nerd/prompts/marathon_checkpoint.json` after every atom, written atomically —
a torn checkpoint would restart a run hundreds of calls deep.

Two details make resume correct rather than merely fast:

- **Decisions are keyed by base content hash.** If a shipped atom changed since
  the checkpoint was written, its previous decision no longer applies and it is
  re-optimized.
- **A checkpoint for a different model is discarded, not resumed.** Its decisions
  were made against another model's documentation; resuming it would silently
  skip most of the corpus and report a near-instant success.

Failures are deliberately **not** checkpointed, so a transient rate limit is
retried on the next run rather than being recorded as "decided".

Already-pinned atoms are skipped by `selectBases`. Optimizing one would produce a
variant of a variant, pinned to the same model, superseding an atom that already
supersedes the original.

## 6. Flags

| Flag | Effect |
|---|---|
| `--marathon` | Run the pass after init |
| `--marathon-max-atoms N` | Bound one run (0 = whole corpus); checkpointed either way, so N runs of 50 equal one run of 50N |
| `--marathon-restart` | Discard the checkpoint and re-optimize everything |

## 7. Verification

| Test | Asserts |
|---|---|
| `internal/core/jit_supersession_test.go` | Real kernel: variant replaces base on its own model; **base survives when the variant is blocked**; mutual conflict cancels both; inert without conflicts |
| `internal/prompt/marathon/marathon_test.go` | All three hard-fail gates, including uncited prose being rejected; variant pinning, one-directional supersession, selector inheritance, base immutability; unchanged/empty/fenced response handling; already-pinned atoms skipped |

## 8. Known limits

- **The optimizer is the model optimizing its own instructions.** That is
  deliberate — it is the best available judge of how to phrase things for
  itself — but it is not an independent check. The system prompt is written
  conservatively ("returning the original is the correct answer far more often
  than not") and every variant records a rationale and citations, but nothing
  yet *verifies* that a rewrite preserved meaning. An eval pass comparing
  variant against base on golden scenarios is the obvious next step and is not
  built.
- **No staleness detection at compile time.** `OptimizedAtom.BaseContentHash`
  records which revision was optimized, but nothing warns at selection time that
  a variant is stale relative to an updated base.
- **Supersession is invisible in the manifest.** A reader sees the variant, not
  the fact that it displaced something.
