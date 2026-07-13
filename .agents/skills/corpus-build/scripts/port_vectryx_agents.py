#!/usr/bin/env python3
"""Transform copied Vectryx agent definitions into codeNERD-appropriate full agents."""
from __future__ import annotations

import re
from pathlib import Path

ROOTS = [
    Path(r"C:\CodeProjects\codeNERD\.claude\agents"),
    Path(r"C:\CodeProjects\codeNERD\.grok\agents"),
]

NAMES = [
    "arch-propose-scout-internal.md",
    "arch-propose-scout-literature.md",
    "arch-propose-scout-convergent.md",
    "arch-propose-scout-divergent.md",
    "arch-propose-synthesizer.md",
    "arch-propose-auditor.md",
    "arch-propose-test-strategist.md",
    "arch-propose-ecosystem-mapper.md",
    "arch-writer.md",
    "cross-cutting-analyst.md",
    "requirements-interrogator.md",
    "corpus-builder.md",
    "corpus-comms-plumber.md",
    "corpus-consumables-keeper.md",
    "corpus-critic.md",
    "corpus-defense-auditor.md",
    "corpus-doc-auditor.md",
    "corpus-feature-tagger.md",
    "corpus-judge.md",
    "corpus-jules-dispatcher.md",
    "corpus-reader.md",
    "corpus-wiring-auditor.md",
]

REPL = [
    ("docs/architecture/", "Docs/architecture/"),
    ("docs\\architecture\\", "Docs/architecture/"),
    ("Vectryx's", "codeNERD's"),
    ("Vectryx", "codeNERD"),
    ("vectryx-cli", "nerd"),
    ("vectryx", "codenerd"),
    ("VECTRYX_", "NERD_"),
    ("internal/storage/", "internal/store/"),
    ("internal/storage", "internal/store"),
    ("internal/telemetry/", "internal/logging/"),
    ("internal/security/", "internal/core/defaults/policy/"),
    ("internal/bridge/", "internal/system/"),
    ("internal/api/rest/", "cmd/nerd/"),
    ("internal/api/", "cmd/nerd/"),
    ("internal/protocols/mcp/", "internal/mcp/"),
    ("internal/protocols/a2a/", "internal/mcp/"),
    ("internal/protocols/adk/", "internal/tools/"),
    ("internal/deductive/", "internal/mangle/"),
    ("internal/agents/", "internal/shards/"),
    ("pkg/cli/", "cmd/nerd/"),
    ("pkg/client/", "internal/perception/"),
    ("pkg/skills/", ".agents/skills/"),
    ("pagekit", "shard-UI"),
    ("Storyworld", "Campaign narrative"),
    ("storyworld", "campaign narrative"),
    ("SeaweedFS", "blob/store"),
    ("BadgerDB", "sqlite/store"),
    ("Orval", "codegen-client"),
    ("mission control", "campaign orchestration"),
    ("Mission Control", "Campaign orchestration"),
    ("RBAC", "constitutional safety (permitted)"),
    ("subgraph isolation", "package/fact-space isolation"),
    ("subgraph", "package-scope"),
    ("Subgraph", "Package-scope"),
    ("wormhole", "attention/routing"),
    ("Wormhole", "Attention/routing"),
    ("SleepCycle", "campaign/consolidation"),
    ("VectryxRAG", "retrieval/RAG"),
    ("VectryxRAP", "retrieval/RAP"),
    ("Logrus", "structured logging"),
    ("logrus", "logging"),
    ("testutil/", "internal/testing/"),
    ("make generate", "go generate / corpus scripts"),
    ("internal/api/rest/handlers", "cmd/nerd"),
    ("configs/*.yaml", ".nerd/config.json / internal/config"),
    ("configs/default.yaml", ".nerd/config.json"),
]

SKILL_MAP = {
    "arch-propose-scout": ["arch-propose", "codenerd-builder", "integration-auditor", "mangle-programming"],
    "arch-propose-synthesizer": ["arch-propose", "codenerd-builder", "mangle-programming", "prompt-architect"],
    "arch-propose-auditor": ["arch-propose", "codenerd-builder", "integration-auditor"],
    "arch-propose-test": ["arch-propose", "go-architect", "stress-tester"],
    "arch-propose-ecosystem": ["arch-propose", "integration-auditor", "codenerd-builder", "prompt-architect"],
    "arch-writer": ["arch-propose", "codenerd-builder", "spec-doc-sprint"],
    "cross-cutting": ["arch-propose", "integration-auditor", "prompt-architect", "codenerd-builder"],
    "requirements-interrogator": ["arch-propose", "codenerd-builder", "mangle-programming", "prompt-architect"],
    "corpus-builder": ["corpus-build", "go-architect", "mangle-programming", "codenerd-builder", "prompt-architect"],
    "corpus-reader": ["corpus-build", "integration-auditor", "codenerd-builder", "arch-propose"],
    "corpus-judge": ["corpus-build", "codenerd-builder", "integration-auditor", "go-architect"],
    "corpus-critic": ["corpus-build", "go-architect", "mangle-programming", "check-work"],
    "corpus-wiring": ["corpus-build", "integration-auditor", "codenerd-builder", "mangle-programming"],
    "corpus-doc": ["corpus-build", "arch-propose", "spec-doc-sprint", "codenerd-builder"],
    "corpus-comms": ["corpus-build", "cli-engine-integration", "codenerd-builder", "mcp"],
    "corpus-defense": ["corpus-build", "mangle-programming", "codenerd-builder", "integration-auditor"],
    "corpus-consumables": ["corpus-build", "codenerd-builder", "go-architect"],
    "corpus-feature": ["corpus-build", "codenerd-builder", "prompt-architect"],
    "corpus-jules": ["corpus-build", "integration-auditor", "stress-tester"],
}


def skills_for(fname: str) -> list[str]:
    stem = fname.replace(".md", "")
    for key, skills in SKILL_MAP.items():
        if stem.startswith(key) or key in stem:
            return skills
    if stem.startswith("arch-propose"):
        return SKILL_MAP["arch-propose-scout"]
    if stem.startswith("corpus-"):
        return ["corpus-build", "codenerd-builder", "go-architect"]
    return ["codenerd-builder"]


def adapt_frontmatter(text: str, fname: str) -> str:
    skills = skills_for(fname)
    text = re.sub(r"(?m)^model:\s*.*$", "model: inherit", text)
    text = re.sub(r"(?m)^reasoning_effort:\s*.*$", "reasoning_effort: high", text)
    text = re.sub(r"(?m)^effort:\s*.*$", "effort: high", text)

    fm_end = text.find("\n---", 3)
    if fm_end == -1:
        return text
    head = text[: fm_end + 4]
    body = text[fm_end + 4 :]

    skills_yaml = "skills:\n" + "\n".join(f"  - {s}" for s in skills)
    if re.search(r"(?m)^skills:", head):
        head = re.sub(r"(?ms)^skills:\n(?:[ \t]+-[^\n]*\n)*", skills_yaml + "\n", head)
        if "skills:" not in head:
            head = head.replace("\n---", "\n" + skills_yaml + "\n---", 1)
    else:
        head = head.replace("\n---", "\n" + skills_yaml + "\n---", 1)

    if "prompt_mode:" not in head:
        head = head.replace("\n---", "\nprompt_mode: full\nagents_md: true\n---", 1)
    elif "agents_md:" not in head:
        head = head.replace("\n---", "\nagents_md: true\n---", 1)

    body = re.sub(
        r"\.claude/hooks/corpus-build/[A-Za-z0-9_\-]+\.ps1",
        ".agents/skills/corpus-build/scripts/ (orchestrator enforces scope; hooks optional)",
        body,
    )
    body = re.sub(
        r"\.claude/hooks/modularization-guard\.ps1",
        "modularization discipline (go-architect / package boundaries)",
        body,
    )

    banner = (
        "\n\n> **codeNERD port of full Vectryx agent body.** "
        "Creative center = LLM; executive = Mangle kernel. "
        "Fact flow: `user_intent` → `next_action` → VirtualStore → articulation. "
        "JIT prompt atoms; constitutional `permitted(...)` (default deny). "
        "Architecture corpora live under `Docs/architecture/`. "
        "Prefer extend-existing packages; audit wiring before deleting “unused” code.\n"
    )
    if "codeNERD port of full Vectryx agent body" not in body:
        body = banner + body

    # Append codeNERD surface cheat sheet if not present
    cheatsheet = """

---

## codeNERD Surface Cheat Sheet (always apply)

| Need | Prefer |
|------|--------|
| Kernel / facts / VirtualStore | `internal/core/` |
| Mangle engine / feedback | `internal/mangle/` |
| Policy / Decl defaults | `internal/core/defaults/` |
| Perception / LLM clients | `internal/perception/` |
| Articulation / Piggyback | `internal/articulation/` |
| Prompt JIT / atoms | `internal/prompt/` |
| Session executor | `internal/session/` |
| Shards / registration | `internal/shards/` |
| Campaigns | `internal/campaign/` |
| Tools / MCP | `internal/tools/`, `internal/mcp/` |
| CLI / TUI | `cmd/nerd/` |
| Memory stores | `internal/store/` |
| Domain skills | `.agents/skills/*` |

Reserved hubs for intent files (do not race-edit): `internal/shards/registration.go`, VirtualStore routing files, `cmd/nerd/main.go` command registration, shared schema/policy files when multi-WU.

Build/test:
```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/<pkg>/...
# binary when needed:
go build -o nerd.exe ./cmd/nerd
```
"""
    if "codeNERD Surface Cheat Sheet" not in body:
        body = body.rstrip() + "\n" + cheatsheet + "\n"

    return head + body


def transform(text: str, fname: str) -> str:
    for a, b in REPL:
        text = text.replace(a, b)
    return adapt_frontmatter(text, fname)


def main() -> int:
    for root in ROOTS:
        root.mkdir(parents=True, exist_ok=True)
        for fname in NAMES:
            p = root / fname
            if not p.exists():
                print("MISSING", p)
                continue
            raw = p.read_text(encoding="utf-8", errors="replace")
            out = transform(raw, fname)
            p.write_text(out, encoding="utf-8")
            print(f"OK {root.name}/{fname}: {len(out)} chars / {p.stat().st_size} bytes")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
