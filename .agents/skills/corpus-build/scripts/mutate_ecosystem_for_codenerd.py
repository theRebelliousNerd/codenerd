#!/usr/bin/env python3
"""
Mutate a full codeNERD arch-propose / corpus-build ecosystem copy into codeNERD.

Runs on every text file under the skill trees, commands, and hooks.
Does NOT touch application Go/Mangle code.
"""
from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(r"C:\CodeProjects\codeNERD")

TREES = [
    ROOT / ".agents" / "skills" / "arch-propose",
    ROOT / ".agents" / "skills" / "corpus-build",
    ROOT / ".claude" / "skills" / "arch-propose",
    ROOT / ".claude" / "skills" / "corpus-build",
    ROOT / ".claude" / "commands",
    ROOT / ".grok" / "commands",
    ROOT / ".claude" / "hooks" / "corpus-build",
    ROOT / ".grok" / "hooks" / "corpus-build",
]

TEXT_EXT = {".md", ".yaml", ".yml", ".py", ".ps1", ".json", ".toml", ".txt"}

# Longer / more specific first
REPLACEMENTS: list[tuple[str, str]] = [
    # paths
    ("Docs/architecture/", "Docs/architecture/"),
    ("docs\\architecture\\", "Docs/architecture/"),
    ("Docs/architecture", "Docs/architecture"),
    ("Docs/", "Docs/"),
    ("Docs/", "Docs/"),
    (".agents/skills/arch-propose", ".agents/skills/arch-propose"),
    (".agents/skills/corpus-build", ".agents/skills/corpus-build"),
    (".grok/agents/", ".grok/agents/"),
    (".claude/commands/", ".claude/commands/"),
    (".claude/hooks/corpus-build", ".claude/hooks/corpus-build"),
    # product
    ("codeNERD's", "codeNERD's"),
    ("codeNERD", "codeNERD"),
    ("nerd", "nerd"),
    ("nerd-evolve", "nerd-evolve"),
    ("integration-auditor", "integration-auditor"),
    ("mangle-programming", "mangle-programming"),
    ("retrieval", "retrieval"),
    ("retrieval", "retrieval"),
    ("codeNERDRAG", "retrieval"),
    ("codeNERDRAP", "retrieval"),
    ("codenerd", "codenerd"),
    ("NERD_", "NERD_"),
    ("`spec-doc-sprint`", "`spec-doc-sprint`"),
    ("use ``spec-doc-sprint`", "use `spec-doc-sprint`"),
    ("use spec-doc-sprint", "use spec-doc-sprint"),
    # packages / surfaces
    ("internal/store/", "internal/store/"),
    ("internal/store/", "internal/store/"),
    ("internal/store", "internal/store"),
    ("internal/logging/", "internal/logging/"),
    ("internal/logging", "internal/logging"),
    ("internal/core/defaults/policy/", "internal/core/defaults/policy/"),
    ("internal/core/defaults/policy", "internal/core/defaults/policy"),
    ("internal/system/", "internal/system/"),
    ("internal/system", "internal/system"),
    ("cmd/nerd", "cmd/nerd"),
    ("cmd/nerd/", "cmd/nerd/"),
    ("cmd/nerd/", "cmd/nerd/"),
    ("internal/mcp/", "internal/mcp/"),
    ("internal/mcp/", "internal/mcp/"),
    ("internal/tools/", "internal/tools/"),
    ("internal/mcp/", "internal/mcp/"),
    ("internal/mangle/", "internal/mangle/"),
    ("internal/mangle", "internal/mangle"),
    ("internal/shards/", "internal/shards/"),
    ("internal/shards/", "internal/shards/"),
    ("Docs/architecture/", "Docs/architecture/"),
    ("cmd/nerd/", "cmd/nerd/"),
    ("internal/perception/", "internal/perception/"),
    (".agents/skills/", ".agents/skills/"),
    ("internal/", "internal/"),
    ("cmd/codenerd", "cmd/nerd"),
    # product concepts -> codeNERD analogues
    ("shard agents / TUI pages", "shard agents / TUI pages"),
    ("shard-UI", "shard-UI"),
    ("Campaign narrative", "Campaign narrative"),
    ("campaign narrative", "campaign narrative"),
    ("CAMPAIGN_NARRATIVE", "CAMPAIGN_NARRATIVE"),
    ("store/blob", "store/blob"),
    ("sqlite store", "sqlite store"),
    ("API-client codegen", "API-client codegen"),
    ("campaign orchestration", "campaign orchestration"),
    ("Campaign orchestration", "Campaign orchestration"),
    ("CAMPAIGN-CONTROLLABILITY", "CAMPAIGN-CONTROLLABILITY"),
    ("CONSTITUTIONAL-SAFETY", "CONSTITUTIONAL-SAFETY"),
    ("constitutional safety (permitted)", "constitutional safety (permitted)"),
    ("package/fact-space isolation", "package/fact-space isolation"),
    ("Package/fact-space isolation", "Package/fact-space isolation"),
    ("package-scope name", "package-scope name"),
    ("package-scope", "package-scope"),
    ("Package-scope", "Package-scope"),
    ("read-before-write (persistent store)", "read-before-write (persistent store) (persistent store)"),
    ("durable/persistent", "durable/persistent"),
    ("attention-routing", "attention-routing"),
    ("Attention-routing", "Attention-routing"),
    ("consolidation cycle", "consolidation cycle"),
    ("consolidation", "consolidation"),
    ("CLI-TUI-SURFACE", "CLI-TUI-SURFACE"),
    ("KERNEL-VIRTUALSTORE-SURFACE", "KERNEL-VIRTUALSTORE-SURFACE"),
    ("structured logging", "structured logging"),
    ("logging", "logging"),
    ("internal/testing/", "internal/testing/"),
    ("go generate / client scripts if present", "go generate / client scripts if present"),
    ("OpenAPI gen if present", "OpenAPI gen if present"),
    ("client parity checks if present", "client parity checks if present"),
    ("go generate", "go generate"),
    ("host go test/build (scoped packages)", "host go test/build (scoped packages)"),
    ("scoped package tests; full-tree with care", "scoped package tests; full-tree with care"),
    ("scoped package path", "scoped package path"),
    (".nerd/config.json and internal/config", ".nerd/config.json and internal/config"),
    (".nerd/config.json", ".nerd/config.json"),
    ("registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main)", "registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main)"),
    ("MCP/tool schemas", "MCP/tool schemas"),
    ("tool/MCP schema", "tool/MCP schema"),
    ("high/", "high/"),
    ("high/", "high/"),
    ("medium/", "medium/"),
    ("high-judgment model", "high-judgment model"),
    ("high-accuracy model", "high-accuracy model"),
    ("fast model", "fast model"),
    ("prompt-architect / corpus-doc-auditor", "prompt-architect / corpus-doc-auditor"),
    ("go-architect / test-forge unit", "go-architect / test-forge unit"),
    ("test-forge integration", "test-forge integration"),
    ("corpus-comms-plumber", "corpus-comms-plumber"),
    ("Docs/architecture/INDEX.md", "Docs/architecture/INDEX.md"),
    ("Docs/architecture/INDEX", "Docs/architecture/INDEX"),
]

CODE_NERD_VISION = """# codeNERD Vision Summary (corpus-build inject)

Regenerate if root AGENTS.md drifts. Ported ecosystem mutation date in skill metadata.

## North Star

codeNERD is a high-assurance, logic-first CLI coding agent.
- **LLM = creative center** (problem solving, synthesis, insight)
- **Mangle kernel = executive** (planning, memory, orchestration, safety, policy)
- **Transduction**: NL/code ↔ formal atoms the kernel can reason over

## Runtime spine

```
user input → perception → user_intent → kernel derives next_action
  → VirtualStore executes → articulation responds
```

OODA: Observe → Orient → Decide → Act.
Constitutional safety: every action must derive `permitted(...)`; default deny.
JIT is the standard for new LLM-facing behavior (prompt atoms under `internal/prompt/atoms/`).

## Key live locations

| Area | Location |
|------|----------|
| Kernel | `internal/core/` |
| Policy | `internal/core/defaults/policy/` |
| Schemas | `internal/core/defaults/schemas.mg` |
| Mangle engine | `internal/mangle/` |
| Perception | `internal/perception/` |
| Articulation | `internal/articulation/` |
| Prompt JIT | `internal/prompt/` |
| Session | `internal/session/` |
| Shards | `internal/shards/` |
| Campaign | `internal/campaign/` |
| Store | `internal/store/` |
| Tools / MCP | `internal/tools/`, `internal/mcp/` |
| CLI | `cmd/nerd/` |
| Architecture corpora | `Docs/architecture/` |

## Build / test

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
go test ./...
```

## What corpus-build must preserve

Strengthen the creative/executive split. Mangle for deduction/policy; Go for effects;
prompt atoms for LLM text; VirtualStore for external effects; audit wiring before deletes.
"""

CODE_NERD_SURFACES = """# codeNERD corpus-build integration surface registry — schema v1
# Machine-authoritative. Human checklist: 02-integration-surface-checklist.md
# Parsed by scripts/verify_surfaces.py (stdlib YAML subset).

- id: A1-kernel
  category: engine
  group: A
  name: Kernel surface
  paths: [internal/core/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: kernel}
      - touches_paths: [internal/core/]
  detection:
    - grep: '<subsystem>'
      dir: internal/core/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A2-schemas
  category: engine
  group: A
  name: Mangle schemas Decl
  paths: [internal/core/defaults/schemas.mg]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: mangle}
      - manifest_field_contains: {field: integration_points, value: schemas}
  detection:
    - grep: 'Decl '
      dir: internal/core/defaults/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A3-policy
  category: engine
  group: A
  name: Policy corpus
  paths: [internal/core/defaults/policy/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: policy}
      - touches_paths: [internal/core/defaults/policy/]
  detection:
    - grep: '<subsystem>'
      dir: internal/core/defaults/policy/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A4-virtual-store
  category: engine
  group: A
  name: VirtualStore routing
  paths: [internal/core/virtual_store.go, internal/core/virtual_store_routing.go]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: virtual_store}
      - touches_paths: [internal/core/]
  detection:
    - grep: '<subsystem>'
      dir: internal/core/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A5-shard-manager
  category: engine
  group: A
  name: Shard manager
  paths: [internal/core/shards/, internal/shards/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: shards}
      - touches_paths: [internal/shards/, internal/core/shards/]
  detection:
    - grep: '<subsystem>'
      dir: internal/shards/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A6-shard-registration
  category: engine
  group: A
  name: Shard registration hub
  paths: [internal/shards/registration.go]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: shards}
  detection:
    - grep: 'Register'
      dir: internal/shards/registration.go
  fix_owner: corpus-wiring-auditor
  evidence_required: file:line

- id: A7-session
  category: engine
  group: A
  name: Session executor
  paths: [internal/session/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: session}
      - touches_paths: [internal/session/]
  detection:
    - grep: '<subsystem>'
      dir: internal/session/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A8-perception
  category: engine
  group: A
  name: Perception transducer
  paths: [internal/perception/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: perception}
      - touches_paths: [internal/perception/]
  detection:
    - grep: '<subsystem>'
      dir: internal/perception/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A9-articulation
  category: engine
  group: A
  name: Articulation / Piggyback
  paths: [internal/articulation/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: articulation}
      - touches_paths: [internal/articulation/]
  detection:
    - grep: '<subsystem>'
      dir: internal/articulation/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A10-prompt-jit
  category: engine
  group: A
  name: Prompt compiler and atoms
  paths: [internal/prompt/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: prompt}
      - touches_paths: [internal/prompt/]
  detection:
    - grep: '<subsystem>'
      dir: internal/prompt/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: A11-mangle-engine
  category: engine
  group: A
  name: Mangle engine package
  paths: [internal/mangle/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: mangle}
      - touches_paths: [internal/mangle/]
  detection:
    - grep: '<subsystem>'
      dir: internal/mangle/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: B1-config
  category: config
  group: B
  name: Config surface
  paths: [internal/config/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: config}
      - touches_paths: [internal/config/]
  detection:
    - grep: '<subsystem>'
      dir: internal/config/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: B2-cli
  category: binary
  group: B
  name: CLI cmd/nerd
  paths: [cmd/nerd/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: cli}
      - touches_paths: [cmd/nerd/]
  detection:
    - grep: '<subsystem>'
      dir: cmd/nerd/
  fix_owner: corpus-comms-plumber
  evidence_required: file:line

- id: C1-store
  category: engine
  group: C
  name: Store tiers
  paths: [internal/store/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: store}
      - touches_paths: [internal/store/]
  detection:
    - grep: '<subsystem>'
      dir: internal/store/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: D1-logging
  category: engine
  group: D
  name: Logging categories
  paths: [internal/logging/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: logging}
      - touches_paths: [internal/logging/]
  detection:
    - grep: '<subsystem>'
      dir: internal/logging/
  fix_owner: corpus-defense-auditor
  evidence_required: file:line

- id: D2-dreamer
  category: engine
  group: D
  name: Dreamer precog safety
  paths: [internal/core/dreamer.go]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: safety}
      - manifest_field_contains: {field: integration_points, value: dreamer}
  detection:
    - grep: 'Dreamer'
      dir: internal/core/
  fix_owner: corpus-defense-auditor
  evidence_required: file:line

- id: E1-campaign
  category: engine
  group: E
  name: Campaign orchestration
  paths: [internal/campaign/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: campaign}
      - touches_paths: [internal/campaign/]
  detection:
    - grep: '<subsystem>'
      dir: internal/campaign/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: F1-tools
  category: protocol
  group: F
  name: Tools registry
  paths: [internal/tools/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: tools}
      - touches_paths: [internal/tools/]
  detection:
    - grep: '<subsystem>'
      dir: internal/tools/
  fix_owner: corpus-builder
  evidence_required: file:line

- id: F2-mcp
  category: protocol
  group: F
  name: MCP integration
  paths: [internal/mcp/]
  applicability:
    any:
      - manifest_field_contains: {field: integration_points, value: mcp}
      - touches_paths: [internal/mcp/]
  detection:
    - grep: '<subsystem>'
      dir: internal/mcp/
  fix_owner: corpus-comms-plumber
  evidence_required: file:line

- id: G1-tests
  category: testing
  group: G
  name: Package tests present
  paths: []
  applicability:
    always: true
  detection: []
  judgment: true
  fix_owner: corpus-builder
  evidence_required: file:line

- id: G2-architecture-docs
  category: docs
  group: G
  name: Architecture corpus status
  paths: [Docs/architecture/]
  applicability:
    any:
      - touches_paths: [Docs/architecture/]
      - manifest_field_contains: {field: integration_points, value: architecture_docs}
  detection:
    - glob: 'IMPLEMENTED_SPEC.md'
      dir: Docs/architecture/<subsystem>/
  fix_owner: corpus-doc-auditor
  evidence_required: file:line or directory exists
"""


def mutate_text(text: str, rel: str) -> str:
    for a, b in REPLACEMENTS:
        text = text.replace(a, b)

    # metadata stamps
    text = re.sub(
        r"(?m)^(\s*author:\s*).*$",
        r"\1codeNERD (full ecosystem port from codeNERD; mutated for codeNERD)",
        text,
    )
    text = re.sub(
        r"(?m)^(\s*last-verified:\s*).*$",
        r"\12026-07-13",
        text,
    )
    text = re.sub(
        r"(?m)^(\s*version:\s*)1\.[0-9.]+",
        r"\g<1>2.0.0",
        text,
    )

    if rel.endswith("SKILL.md") and "codeNERD full ecosystem port" not in text:
        banner = (
            "\n\n> **codeNERD full ecosystem port.** Entire skill tree copied from codeNERD "
            "then mutated file-by-file. Agent fleet lives under `.grok/agents/` and "
            "`.grok/agents/` with bound `skills:` frontmatter. Architecture corpora: "
            "`Docs/architecture/`. North star: LLM creative / Mangle executive; "
            "`permitted(...)` default deny; JIT prompt atoms.\n"
        )
        # after first H1 body start
        m = re.search(r"(?m)^# .+$", text)
        if m:
            insert_at = m.end()
            text = text[:insert_at] + banner + text[insert_at:]

    return text


def should_skip(path: Path) -> bool:
    if path.suffix.lower() not in TEXT_EXT and path.name not in ("SKILL.md",):
        if path.suffix.lower() in {".png", ".jpg", ".exe", ".dll"}:
            return True
    if "__pycache__" in path.parts or path.suffix == ".pyc":
        return True
    # keep our mutator scripts
    return False


def process_tree(tree: Path) -> int:
    if not tree.exists():
        print("missing tree", tree)
        return 0
    n = 0
    for path in tree.rglob("*"):
        if not path.is_file() or should_skip(path):
            continue
        if path.suffix.lower() not in TEXT_EXT and path.name not in (
            "SKILL.md",
            "CHANGELOG.md",
            "README.md",
        ):
            # still try common text without extension? skip
            if path.suffix:
                continue
        try:
            raw = path.read_text(encoding="utf-8", errors="replace")
        except Exception as e:
            print("read fail", path, e)
            continue
        rel = str(path.relative_to(ROOT)).replace("\\", "/")
        out = mutate_text(raw, rel)
        if out != raw:
            path.write_text(out, encoding="utf-8")
            n += 1
            print("mutated", rel)
        else:
            print("unchanged", rel)
    return n


def write_overrides() -> None:
    # vision + surfaces authoritative for codeNERD
    for base in (
        ROOT / ".agents" / "skills" / "corpus-build" / "references",
        ROOT / ".claude" / "skills" / "corpus-build" / "references",
    ):
        if not base.exists():
            continue
        (base / "vision-summary.md").write_text(CODE_NERD_VISION, encoding="utf-8")
        (base / "surfaces.yaml").write_text(CODE_NERD_SURFACES, encoding="utf-8")
        journal = base / "journal.md"
        journal.write_text(
            """# corpus-build Journal (codeNERD)

Living log of runs, SEED insights, and measured economics.
No invented time/cost estimates — measured ledger only.

## Economics table

| Date | Subsystem | Run ID | WUs | Gate cycles | Notes |
|------|-----------|--------|-----|-------------|-------|
| 2026-07-13 | (ecosystem port) | port | — | — | Full codeNERD skill tree copied + mutated |

## Seeds

<!-- SEED:subsystem-X:insight -->
<!-- SEED:reuse:pattern -->
<!-- SEED:gap:capability -->
<!-- SEED:mangle:... -->
<!-- SEED:prompt:... -->

## Lessons

- Full ecosystem port 2026-07-13: skills + references + scripts + hooks + command + PLAN docs + agent fleet.
""",
            encoding="utf-8",
        )
        print("wrote overrides in", base)


def main() -> int:
    total = 0
    for t in TREES:
        total += process_tree(t)
    write_overrides()
    # sync .claude skills from .agents after mutation (single source of truth = .agents)
    import shutil

    for name in ("arch-propose", "corpus-build"):
        src = ROOT / ".agents" / "skills" / name
        dst = ROOT / ".claude" / "skills" / name
        if dst.exists():
            shutil.rmtree(dst)
        shutil.copytree(src, dst)
        print("synced .claude/skills/", name)
    print("total text files mutated:", total)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
