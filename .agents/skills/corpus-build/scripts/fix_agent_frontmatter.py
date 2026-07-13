#!/usr/bin/env python3
"""Normalize agent frontmatter after port (skills list, no duplicate YAML junk)."""
from __future__ import annotations

import re
from pathlib import Path

ROOTS = [
    Path(r"C:\CodeProjects\codeNERD\.claude\agents"),
    Path(r"C:\CodeProjects\codeNERD\.grok\agents"),
]

SKILL_MAP = {
    "arch-propose-scout-internal": ["arch-propose", "codenerd-builder", "integration-auditor", "mangle-programming"],
    "arch-propose-scout-literature": ["arch-propose", "codenerd-builder", "research-builder"],
    "arch-propose-scout-convergent": ["arch-propose", "codenerd-builder", "integration-auditor"],
    "arch-propose-scout-divergent": ["arch-propose", "codenerd-builder", "nerd-evolve"],
    "arch-propose-synthesizer": ["arch-propose", "codenerd-builder", "mangle-programming", "prompt-architect"],
    "arch-propose-auditor": ["arch-propose", "codenerd-builder", "integration-auditor"],
    "arch-propose-test-strategist": ["arch-propose", "go-architect", "stress-tester", "corpus-build"],
    "arch-propose-ecosystem-mapper": ["arch-propose", "integration-auditor", "codenerd-builder", "prompt-architect"],
    "arch-writer": ["arch-propose", "codenerd-builder", "spec-doc-sprint"],
    "cross-cutting-analyst": ["arch-propose", "integration-auditor", "prompt-architect", "codenerd-builder"],
    "requirements-interrogator": ["arch-propose", "codenerd-builder", "mangle-programming", "prompt-architect"],
    "corpus-builder": ["corpus-build", "go-architect", "mangle-programming", "codenerd-builder", "prompt-architect"],
    "corpus-reader": ["corpus-build", "integration-auditor", "codenerd-builder", "arch-propose"],
    "corpus-judge": ["corpus-build", "codenerd-builder", "integration-auditor", "go-architect"],
    "corpus-critic": ["corpus-build", "go-architect", "mangle-programming", "check-work"],
    "corpus-wiring-auditor": ["corpus-build", "integration-auditor", "codenerd-builder", "mangle-programming"],
    "corpus-doc-auditor": ["corpus-build", "arch-propose", "spec-doc-sprint", "codenerd-builder"],
    "corpus-comms-plumber": ["corpus-build", "cli-engine-integration", "codenerd-builder"],
    "corpus-defense-auditor": ["corpus-build", "mangle-programming", "codenerd-builder", "integration-auditor"],
    "corpus-consumables-keeper": ["corpus-build", "codenerd-builder", "go-architect"],
    "corpus-feature-tagger": ["corpus-build", "codenerd-builder", "prompt-architect"],
    "corpus-jules-dispatcher": ["corpus-build", "integration-auditor", "stress-tester"],
}


def skills_for(stem: str) -> list[str]:
    if stem in SKILL_MAP:
        return SKILL_MAP[stem]
    if stem.startswith("arch-propose"):
        return ["arch-propose", "codenerd-builder"]
    if stem.startswith("corpus-"):
        return ["corpus-build", "codenerd-builder", "go-architect"]
    return ["codenerd-builder"]


def rebuild(path: Path) -> None:
    text = path.read_text(encoding="utf-8", errors="replace")
    if not text.startswith("---"):
        print("no fm", path)
        return
    end = text.find("\n---", 3)
    if end == -1:
        print("bad fm", path)
        return
    head = text[3:end].strip("\n")
    body = text[end + 4 :]  # after \n---

    # parse simple key: value and multi tools/disallowedTools
    desc_lines = []
    tools: list[str] = []
    disallowed: list[str] = []
    memory = "project"
    description = ""
    name = path.stem
    in_desc = False
    in_tools = False
    in_dis = False
    in_hooks = False
    hooks_raw = []

    for line in head.splitlines():
        if in_hooks:
            hooks_raw.append(line)
            continue
        if re.match(r"^hooks:\s*$", line):
            in_hooks = True
            hooks_raw = [line]
            in_tools = in_dis = in_desc = False
            continue
        if re.match(r"^name:\s*", line):
            name = line.split(":", 1)[1].strip().strip('"').strip("'")
            in_desc = in_tools = in_dis = False
            continue
        if re.match(r"^description:\s*", line):
            rest = line.split(":", 1)[1].strip()
            if rest in ('"', "'", ">", "|", "") or rest.startswith(">"):
                in_desc = True
                description = rest.lstrip(">").strip()
                if description.startswith('"') or description.startswith("'"):
                    description = description.strip("\"'")
            else:
                description = rest.strip("\"'")
                in_desc = False
            in_tools = in_dis = False
            continue
        if in_desc:
            if re.match(r"^[a-zA-Z0-9_]+:\s*", line) and not line.startswith(" "):
                in_desc = False
            else:
                description += " " + line.strip().strip("\"'")
                continue
        if re.match(r"^tools:\s*$", line):
            in_tools = True
            in_dis = False
            continue
        if re.match(r"^disallowedTools:\s*$", line):
            in_dis = True
            in_tools = False
            continue
        if re.match(r"^memory:\s*", line):
            memory = line.split(":", 1)[1].strip()
            in_tools = in_dis = False
            continue
        if in_tools and re.match(r"^\s*-\s+", line):
            tools.append(re.sub(r"^\s*-\s*", "", line).strip())
            continue
        if in_dis and re.match(r"^\s*-\s+", line):
            disallowed.append(re.sub(r"^\s*-\s*", "", line).strip())
            continue
        if re.match(r"^[a-zA-Z0-9_]+:\s*", line):
            in_tools = in_dis = False

    if not tools:
        tools = ["Read", "Glob", "Grep", "Bash", "Write"]
        if "corpus-builder" in path.stem or "writer" in path.stem or "doc-auditor" in path.stem:
            tools = ["Read", "Write", "Edit", "Glob", "Grep", "Bash"]
        if "scout" in path.stem or "judge" in path.stem or "reader" in path.stem or "interrogator" in path.stem:
            if "Write" not in tools:
                tools.append("Write")

    # corpus builders should not spawn agents
    if path.stem.startswith("corpus-") and path.stem not in ("corpus-judge",):
        if "Agent" not in disallowed and path.stem != "requirements-interrogator":
            disallowed = list(dict.fromkeys(disallowed + ["Agent"]))

    skills = skills_for(path.stem)
    if not description:
        description = f"{path.stem} agent for codeNERD architecture/corpus pipeline."

    # collapse whitespace in description
    description = re.sub(r"\s+", " ", description).strip()

    # permission_mode plan for scouts / read-heavy
    perm = "default"
    if any(x in path.stem for x in ("scout", "reader", "judge", "critic", "interrogator", "auditor", "analyst", "synthesizer", "mapper", "strategist")):
        if "builder" not in path.stem and "writer" not in path.stem and "doc-auditor" not in path.stem and "dispatcher" not in path.stem:
            perm = "plan"

    fm = [
        "---",
        f"name: {name}",
        "description: >",
        f"  {description}",
        "model: inherit",
        "effort: high",
        "reasoning_effort: high",
        f"memory: {memory}",
        "prompt_mode: full",
        f"permission_mode: {perm}",
        "agents_md: true",
        "tools:",
    ]
    for t in tools:
        fm.append(f"  - {t}")
    if disallowed:
        fm.append("disallowedTools:")
        for t in disallowed:
            fm.append(f"  - {t}")
    fm.append("skills:")
    for s in skills:
        fm.append(f"  - {s}")
    # Drop Vectryx-only hooks (scripts not ported); orchestrator/skill enforces scope.
    fm.append("---")

    # Clean body: remove broken leftover hook paths still saying .claude/hooks if doubled
    body = body.strip("\n")
    # ensure banner once
    banner = (
        "> **codeNERD port of full Vectryx agent body.** "
        "Creative center = LLM; executive = Mangle kernel. "
        "Fact flow: `user_intent` → `next_action` → VirtualStore → articulation. "
        "JIT prompt atoms; constitutional `permitted(...)` (default deny). "
        "Architecture corpora live under `Docs/architecture/`. "
        "Prefer extend-existing packages; audit wiring before deleting “unused” code."
    )
    if "codeNERD port of full Vectryx agent body" not in body:
        body = banner + "\n\n" + body
    else:
        # ensure banner is at top
        pass

    out = "\n".join(fm) + "\n\n" + body + "\n"
    path.write_text(out, encoding="utf-8")
    print(f"fixed {path} ({path.stat().st_size} bytes)")


def main() -> int:
    for root in ROOTS:
        for p in sorted(root.glob("*.md")):
            if p.name.startswith(
                (
                    "arch-propose-",
                    "corpus-",
                    "arch-writer",
                    "cross-cutting",
                    "requirements-interrogator",
                )
            ) or p.stem in (
                "arch-writer",
                "cross-cutting-analyst",
                "requirements-interrogator",
            ):
                rebuild(p)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
