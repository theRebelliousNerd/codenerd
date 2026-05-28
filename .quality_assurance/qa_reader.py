"""Dump all QA files for a given subsystem in one shot.

Usage: python qa_reader.py <subsystem> [--pending-only]

Examples:
  python qa_reader.py campaign
  python qa_reader.py core --pending-only
  python qa_reader.py --list
"""
import sys
import os
import glob
import re

# Fix Windows console encoding for emoji/unicode in QA docs
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

QA_DIR = os.path.dirname(os.path.abspath(__file__))


def parse_frontmatter(content):
    """Extract YAML frontmatter as a dict."""
    if not content.startswith("---"):
        return {}, content
    end = content.index("---", 3)
    raw = content[3:end].strip()
    body = content[end + 3:].lstrip("\n")
    meta = {}
    for line in raw.split("\n"):
        if ":" in line:
            k, v = line.split(":", 1)
            v = v.strip()
            if v == "true":
                v = True
            elif v == "false":
                v = False
            meta[k.strip()] = v
    return meta, body


def main():
    if len(sys.argv) < 2 or sys.argv[1] == "--help":
        print(__doc__)
        return

    files = sorted(glob.glob(os.path.join(QA_DIR, "**", "*.md"), recursive=True))
    all_files = []
    for fpath in files:
        if "remediation" in fpath:
            continue
        fname = os.path.basename(fpath)
        if fname in ("JOURNAL.md",):
            continue
        with open(fpath, "r", encoding="utf-8") as f:
            content = f.read()
        meta, body = parse_frontmatter(content)
        all_files.append((fname, meta, body, fpath))

    # --list mode: show subsystem summary
    if sys.argv[1] == "--list":
        buckets = {}
        for fname, meta, body, _ in all_files:
            sub = meta.get("subsystem", "unknown")
            rem = meta.get("remediated", False)
            buckets.setdefault(sub, {"total": 0, "remediated": 0, "pending": 0})
            buckets[sub]["total"] += 1
            if rem:
                buckets[sub]["remediated"] += 1
            else:
                buckets[sub]["pending"] += 1

        print(f"{'Subsystem':<20} {'Total':>5} {'Done':>5} {'Pending':>7}")
        print("-" * 40)
        for sub in sorted(buckets.keys(), key=lambda s: -buckets[s]["pending"]):
            b = buckets[sub]
            print(f"{sub:<20} {b['total']:>5} {b['remediated']:>5} {b['pending']:>7}")
        total = sum(b["total"] for b in buckets.values())
        done = sum(b["remediated"] for b in buckets.values())
        print("-" * 40)
        print(f"{'TOTAL':<20} {total:>5} {done:>5} {total - done:>7}")
        return

    subsystem = sys.argv[1].lower()
    pending_only = "--pending-only" in sys.argv

    matches = []
    for fname, meta, body, fpath in all_files:
        if meta.get("subsystem") != subsystem:
            continue
        if pending_only and meta.get("remediated", False):
            continue
        matches.append((fname, meta, body))

    if not matches:
        print(f"No QA files found for subsystem '{subsystem}'")
        if pending_only:
            print("(try without --pending-only)")
        return

    status = "pending " if pending_only else ""
    force = "--force" in sys.argv

    # Safeguard: if too many files, show list instead of dumping
    if len(matches) > 4 and not force:
        print(f"# {len(matches)} {status}QA files for subsystem: {subsystem}")
        print(f"# WARNING: That's a lot of content. Showing file list instead.\n")
        for fname, meta, body in matches:
            rem = "REMEDIATED" if meta.get("remediated") else "PENDING"
            print(f"  [{rem:10s}] {fname}")
        print(f"\nTo dump all content, re-run with --force")
        print(f"TIP: Tackle in batches of 4. Read specific files with:")
        print(f"  view_file <qa_dir>/<filename>")
        return

    print(f"# {len(matches)} {status}QA files for subsystem: {subsystem}\n")

    for fname, meta, body in matches:
        rem = "REMEDIATED" if meta.get("remediated") else "PENDING"
        print(f"{'=' * 80}")
        print(f"# FILE: {fname}  [{rem}]")
        print(f"{'=' * 80}")
        print(body)
        print()


if __name__ == "__main__":
    main()
