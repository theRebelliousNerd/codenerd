#!/usr/bin/env python3
"""Generate FULL architecture corpora 1:1 with every internal/* package root."""
from __future__ import annotations

import json
import re
import sys
from datetime import date
from pathlib import Path

TODAY = date.today().isoformat()

ROLES = {
    "articulation": "Atoms→NL emission, Piggyback protocol, prompt assembly bridge",
    "autopoiesis": "Self-improvement: Ouroboros tool generation, SafetyChecker, Thunderdome",
    "browser": "Browser automation / Rod session management and honeypot surfaces",
    "build": "Build-time environment helpers (CGO/sqlite flags, build env)",
    "campaign": "Multi-phase goal orchestration, decomposition, context paging",
    "config": "Configuration loading, engines, limits, user and memory config",
    "context": "Context activation, scoring, and window management",
    "core": "Mangle kernel, VirtualStore, Dreamer, facts, API scheduler, shard manager plumbing",
    "diff": "Diff utilities for code change analysis",
    "embedding": "Embedding engines (including Ollama) and vector generation",
    "features": "Feature flags and feature configuration defaults",
    "init": "Workspace/project initialization and scanning",
    "jit": "JIT-related config/types supporting prompt compilation",
    "logging": "Categorized logging system for debug/diagnostics",
    "mangle": "Mangle engine bindings, differential evaluation, generation feedback",
    "mcp": "MCP server/client integration surfaces",
    "northstar": "North-star goal tracking and alignment helpers",
    "observability": "Flight recorder and runtime metrics",
    "perception": "NL→atoms transduction, semantic classification, LLM clients",
    "persist": "Persistence helpers bridging stores and runtime",
    "prompt": "JIT prompt compiler, atoms, selector, budget, resolver",
    "regression": "Regression harness utilities",
    "retrieval": "Retrieval / knowledge lookup helpers",
    "session": "Session execution loop and clean executor",
    "shards": "Domain and system shard implementations + registration",
    "sqlpragmas": "SQLite pragma helpers for safe DB open",
    "store": "Memory tiers and durable store implementations",
    "system": "System factory / boot wiring helpers",
    "tactile": "Tactile routing / action-to-tool surfaces",
    "testing": "Internal test helpers and harness utilities",
    "tools": "Tool registry and research/tool integrations",
    "transparency": "Transparency event bus / glass-box observability",
    "types": "Shared type definitions used across packages",
    "usage": "Usage / token accounting helpers",
    "ux": "UX helpers for CLI presentation",
    "verification": "Verification utilities for agent outputs",
    "world": "World model: filesystem topology, AST/symbol projection",
}


def find_root() -> Path:
    cwd = Path.cwd()
    for p in [cwd, *cwd.parents]:
        if (p / "go.mod").exists() and (p / "internal").is_dir():
            return p
    raise SystemExit("could not find codeNERD root (go.mod + internal/)")


def list_go(root: Path, src: Path):
    srcs, tests = [], []
    if not src.exists():
        return srcs, tests
    for p in src.rglob("*.go"):
        rel = p.relative_to(root).as_posix()
        try:
            lines = sum(1 for _ in open(p, encoding="utf-8", errors="replace"))
        except OSError:
            lines = 0
        (tests if p.name.endswith("_test.go") else srcs).append((rel, lines))
    srcs.sort()
    tests.sort()
    return srcs, tests


def list_mg(root: Path, src: Path):
    out = []
    if not src.exists():
        return out
    for p in src.rglob("*.mg"):
        rel = p.relative_to(root).as_posix()
        try:
            lines = sum(1 for _ in open(p, encoding="utf-8", errors="replace"))
        except OSError:
            lines = 0
        out.append((rel, lines))
    out.sort()
    return out


def top_files(files, n=20):
    return sorted(files, key=lambda x: -x[1])[:n]


def extract_types(root: Path, src: Path, limit=40):
    found = []
    if not src.exists():
        return found
    pat = re.compile(r"^type\s+([A-Z][A-Za-z0-9_]*)\s+")
    for p in sorted(src.rglob("*.go")):
        if p.name.endswith("_test.go"):
            continue
        try:
            text = p.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        for i, line in enumerate(text.splitlines(), 1):
            m = pat.match(line)
            if m:
                found.append((m.group(1), f"{p.relative_to(root).as_posix()}:{i}"))
                if len(found) >= limit:
                    return found
    return found


def extract_funcs(root: Path, src: Path, limit=30):
    found = []
    if not src.exists():
        return found
    pat = re.compile(r"^func\s+(?:\([^)]+\)\s+)?([A-Z][A-Za-z0-9_]*)\s*\(")
    for p in sorted(src.rglob("*.go")):
        if p.name.endswith("_test.go"):
            continue
        try:
            text = p.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        for i, line in enumerate(text.splitlines(), 1):
            m = pat.match(line)
            if m:
                found.append((m.group(1), f"{p.relative_to(root).as_posix()}:{i}"))
                if len(found) >= limit:
                    return found
    return found


def tier_for(name, src_count, mg_count):
    if name in (
        "core",
        "mangle",
        "perception",
        "prompt",
        "shards",
        "campaign",
        "autopoiesis",
        "world",
        "store",
    ):
        return 3
    if src_count >= 20 or mg_count > 0:
        return 3
    return 2


def pct(src_count, test_count):
    if src_count == 0:
        return 0
    base = 70
    if test_count >= max(1, src_count // 2):
        base = 85
    if test_count >= src_count:
        base = 90
    if src_count > 50:
        base = min(base, 88)
    return base


def write(path: Path, content: str):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content.replace("\r\n", "\n"), encoding="utf-8")


def gen_corpus(root: Path, out: Path, name: str) -> dict:
    rel = f"internal/{name}"
    src = root / rel
    role = ROLES.get(name, f"codeNERD package `internal/{name}` — see source inventory")
    srcs, tests = list_go(root, src)
    mgs = list_mg(root, src)
    types = extract_types(root, src)
    funcs = extract_funcs(root, src)
    completion = pct(len(srcs), len(tests))
    tier = tier_for(name, len(srcs), len(mgs))
    top = top_files(srcs, 20)
    top_tests = top_files(tests, 10)
    status = "Realized — living code" if srcs else "Present — empty/minimal package"

    banner = (
        f"> Last verified against codebase: {TODAY}\n"
        f"> Status: Living Reference Document — **code-grounded full corpus**\n"
        f"> Mode: 1:1 with `internal/{name}/` (complete internal coverage)\n"
        f"> **Implementation: `{rel}/` — {len(srcs)} non-test .go, {len(tests)} tests, {len(mgs)} .mg**\n"
    )

    inv_rows = "\n".join(f"| `{p}` | {n} | source |" for p, n in top) or "| — | 0 | missing |"
    all_src_rows = "\n".join(f"| `{p}` | {n} |" for p, n in srcs[:200]) or "| — | 0 |"
    if len(srcs) > 200:
        all_src_rows += f"\n| … | {len(srcs) - 200} more files omitted from table |"
    type_rows = "\n".join(f"| `{t}` | `{loc}` |" for t, loc in types) or "| — | — |"
    func_rows = "\n".join(f"| `{t}` | `{loc}` |" for t, loc in funcs) or "| — | — |"
    mg_rows = "\n".join(f"| `{p}` | {n} |" for p, n in top_files(mgs, 15)) or "| — | 0 |"
    test_rows = "\n".join(f"| `{p}` | {n} |" for p, n in top_tests) or "| — | 0 |"

    base = out / name

    write(
        base / "README.md",
        f"""# {name} — Architecture Corpus

{banner}

## Role

{role}

## Source location

- Primary: `{rel}/` (**1:1 package root**)
- Non-test Go files: **{len(srcs)}**
- Test files: **{len(tests)}**
- Mangle sources: **{len(mgs)}**
- Tier: **{tier}** (full foundation always; higher tier adds more cross-cuts)
- Heuristic implementation completeness: **{completion}%**

## Full document set

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, funcs, models |
| [02-CURRENT-STATE-{name.upper()}.md](02-CURRENT-STATE-{name.upper()}.md) | Living inventory |
| [03-GAP-ANALYSIS-{name.upper()}.md](03-GAP-ANALYSIS-{name.upper()}.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety + verify gates |
| [05-CROSS-SYSTEM-WIRING.md](05-CROSS-SYSTEM-WIRING.md) | Integration surfaces |
| [06-TESTING-STRATEGY.md](06-TESTING-STRATEGY.md) | Test plan from inventory |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Import/dependency notes |
| [08-FAILURE-MODES.md](08-FAILURE-MODES.md) | Failure / risk surface |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + public surface |
| [TODO.md](TODO.md) | Open work |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Open design questions |
| [_progress.md](_progress.md) | Generation progress |

## Verify

```powershell
go test ./internal/{name}/...
```
""",
    )

    write(
        base / "00-ALIGNMENT-VISION-REVIEW.md",
        f"""# {name} — Alignment & Vision Review

{banner}

## North-star fit

codeNERD: LLM creative center; Mangle kernel executive. Package role:

**{role}**

| Dimension | Score (0–5) | Notes |
|-----------|-------------|-------|
| Creative/executive split | {"5" if name in ("core", "mangle", "perception", "articulation", "prompt") else "3"} | Relative to fact-flow spine |
| Fact-flow placement | {"5" if name in ("core", "perception", "session", "shards", "mangle") else "3"} | See domain model |
| Constitutional safety | {"5" if name in ("core", "mangle", "autopoiesis") else "3"} | permitted / safety surfaces |
| JIT / atom discipline | {"5" if name == "prompt" else ("4" if name in ("articulation", "jit") else "2")} | prompt atoms |
| Observability | {"5" if name in ("logging", "observability", "transparency") else "3"} | logs/metrics |
| Test grounding | {"4" if len(tests) >= max(1, len(srcs) // 2) else "2"} | {len(tests)} tests / {len(srcs)} src |

## Verdict

Living package under `{rel}/`. Full corpus is **code-grounded**, not pre-implementation fiction.
""",
    )

    write(
        base / "01-DOMAIN-MODEL.md",
        f"""# {name} — Domain Model

{banner}

## Package

`{rel}/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
{type_rows}

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
{func_rows}

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | {len(mgs)} |

| Path | Lines |
|------|------:|
{mg_rows}

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **{role}**
""",
    )

    write(
        base / f"02-CURRENT-STATE-{name.upper()}.md",
        f"""# {name} — Current State

{banner}

## 1. Source location

- Primary package: `{rel}/` (exists; {len(srcs)} non-test Go files)
- 1:1 mapping: `Docs/architecture/{name}/` ↔ `internal/{name}/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
{inv_rows}

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
{all_src_rows}

## 4. Tests (sample)

| Path | Lines |
|------|------:|
{test_rows}

## 5. Behavior summary

Package **{name}** is a living codeNERD subsystem: {role}.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic ({completion}%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
""",
    )

    write(
        base / f"03-GAP-ANALYSIS-{name.upper()}.md",
        f"""# {name} — Gap Analysis

{banner}

## Spec vs reality

| Area | Status | Notes |
|------|--------|-------|
| Package on disk | Yes | `{rel}/` |
| Source files | {len(srcs)} | non-test .go |
| Tests | {len(tests)} | `*_test.go` |
| Types sampled | {len(types)} | export scan |
| Mangle local | {len(mgs)} | package `.mg` |
| Full architecture corpus | Yes | this directory |

## Gaps (gates, not calendar)

1. Deep behavioral deep-dives beyond inventory when package is under active evolution.
2. Wiring proof for any new public entrypoints (registration, VirtualStore, CLI).
3. Test gaps if test count << source count (currently {len(tests)} vs {len(srcs)}).
4. Docs/Spec 18-file product templates remain a separate `spec-doc-sprint` track.

## Non-gaps

Implementation exists under `{rel}/`; do not treat as pre-implementation 0%.
""",
    )

    write(
        base / "04-INVARIANTS-AND-GATES.md",
        f"""# {name} — Invariants and Gates

{banner}

## Invariants

1. Source under `{rel}/` is authoritative over this corpus.
2. System actions remain compatible with `permitted(...)` / default deny.
3. New Mangle predicates require `Decl`; safe negation; stratification.
4. LLM-facing changes prefer prompt atoms (JIT) over ad-hoc prose.
5. Go: context-first I/O, wrapped errors, race-safe concurrency.

## Gates

| Gate | Check |
|------|-------|
| Tests | `go test ./internal/{name}/...` |
| Race (when concurrent) | `go test -race ./internal/{name}/...` |
| Binary (if CLI-impacting) | CGO sqlite-vec build of `./cmd/nerd` |
| Path existence | All cited `internal/` paths resolve |
| Surfaces | `validate_architecture_corpora.py` + optional `verify_surfaces.py` |
""",
    )

    write(
        base / "05-CROSS-SYSTEM-WIRING.md",
        f"""# {name} — Cross-System Wiring

{banner}

## Owned package

`{rel}/`

## Integration checklist (verify before claiming live)

| Surface | Path | Notes |
|---------|------|-------|
| This package | `{rel}/` | Exists |
| Kernel | `internal/core/` | Facts / VirtualStore / Dreamer |
| Mangle engine | `internal/mangle/` | Evaluation / feedback |
| Schemas/policy | `internal/core/defaults/` | Global Decl/policy |
| Shard registration | `internal/shards/registration.go` | If registers shards |
| Session | `internal/session/` | Execution loop |
| Prompt JIT | `internal/prompt/` | Atoms/compiler |
| Articulation | `internal/articulation/` | Piggyback/assembly |
| Config | `internal/config/` | Settings |
| CLI | `cmd/nerd/` | User entry |
| Tools/MCP | `internal/tools/`, `internal/mcp/` | External tools |

## Honesty

Do not invent routes or registrations. Grep registration hubs and callers before asserting a wire is live for **{name}**.
""",
    )

    write(
        base / "06-TESTING-STRATEGY.md",
        f"""# {name} — Testing Strategy

{banner}

## Current inventory

| Kind | Count |
|------|------:|
| Source files | {len(srcs)} |
| Test files | {len(tests)} |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `{rel}/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/{name}/...
go test -race ./internal/{name}/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
{test_rows}
""",
    )

    write(
        base / "07-DEPENDENCY-MAP.md",
        f"""# {name} — Dependency Map

{banner}

## Primary package

`{rel}/`

## Typical edges (codeNERD graph — validate with imports)

**Often upstream of many packages:** `internal/core`, `internal/config`, `internal/logging`, `internal/types`

**Often downstream consumers:** `cmd/nerd`, `internal/session`, `internal/shards`

Package-specific role: {role}

## How to refresh

```powershell
rg "codenerd/internal/{name}" -g "*.go" --glob "!*_test.go"
```

Record concrete import edges in deep-dives when this package is under design focus.
""",
    )

    write(
        base / "08-FAILURE-MODES.md",
        f"""# {name} — Failure Modes

{banner}

## Generic failure classes for `{rel}/`

| Mode | Symptoms | Mitigation |
|------|----------|------------|
| Missing wiring | Feature code exists but never runs | Grep registration / VirtualStore / CLI hooks |
| Kernel policy deny | Action blocked | Check `permitted` derivation and policy corpus |
| Mangle load failure | Boot dump `debug_program_ERROR.mg` | Decl, safety, stratification |
| LLM/client failure | Perception/articulation errors | Client factory, config engines |
| Store/IO failure | Persist errors | Context cancel, wrap errors, sqlite pragmas |
| Race/leak | Flaky tests, hung sessions | `-race`, goroutine lifecycle |

## Package-specific note

{role}

Revisit this file after incidents; attach real log paths under `.nerd/logs/` when available.
""",
    )

    if tier >= 3:
        if name in ("core", "mangle", "autopoiesis") or mgs:
            write(
                base / "09-MANGLE-SURFACE.md",
                f"""# {name} — Mangle Surface

{banner}

## Local `.mg`

| Path | Lines |
|------|------:|
{mg_rows}

## Global defaults

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Guardrails

Decl before use; `/atoms`; Upper variables; safe negation; `|>` aggregation.
""",
            )
        if name in ("prompt", "articulation", "jit"):
            write(
                base / "09-PROMPT-JIT-SURFACE.md",
                f"""# {name} — Prompt / JIT Surface

{banner}

## Anchors

- `internal/prompt/compiler.go`
- `internal/prompt/atoms/`
- `internal/articulation/prompt_assembler.go`

New LLM-facing behavior → atoms first.
""",
            )
        if name in ("core", "session", "shards", "autopoiesis", "system"):
            write(
                base / "09-CONSTITUTIONAL-SAFETY.md",
                f"""# {name} — Constitutional Safety

{banner}

- Default deny; `permitted(...)`
- Dreamer: `internal/core/dreamer.go` when present
- Package role: {role}
""",
            )

    components = [
        ("Source package tree", "Implemented" if srcs else "Minimal/empty", f"{completion}%" if srcs else "10%"),
        (
            "Exported types (sampled)",
            "Implemented" if types else "Partial",
            f"{min(completion, 80)}%" if types else "30%",
        ),
        (
            "Tests",
            "Implemented" if tests else "Missing/sparse",
            f"{min(completion, 90)}%" if tests else "0%",
        ),
        (
            "Mangle local sources",
            "Implemented" if mgs else "N/A or global-only",
            "n/a" if not mgs else f"{min(completion, 85)}%",
        ),
        ("Full architecture corpus", "Implemented", "100%"),
    ]
    comp_rows = "\n".join(f"| {a} | {b} | **{c}** |" for a, b, c in components)

    write(
        base / "IMPLEMENTED_SPEC.md",
        f"""# {name} — Implemented Spec

{banner}

## 1. Purpose

{role}

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `{rel}/` | Primary implementation |
| `Docs/architecture/{name}/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
{comp_rows}

**Overall (heuristic): {completion}%** as living package ({len(srcs)} src / {len(tests)} tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
{inv_rows}

### Types (sampled)

| Type | Location |
|------|----------|
{type_rows}

### Functions (sampled)

| Symbol | Location |
|--------|----------|
{func_rows}

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | {"Primary" if name in ("core", "mangle") else "Related"} |
| VirtualStore | {"Owner" if name == "core" else "Consumer if effectful"} |
| Shards | {"Owner" if name == "shards" else "Related"} |
| Prompt JIT | {"Owner" if name == "prompt" else ("Bridge" if name in ("articulation", "jit") else "Optional")} |
| CLI | Related via `cmd/nerd` |
| Config | {"Owner" if name == "config" else "Reader"} |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
""",
    )

    write(
        base / "TODO.md",
        f"""# {name} — TODO

- [ ] Deep-dive behavioral docs when this package is the active design target
- [ ] Reconcile with Docs/Spec via spec-doc-sprint when product specs are needed
- [ ] Re-run inventory after large refactors
- [ ] Prove wiring for any new public entrypoints
- [ ] Raise test depth if tests ({len(tests)}) lag sources ({len(srcs)})
""",
    )

    write(
        base / "OPEN-QUESTIONS.md",
        f"""# {name} — Open Questions

1. Which cross-package callers are load-bearing for `{rel}/` after the next major refactor?
2. Are there dormant integration points that look unused but are registration-driven?
3. Should any logic here move into Mangle policy vs stay in Go?
4. What observability category should own this package's critical path logs?
""",
    )

    write(
        base / "_progress.md",
        f"""# {name} — Progress

| Date | Event |
|------|-------|
| {TODAY} | Full corpus generated (1:1 with internal/{name}, tier {tier}) |
| {TODAY} | Inventory: {len(srcs)} go, {len(tests)} tests, {len(mgs)} mg |
""",
    )

    return {
        "name": name,
        "path": rel,
        "tier": tier,
        "role": role,
        "src": len(srcs),
        "tests": len(tests),
        "mg": len(mgs),
        "completion": completion,
        "status": status,
    }


def main() -> int:
    root = find_root()
    out = root / "Docs" / "architecture"
    packages = sorted(
        p.name for p in (root / "internal").iterdir() if p.is_dir() and not p.name.startswith(".")
    )
    results = [gen_corpus(root, out, n) for n in packages]

    lines = [
        "# Dark Factory Journal",
        "",
        f"> Complete 1:1 coverage of `internal/*` — {TODAY}",
        "",
        "## Decision",
        "",
        "- **Every** root folder under `internal/` gets a full architecture corpus under `Docs/architecture/<same-name>/`.",
        "- No leaf exclusions.",
        "- Code-grounded honesty (living code status, not pre-impl 0%).",
        "- Full document set per package: foundation 00–04, cross-wiring, testing, deps, failure modes, IMPLEMENTED_SPEC, governance.",
        "- Tier 2–3 only scales optional extra cross-cuts (Mangle/prompt/safety), not whether a full corpus exists.",
        "",
        f"## Package results ({len(results)} total)",
        "",
        "| Corpus | Source | Tier | src | tests | mg | % |",
        "|--------|--------|-----:|----:|------:|---:|--:|",
    ]
    for r in results:
        lines.append(
            f"| {r['name']} | `{r['path']}` | {r['tier']} | {r['src']} | {r['tests']} | {r['mg']} | {r['completion']} |"
        )
    lines += [
        "",
        "## Also present (non-internal)",
        "",
        "- `Docs/architecture/cli/` maps to `cmd/nerd/` (CLI surface; not an internal/ root).",
        "",
        "## Deviation from earlier spine-only run",
        "",
        "- Replaced spine-only + leaf exclusions with full 1:1 internal coverage per user correction.",
        "",
    ]
    write(out / "DARK-FACTORY-JOURNAL.md", "\n".join(lines) + "\n")

    rows = []
    for r in results:
        rows.append(
            f"| [{r['name']}]({r['name']}/) | `{r['path']}/` | {r['status']} | T{r['tier']} | {r['src']} go / {r['tests']} tests | [SPEC]({r['name']}/IMPLEMENTED_SPEC.md) |"
        )
    index = f"""# Architecture Corpora Index

> Last updated: {TODAY} — **1:1 with every `internal/*` package root**

Each top-level directory under `internal/` has a matching full corpus directory here.
Decisions: [DARK-FACTORY-JOURNAL.md](DARK-FACTORY-JOURNAL.md)

## Realized — internal packages ({len(results)})

| Corpus | Source | Status | Tier | Inventory | Spec |
|--------|--------|--------|------|-----------|------|
{chr(10).join(rows)}

## Non-internal surfaces

| Corpus | Source | Notes |
|--------|--------|-------|
| [cli](cli/) | `cmd/nerd/` | CLI/TUI entry (not under internal/) |

## Proposed (greenfield only)

| Feature | Notes |
|---------|-------|
| — | New packages register via arch-propose before `internal/<name>/` exists |

## Coverage rule

```
for dir in internal/*:
  require Docs/architecture/<dir>/ with full foundation + IMPLEMENTED_SPEC
```

Validated by: `python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py`
"""
    write(out / "INDEX.md", index)

    print(f"Generated {len(results)} corpora for internal packages:")
    for r in results:
        print(f"  {r['name']}: {r['src']} src, T{r['tier']}, {r['completion']}%")
    return 0


if __name__ == "__main__":
    sys.exit(main())
