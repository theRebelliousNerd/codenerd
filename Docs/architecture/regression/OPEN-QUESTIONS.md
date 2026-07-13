# regression — Open Questions

> Last verified against codebase: **2026-07-13**  
> Real questions with decision impact. Not a generic filler list.

---

## Q1 — Who is the first-class host?

**Options:** CLI only · campaign assault stage · Nemesis gauntlet preflight · VirtualStore action · leave library-only  

**Impact:** Determines UX, safety envelope, and whether package comment stays.  

**Bias:** CLI-first is lowest risk (operator-initiated, no policy engine required).

---

## Q2 — Is vacuous success acceptable?

`RunBattery` on empty tasks returns `(nil, nil)`.  

**Should CI treat that as green?**  
If yes, a deleted task list silently passes. If no, hosts must special-case.

---

## Q3 — Should `RunBattery` return a non-nil error on task failure?

Today task failures are **Result-only**. That surprises Go callers used to `err != nil`.  

**Options:** keep dual-channel · return `ErrSuiteFailed` wrapping results · helper `AllPassed([]Result) error`.

---

## Q4 — Unix shell: login or not?

`bash -l` is convenient for developer PATH; hostile to hermetic CI.  

**Should the default change?** Migration risk for any future batteries relying on profile PATH.

---

## Q5 — One battery file or many?

`DefaultBatteryPath` implies a single suite.  

**Need** `battery.d/*.yaml` or named suites (`smoke`, `full`) for larger repos?

---

## Q6 — Relationship to campaign assault

Assault already orchestrates multi-stage shell/test work.  

**Is regression a subset, a plugin stage, or a parallel product?** Avoid two competing YAML formats without a story.

---

## Q7 — Should Nemesis own “regression” memory?

Docs/skills describe armory persistence of successful attacks as regression tests.  

**Does that stay in Nemesis, or could armory emit `Task`s into this package’s runner?** Keep boundaries clear.

---

## Q8 — Agent-writable batteries?

If agents can edit `.nerd/regression/battery.yaml` and then run it, that is **arbitrary code execution** with extra steps.  

**Policy:** agent may propose tasks; human or permitted action must approve run?

---

## Q9 — Version field semantics?

`version: 1` is loaded and ignored.  

**When does v2 happen**, and do we reject unknown versions fail-closed?

---

## Q10 — Keep or cull?

If after an explicit product decision no host will call this within N releases, **deprecate**. Until then, treat as dormant asset, not garbage.

---

## Resolved for this corpus

| Item | Resolution |
|------|------------|
| Does the package exist? | Yes — `internal/regression/` |
| Is it pre-implementation 0%? | **No** |
| Does any Go package import it today? | **No** (2026-07-13 grep) |
