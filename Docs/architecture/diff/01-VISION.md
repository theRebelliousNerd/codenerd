---
doc-class: north-star
subsystem: diff
implementation-status: planned
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# Diff — Vision (north-star)

This document describes the finished behaviour of the diff capability. It is
a plan, not a description of what runs today. Nothing below is claimed as
shipped; the shipped record lives in the current-state and implemented-spec
documents. This file may cite no code.

## The finished behaviour

When the diff capability is finished, every change the agent makes is shown
exactly where it is needed, in the cheapest form that still carries the
meaning:

1. **Every repair round would show its own edits.** After a turn writes files,
   the turn summary would include a compact unified view of what changed —
   added lines, removed lines, and enough surrounding context to orient the
   reader, nothing more. An empty change would show nothing at all.
2. **The interactive view and the agent loop would agree.** A human reviewing
   changes in the terminal interface and the agent reviewing its own edits
   mid-loop would see the same line groupings, the same context widths, and
   the same word-level emphasis inside changed lines. Two views of one change
   would never disagree about what changed.
3. **Large or hostile inputs would stay bounded.** Very large comparisons
   would complete within a fixed time budget. Context width would be clamped
   to a sane range. Content that is not text would be flagged as binary and
   shown as a single note rather than expanded into meaningless line noise.
4. **Repeated comparisons would be cheap and safe.** Identical comparisons
   requested twice would be served from a bounded in-process cache that never
   grows without limit and never lets one caller mutate another caller's
   result. Trust in cached content would be explicit and opt-in: display may
   trust the cache, while anything that applies a change would verify first.
5. **Line structure and word emphasis would stay separate.** Line grouping and
   hunk framing would belong to the line layer; fine-grained emphasis inside
   a changed line pair would be a separate word-span layer that never leaks
   third-party types to callers. Rendering choices such as header formatting
   and markers would belong to the callers, never to the diff engine itself.

## Why this matters to codeNERD's north star

This capability serves one sentence of the project vision (agents.md,
"The Vision", Steve 2026-09-18):

> Tools exist for exactly three things: condense the search space, reduce the
> turns to complete the task, and offload cognition to deterministic code
> (a change with a blast radius is carried out by the tool, not by the
> LLM hand-editing).

A finished diff capability serves the second and third clauses directly.
Showing a repair round its own edits removes re-diagnosis turns that would
otherwise be spent rediscovering what just changed. Computing those views in
deterministic code rather than by asking the model to describe its edits
keeps a blast-radius operation — stating what changed in the tree — out of
the model's discretion. No other vision sentence governs this package.

## What would prove it is done

The vision is buildable through the following gaps. Each gap's exit criterion
is machine-checkable: a test that passes or a gate that reaches zero.

| Gap ID | Capability | Exit criteria |
|---|---|---|
| GAP-DIFF-01 | Turn summaries always carry own-edit views | A test that runs a multi-file repair round and asserts every touched path appears once in the summary, and that a no-change round renders nothing, passes. |
| GAP-DIFF-02 | Interactive view and loop agree on one change | A test that renders the same change through both consumers and asserts identical hunks, context widths, and word spans, passes. |
| GAP-DIFF-03 | Bounded cost under large inputs | A benchmark with a fixed time budget and clamped context passes without heap growth across repeated runs. |
| GAP-DIFF-04 | Binary content short-circuits safely | A test feeding non-text content on either side asserts a binary flag with zero hunks, passes. |
| GAP-DIFF-05 | Cache isolation with opt-in trust | Concurrent compute-while-clear and mutation-after-fetch tests assert deep-copy isolation and zero counter loss, pass. |

When all five exit criteria hold, the finished behaviour described above holds.
