#!/usr/bin/env python3
"""Mirror corpus-build/references/common/ into every corpus-build micro-skill.

Common references (anti-hallucination gate, package-scope/write discipline,
reporting format, spec-attribution format -- PLAN-corpus-build.md Sec.4) live
once at .agents/skills/corpus-build/references/common/ (canon). Windows
checkouts here run with core.symlinks=false by default, so a symlink
silently materializes as plain text and a junction doesn't survive git --
both are unacceptable for load-bearing rule text every fleet agent depends
on. This script is the sync mechanism instead: boring, verifiable, and
CI-checkable (`--check` fails when a mirror drifts from canon).

Targets are every existing `.claude/skills/corpus-*/` directory EXCEPT
`corpus-build` itself (the canon owner) and `corpus-feature-tagging` (a
separate, correctly-wired lineage per PLAN-corpus-build.md Sec.1 -- not part
of this fleet). Most corpus-build micro-skills (critic, comms-plumber,
defense-auditor, consumables-keeper, doc-auditor, jules-dispatcher, ...) are
built incrementally by parallel work items; this script tolerates their
absence and simply reports which target directories it found.

Each mirrored file gets a stamped header line so drift is visible in a raw
diff without running this script:

    <!-- SYNCED from corpus-build/references/common/<name> sha256:<first12> -- DO NOT EDIT, edit canon + rerun sync_common_refs.py -->

Usage:
    python sync_common_refs.py             # sync canon -> every found target
    python sync_common_refs.py --check     # verify only, exit 1 on drift
    python sync_common_refs.py --quiet     # suppress informational prints
"""

import argparse
import hashlib
import sys
from pathlib import Path

EXCLUDED_SKILLS = {"corpus-build", "corpus-feature-tagging"}
HEADER_TEMPLATE = (
    "<!-- SYNCED from corpus-build/references/common/{name} "
    "sha256:{digest12} -- DO NOT EDIT, edit canon + rerun "
    "sync_common_refs.py -->\n\n"
)


def find_project_root() -> Path:
    cwd = Path.cwd()
    for parent in [cwd, *cwd.parents]:
        if (parent / "go.mod").exists():
            return parent
    return cwd


def canon_dir(root: Path) -> Path:
    return root / ".claude" / "skills" / "corpus-build" / "references" / "common"


def skills_root(root: Path) -> Path:
    return root / ".claude" / "skills"


def sha256_hex(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def load_canon_files(canon: Path) -> dict:
    """name -> raw text content, for every file directly in canon (non-recursive)."""
    files = {}
    if not canon.exists():
        return files
    for f in sorted(canon.iterdir()):
        if f.is_file():
            files[f.name] = f.read_text(encoding="utf-8", errors="replace")
    return files


def render_mirror(name: str, canon_text: str) -> str:
    digest = sha256_hex(canon_text)
    header = HEADER_TEMPLATE.format(name=name, digest12=digest[:12])
    return header + canon_text


def discover_targets(root: Path) -> dict:
    """Returns {"synced": [dirs], "excluded": [names], "found": [names]}."""
    base = skills_root(root)
    found = []
    excluded = []
    targets = []
    if not base.exists():
        return {"found": found, "excluded": excluded, "targets": targets}
    for d in sorted(base.iterdir()):
        if not d.is_dir() or not d.name.startswith("corpus-"):
            continue
        found.append(d.name)
        if d.name in EXCLUDED_SKILLS:
            excluded.append(d.name)
            continue
        targets.append(d)
    return {"found": found, "excluded": excluded, "targets": targets}


def sync(root: Path, quiet: bool) -> int:
    canon = canon_dir(root)
    canon_files = load_canon_files(canon)
    if not canon_files:
        print(f"[sync_common_refs] no canon files found under {canon}", file=sys.stderr)
        return 1

    disc = discover_targets(root)
    if not quiet:
        print(f"[sync_common_refs] canon: {len(canon_files)} file(s) in {canon}")
        print(f"[sync_common_refs] discovered {len(disc['found'])} corpus-* dir(s): "
              f"{', '.join(disc['found']) if disc['found'] else '(none)'}")
        if disc["excluded"]:
            print(f"[sync_common_refs] excluded (not fleet targets): "
                  f"{', '.join(disc['excluded'])}")

    if not disc["targets"]:
        if not quiet:
            print("[sync_common_refs] no sync targets found yet -- nothing to do "
                  "(micro-skills are created incrementally by other work items)")
        return 0

    synced_files = 0
    for target_dir in disc["targets"]:
        common_dir = target_dir / "references" / "common"
        common_dir.mkdir(parents=True, exist_ok=True)
        for name, canon_text in canon_files.items():
            mirror_path = common_dir / name
            new_content = render_mirror(name, canon_text)
            existing = mirror_path.read_text(encoding="utf-8", errors="replace") if mirror_path.exists() else None
            if existing != new_content:
                mirror_path.write_text(new_content, encoding="utf-8", newline="\n")
                synced_files += 1
        if not quiet:
            print(f"[sync_common_refs] synced -> {common_dir.relative_to(root).as_posix()}")

    if not quiet:
        print(f"[sync_common_refs] wrote/updated {synced_files} mirror file(s) "
              f"across {len(disc['targets'])} target(s)")
    return 0


def check(root: Path, quiet: bool) -> int:
    canon = canon_dir(root)
    canon_files = load_canon_files(canon)
    if not canon_files:
        print(f"[sync_common_refs] --check: no canon files found under {canon}", file=sys.stderr)
        return 1

    disc = discover_targets(root)
    if not quiet:
        print(f"[sync_common_refs] --check: canon has {len(canon_files)} file(s); "
              f"{len(disc['targets'])} sync target(s) found "
              f"({', '.join(d.name for d in disc['targets']) if disc['targets'] else 'none'})")

    if not disc["targets"]:
        if not quiet:
            print("[sync_common_refs] --check: no targets to verify -- OK")
        return 0

    drifted = []
    for target_dir in disc["targets"]:
        common_dir = target_dir / "references" / "common"
        if not common_dir.exists():
            drifted.append(f"{common_dir.relative_to(root).as_posix()} -- missing entirely")
            continue
        for name, canon_text in canon_files.items():
            mirror_path = common_dir / name
            expected = render_mirror(name, canon_text)
            if not mirror_path.exists():
                drifted.append(f"{mirror_path.relative_to(root).as_posix()} -- missing")
                continue
            actual = mirror_path.read_text(encoding="utf-8", errors="replace")
            if actual != expected:
                drifted.append(f"{mirror_path.relative_to(root).as_posix()} -- content drift vs canon")
        # orphaned mirror files (present locally, absent from canon)
        for f in sorted(common_dir.iterdir()):
            if f.is_file() and f.name not in canon_files:
                drifted.append(f"{f.relative_to(root).as_posix()} -- orphaned (not in canon)")

    if drifted:
        print(f"[sync_common_refs] --check: {len(drifted)} drifted file(s):", file=sys.stderr)
        for d in drifted:
            print(f"  - {d}", file=sys.stderr)
        return 1

    if not quiet:
        print("[sync_common_refs] --check: all mirrors match canon -- OK")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--check", action="store_true",
        help="Verify existing mirrors match canon; exit 1 listing drift. "
             "Does not write anything.",
    )
    parser.add_argument(
        "--quiet", action="store_true",
        help="Suppress informational prints.",
    )
    args = parser.parse_args()

    root = find_project_root()
    if args.check:
        return check(root, args.quiet)
    return sync(root, args.quiet)


if __name__ == "__main__":
    raise SystemExit(main())
