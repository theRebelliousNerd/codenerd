#!/usr/bin/env python3
"""Compile the machine-readable context index for corpus-build v2.

Unifies three EXISTING metadata systems into one generated artifact,
Docs/architecture/roadmap/33_corpus_context_index.json (PLAN-corpus-build.md
Sec.6):

  a. NERD_FEATURE HTML-comment tags across Docs/architecture/**/*.md
     (FEATURE_TAGGING_SCHEMA.md format: id/topic/plane/status). Parses the
     docs directly (ground truth), then cross-checks against the existing
     31_feature_tag_register.csv and reports any drift to stderr WITHOUT
     failing -- the register is a downstream artifact, not the source of
     truth (FEATURE_TAGGING_SCHEMA.md Sec.2 rule 5).
  b. The graphcad-style 6-field YAML frontmatter block (doc-class,
     doc-audience, doc-opsec, implementation-status, last-verified,
     supersedes). Partial corpus coverage is BY DESIGN -- most corpora have
     none yet; this never fails on absence.
  c. Roadmap CSVs: 21_feature_inventory.csv (subsystem coverage
     cross-check) and Docs/architecture/INDEX.md (subsystem -> tier + internal/ source
     path; drives the "packages" map).

Tag-block scanning reuses the exact filename/directory exclusion rules
from the canonical extractor (scripts/extract_feature_tags.py) so this
index's tagged-doc set matches 31_feature_tag_register.csv's universe:
governance/summary docs (IMPLEMENTED_SPEC.md, TODO.md, README.md, etc.)
are not canonical tag owners (FEATURE_TAGGING_SCHEMA.md Sec.5-6), and the
derived Docs/architecture/compiled/ mirror is excluded everywhere (it is a
regenerated concatenation of the live corpus and would double-count every
tag and every frontmatter block it mirrors).

Frontmatter scanning has NO filename exclusions (graphcad tags TODO.md,
README.md, and CLAUDE.md with governance frontmatter) -- only the
compiled/ mirror is excluded, for the same duplication reason.

Output is fully deterministic (sorted keys, sorted list members) so
regen diffs are meaningful; the only source of run-to-run churn is the
top-level "generated" timestamp, which --check ignores when diffing.

Usage:
    python build_tag_index.py                 # regenerate + write + report
    python build_tag_index.py --check          # regen to memory, diff
                                                # against committed file,
                                                # exit 1 on drift
    python build_tag_index.py --quiet          # suppress info/diagnostic
                                                # prints (errors still exit
                                                # non-zero)
"""

import argparse
import csv
import json
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

# ---------------------------------------------------------------------------
# Repo root discovery (matches the other corpus-build scripts' convention)
# ---------------------------------------------------------------------------


def find_project_root() -> Path:
    cwd = Path.cwd()
    for parent in [cwd, *cwd.parents]:
        if (parent / "go.mod").exists():
            return parent
    return cwd


# ---------------------------------------------------------------------------
# (a) NERD_FEATURE tag parsing -- mirrors scripts/extract_feature_tags.py
# ---------------------------------------------------------------------------

TAG_RE = re.compile(r"<!--\s*NERD_FEATURE\s*\n(.*?)-->", re.DOTALL)
TAG_FIELD_RE = re.compile(r"^(id|topic|plane|status):\s*(.+)$")

# Exact parity with scripts/extract_feature_tags.py::should_scan(). Canonical
# owner docs are numbered deep-dives; these filenames are downstream/
# governance views per FEATURE_TAGGING_SCHEMA.md Sec.5-6 and are never tag
# owners even if a stray "NERD_FEATURE" string appears in their prose.
TAG_EXCLUDED_FILENAMES = {
    "IMPLEMENTED_SPEC.md", "TODO.md", "_progress.md", "OPEN-QUESTIONS.md",
    "README.md", "CLAUDE.md", "AGENTS.md", "FEATURE_TAGGING_SCHEMA.md",
}


def should_scan_for_tags(path: Path) -> bool:
    if "compiled" in path.parts:
        return False
    return path.name not in TAG_EXCLUDED_FILENAMES


def parse_tag_blocks(text: str) -> list:
    """Return [(fields_dict, line_no), ...] for every well-formed tag block."""
    blocks = []
    for match in TAG_RE.finditer(text):
        fields = {}
        for raw_line in match.group(1).splitlines():
            line = raw_line.strip()
            if not line:
                continue
            fm = TAG_FIELD_RE.match(line)
            if fm:
                fields[fm.group(1)] = fm.group(2).strip()
        if not all(k in fields for k in ("id", "topic", "plane", "status")):
            continue
        line_no = text[: match.start()].count("\n") + 1
        blocks.append((fields, line_no))
    return blocks


# ---------------------------------------------------------------------------
# (b) 6-field frontmatter parsing (graphcad doc-class convention)
# ---------------------------------------------------------------------------

FRONTMATTER_KEYS = (
    "doc-class", "doc-audience", "doc-opsec",
    "implementation-status", "last-verified", "supersedes",
)
FRONTMATTER_LINE_RE = re.compile(r"^([A-Za-z0-9_-]+):\s*(.*)$")
FRONTMATTER_LIST_ITEM_RE = re.compile(r"^\s+-\s+(.*)$")


def should_scan_for_frontmatter(path: Path) -> bool:
    # Frontmatter IS placed on TODO.md/README.md/CLAUDE.md by design
    # (graphcad governance docs); only the derived compiled/ mirror is
    # excluded to avoid double-counting.
    return "compiled" not in path.parts


def parse_frontmatter(text: str) -> dict:
    """Minimal hand-rolled parser for the fixed 6-field block only.

    Not a general YAML parser -- deliberately narrow, stdlib-only, and
    tolerant of the two shapes seen in the corpus: `key: value` and
    `key:` followed by `  - item` list lines (or `key: []`).
    """
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}
    end_idx = None
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            end_idx = i
            break
    if end_idx is None:
        return {}

    result = {}
    body = lines[1:end_idx]
    i = 0
    while i < len(body):
        line = body[i]
        if not line.strip():
            i += 1
            continue
        fm = FRONTMATTER_LINE_RE.match(line)
        if not fm:
            i += 1
            continue
        key, val = fm.group(1), fm.group(2).strip()
        if val == "":
            items = []
            j = i + 1
            while j < len(body):
                im = FRONTMATTER_LIST_ITEM_RE.match(body[j])
                if not im:
                    break
                items.append(im.group(1).strip())
                j += 1
            result[key] = items
            i = j
            continue
        if val == "[]":
            result[key] = []
        else:
            if len(val) >= 2 and val[0] == val[-1] and val[0] in "\"'":
                val = val[1:-1]
            result[key] = val
        i += 1
    return {k: v for k, v in result.items() if k in FRONTMATTER_KEYS}


# ---------------------------------------------------------------------------
# (c) Docs/architecture/INDEX.md table parsing -> packages map source paths
# ---------------------------------------------------------------------------

SEPARATOR_ROW_RE = re.compile(r"^\|[\s:-]+\|")
BACKTICK_SPAN_RE = re.compile(r"`([^`]+)`")
BRACKET_LINK_RE = re.compile(r"^\[([^\]]+)\]")
SOURCE_PATH_PREFIXES = (
    "internal/", "web/", "internal/", "cmd/", "tests/", "MCP/tool schemas", "tools/",
)


def _split_row(line: str) -> list:
    return [c.strip() for c in line.strip().strip("|").split("|")]


def _clean_subsystem_cell(cell: str) -> str:
    cell = cell.strip()
    m = BRACKET_LINK_RE.match(cell)
    name = m.group(1) if m else cell
    name = name.strip("`").strip()
    name = re.split(r"\s*\(", name)[0].strip()
    return name


def _extract_source_paths(cell: str) -> list:
    paths = []
    for m in BACKTICK_SPAN_RE.finditer(cell):
        val = m.group(1).strip()
        if any(val.startswith(p) for p in SOURCE_PATH_PREFIXES):
            paths.append(val)
    return paths


def parse_global_index_packages(text: str) -> dict:
    """Scan every markdown table in Docs/architecture/INDEX.md; for any table with both
    a Subsystem/Directory column and a Source[...] column, extract
    subsystem -> source-path(s). Tables without both columns (Proposed
    Subsystems, Meta/Derived) are silently skipped -- no source paths to
    extract there.

    First-writer-wins on path collisions: subsystem/tier tables appear
    before the Standalone & Virtual table in document order, so a virtual
    subsystem's cross-reference to an already-owned canonical path (e.g.
    causal's mention of internal/mangle/) never clobbers the canonical
    owner.
    """
    packages = {}
    lines = text.splitlines()
    n = len(lines)
    i = 0
    while i < n:
        line = lines[i]
        if line.strip().startswith("|") and i + 1 < n and SEPARATOR_ROW_RE.match(lines[i + 1]):
            header = _split_row(line)
            sub_idx = src_idx = None
            for idx, h in enumerate(header):
                hl = h.lower()
                if hl in ("subsystem", "directory"):
                    sub_idx = idx
                if hl.startswith("source"):
                    src_idx = idx
            if sub_idx is None or src_idx is None:
                i += 1
                continue
            j = i + 2
            while j < n and lines[j].strip().startswith("|"):
                row = _split_row(lines[j])
                if len(row) > max(sub_idx, src_idx):
                    sub_name = _clean_subsystem_cell(row[sub_idx])
                    for p in _extract_source_paths(row[src_idx]):
                        packages.setdefault(p, {"subsystem": sub_name, "features": []})
                j += 1
            i = j
            continue
        i += 1
    return packages


# ---------------------------------------------------------------------------
# Roadmap CSV loaders (cross-check inputs, not schema fields)
# ---------------------------------------------------------------------------


def load_tag_register(root: Path) -> dict:
    path = root / "docs" / "architecture" / "roadmap" / "31_feature_tag_register.csv"
    register = {}
    if not path.exists():
        return register
    with path.open(encoding="utf-8", newline="") as f:
        for row in csv.DictReader(f):
            fid = (row.get("feature_id") or "").strip()
            if fid:
                register[fid] = row
    return register


def load_feature_inventory_subsystems(root: Path) -> set:
    path = root / "docs" / "architecture" / "roadmap" / "21_feature_inventory.csv"
    subsystems = set()
    if not path.exists():
        return subsystems
    with path.open(encoding="utf-8", newline="") as f:
        for row in csv.DictReader(f):
            sub = (row.get("subsystem") or "").strip()
            if sub:
                subsystems.add(sub)
    return subsystems


# ---------------------------------------------------------------------------
# Index assembly
# ---------------------------------------------------------------------------


def doc_subsystem(root: Path, path: Path) -> str:
    rel = path.relative_to(root / "docs" / "architecture")
    parts = rel.parts
    if len(parts) <= 1:
        return "_root"
    return parts[0]


def build_index(root: Path):
    """Returns (data, discrepancies, diagnostics)."""
    corpus_dir = root / "docs" / "architecture"
    docs_out = {}
    features_out = {}
    duplicate_notes = []

    docs_total = 0
    docs_tagged = 0
    docs_frontmattered = 0

    for path in sorted(corpus_dir.rglob("*.md")):
        if "compiled" in path.parts:
            continue
        docs_total += 1
        rel = path.relative_to(root).as_posix()
        subsystem = doc_subsystem(root, path)

        try:
            text = path.read_text(encoding="utf-8", errors="replace")
        except OSError as exc:
            duplicate_notes.append(f"could not read {rel}: {exc}")
            continue

        tag_blocks = parse_tag_blocks(text) if should_scan_for_tags(path) else []
        frontmatter = parse_frontmatter(text) if should_scan_for_frontmatter(path) else {}

        if tag_blocks:
            docs_tagged += 1
        if frontmatter:
            docs_frontmattered += 1

        if not tag_blocks and not frontmatter:
            continue  # untagged/unframtmattered doc: by design, doesn't appear

        doc_feature_ids = []
        for fields, _line_no in tag_blocks:
            fid = fields["id"]
            doc_feature_ids.append(fid)
            if fid in features_out:
                if features_out[fid]["owner_doc"] != rel:
                    duplicate_notes.append(
                        f"duplicate feature id '{fid}': first seen in "
                        f"{features_out[fid]['owner_doc']}, also tagged in {rel}"
                    )
                continue
            features_out[fid] = {
                "topic": fields["topic"],
                "plane": fields["plane"],
                "status": fields["status"],
                "owner_doc": rel,
                "subsystem": subsystem,
                "source_paths": [],
            }

        entry = {
            "subsystem": subsystem,
            "features": sorted(set(doc_feature_ids)),
        }
        if frontmatter.get("doc-class"):
            entry["doc_class"] = frontmatter["doc-class"]
        if frontmatter.get("last-verified"):
            entry["last_verified"] = frontmatter["last-verified"]
        docs_out[rel] = entry

    # (c) Docs/architecture/INDEX.md -> packages map
    global_index_path = corpus_dir / "Docs/architecture/INDEX.md"
    packages_out = {}
    if global_index_path.exists():
        gi_text = global_index_path.read_text(encoding="utf-8", errors="replace")
        packages_out = parse_global_index_packages(gi_text)
    else:
        duplicate_notes.append("Docs/architecture/INDEX.md not found -- packages map will be empty")

    # Close the loop: packages <-> features, joined on subsystem name.
    for pkg_path, info in packages_out.items():
        sub = info["subsystem"]
        info["features"] = sorted(
            fid for fid, f in features_out.items() if f["subsystem"] == sub
        )
    for fid, f in features_out.items():
        sub = f["subsystem"]
        f["source_paths"] = sorted(
            p for p, info in packages_out.items() if info["subsystem"] == sub
        )

    data = {
        "generated": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "coverage": {
            "docs_total": docs_total,
            "docs_tagged": docs_tagged,
            "docs_frontmattered": docs_frontmattered,
        },
        "docs": docs_out,
        "features": features_out,
        "packages": packages_out,
    }

    # ---- cross-checks (report only; never fail the build) ----
    discrepancies = list(duplicate_notes)

    register = load_tag_register(root)
    doc_ids = set(features_out.keys())
    reg_ids = set(register.keys())
    for fid in sorted(doc_ids - reg_ids):
        discrepancies.append(
            f"tag '{fid}' found in docs (owner_doc={features_out[fid]['owner_doc']}) "
            f"but not present in 31_feature_tag_register.csv"
        )
    for fid in sorted(reg_ids - doc_ids):
        discrepancies.append(
            f"tag '{fid}' present in 31_feature_tag_register.csv "
            f"(owner_doc={register[fid].get('owner_doc')}) but not found in docs"
        )
    for fid in sorted(doc_ids & reg_ids):
        f = features_out[fid]
        r = register[fid]
        if f["topic"] != (r.get("canonical_topic") or ""):
            discrepancies.append(
                f"tag '{fid}' topic drift: docs='{f['topic']}' "
                f"csv='{r.get('canonical_topic')}'"
            )
        if f["plane"] != (r.get("state_plane") or ""):
            discrepancies.append(
                f"tag '{fid}' plane drift: docs='{f['plane']}' "
                f"csv='{r.get('state_plane')}'"
            )
        if f["status"] != (r.get("status") or ""):
            discrepancies.append(
                f"tag '{fid}' status drift: docs='{f['status']}' "
                f"csv='{r.get('status')}'"
            )
        reg_owner = (r.get("owner_doc") or "").replace("\\", "/")
        if reg_owner and reg_owner != f["owner_doc"]:
            discrepancies.append(
                f"tag '{fid}' owner_doc drift: docs='{f['owner_doc']}' csv='{reg_owner}'"
            )

    diagnostics = []
    inventory_subsystems = load_feature_inventory_subsystems(root)
    package_subsystems = {info["subsystem"] for info in packages_out.values()}
    missing_pkg = sorted(inventory_subsystems - package_subsystems)
    if missing_pkg:
        diagnostics.append(
            "subsystems in 21_feature_inventory.csv with no packages entry "
            f"from Docs/architecture/INDEX.md: {', '.join(missing_pkg)}"
        )

    return data, discrepancies, diagnostics


def render_json(data: dict) -> str:
    return json.dumps(data, indent=2, sort_keys=True) + "\n"


def diff_summary(committed: dict, fresh: dict) -> list:
    """Coarse structural diff for --check reporting (ignores 'generated')."""
    lines = []
    for key in ("coverage", "docs", "features", "packages"):
        c = committed.get(key)
        f = fresh.get(key)
        if c == f:
            continue
        if key == "coverage":
            lines.append(f"coverage drift: committed={c} fresh={f}")
            continue
        c_keys = set(c or {})
        f_keys = set(f or {})
        added = sorted(f_keys - c_keys)
        removed = sorted(c_keys - f_keys)
        changed = sorted(k for k in (c_keys & f_keys) if c[k] != f[k])
        if added:
            lines.append(f"{key}: +{len(added)} new (e.g. {added[:5]})")
        if removed:
            lines.append(f"{key}: -{len(removed)} removed (e.g. {removed[:5]})")
        if changed:
            lines.append(f"{key}: {len(changed)} changed (e.g. {changed[:5]})")
    return lines


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--check", action="store_true",
        help="Regenerate to memory, diff against the committed index, "
             "exit 1 on drift. Does not write the file.",
    )
    parser.add_argument(
        "--quiet", action="store_true",
        help="Suppress informational/diagnostic prints.",
    )
    args = parser.parse_args()

    root = find_project_root()
    output_path = root / "docs" / "architecture" / "roadmap" / "33_corpus_context_index.json"

    data, discrepancies, diagnostics = build_index(root)

    if discrepancies and not args.quiet:
        print(
            f"[build_tag_index] {len(discrepancies)} discrepancies found "
            f"(reported, not fatal):",
            file=sys.stderr,
        )
        for d in discrepancies[:200]:
            print(f"  - {d}", file=sys.stderr)
        if len(discrepancies) > 200:
            print(f"  ... and {len(discrepancies) - 200} more", file=sys.stderr)
    elif discrepancies:
        print(f"[build_tag_index] {len(discrepancies)} discrepancies (see --verbose)", file=sys.stderr)

    if diagnostics and not args.quiet:
        for d in diagnostics:
            print(f"[build_tag_index] note: {d}", file=sys.stderr)

    if args.check:
        if not output_path.exists():
            print(f"[build_tag_index] --check: {output_path} does not exist", file=sys.stderr)
            return 1
        try:
            committed = json.loads(output_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            print(f"[build_tag_index] --check: cannot parse committed index: {exc}", file=sys.stderr)
            return 1

        committed_cmp = dict(committed)
        fresh_cmp = dict(data)
        committed_cmp.pop("generated", None)
        fresh_cmp.pop("generated", None)

        if committed_cmp == fresh_cmp:
            if not args.quiet:
                print("[build_tag_index] --check: index up to date")
            return 0

        print("[build_tag_index] --check: DRIFT DETECTED between committed index and live corpus:", file=sys.stderr)
        for line in diff_summary(committed_cmp, fresh_cmp):
            print(f"  - {line}", file=sys.stderr)
        return 1

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(render_json(data), encoding="utf-8", newline="\n")

    if not args.quiet:
        cov = data["coverage"]
        print(
            f"[build_tag_index] wrote {output_path.relative_to(root).as_posix()}: "
            f"{cov['docs_total']} docs scanned, {cov['docs_tagged']} tagged, "
            f"{cov['docs_frontmattered']} frontmattered, "
            f"{len(data['features'])} features, {len(data['packages'])} packages"
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
