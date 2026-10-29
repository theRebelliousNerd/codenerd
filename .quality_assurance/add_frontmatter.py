"""Add YAML frontmatter to all QA boundary analysis files."""
import os
import glob
from datetime import datetime

QA_DIR = os.path.dirname(os.path.abspath(__file__))

# QAs remediated in this session (2026-05-12) based on recent commits
REMEDIATED = {
    # Commit 1200448d — intelligence gatherer blast radius
    "2026-03-26_05-30-EST_intelligence_gatherer_boundary_analysis.md": "2026-05-12",
    "2026-03-12_10-00-EST_tool_pregenerator_boundary_analysis.md": "2026-05-12",
    "2026-04-15_12-25-44-AM-EST_toolgen_boundary_analysis.md": "2026-05-12",
    # Commit 1200448d — also covered action_validator, validator_exec, validator_syntax
    "2026-04-01_00-26-00-EST_action_validator_boundary_analysis.md": "2026-05-12",
    "2026-04-27_04-14-58-AM-EST_validator_exec_boundary_analysis.md": "2026-05-12",
    "2026-05-09_04-30-00-EST_validator_syntax_boundary_analysis.md": "2026-05-12",
    "2026-05-08_12-25-23-AM-EST_validator_paranoid_boundary_analysis.md": "2026-05-12",
    "2026-04-02_00-16-08-EST_kernel_validation_boundary_analysis.md": "2026-05-12",
    # Commit 1200448d — understanding_adapter, semantic_classifier
    "2026-04-22_00-15-EST_understanding_adapter_boundary_analysis.md": "2026-05-12",
    "2026-04-23_22-33-55-EST_semantic_classifier_boundary_analysis.md": "2026-05-12",
    # Commit 6336fbcf — tdd loop
    "2026-05-12_04-26-23-AM-EST_tdd_loop_boundary_analysis.md": "2026-05-12",
}

files = sorted(glob.glob(os.path.join(QA_DIR, "**", "*.md"), recursive=True))
updated = 0
skipped = 0

for fpath in files:
    if "remediation" in fpath:
        continue
    fname = os.path.basename(fpath)
    if fname == "JOURNAL.md" or fname == "add_frontmatter.py":
        continue

    with open(fpath, "r", encoding="utf-8") as f:
        content = f.read()

    # Skip if already has frontmatter
    if content.startswith("---"):
        skipped += 1
        continue

    # Build frontmatter
    is_remediated = fname in REMEDIATED
    remediated_date = REMEDIATED.get(fname, "")

    frontmatter = "---\n"
    frontmatter += f"remediated: {'true' if is_remediated else 'false'}\n"
    if is_remediated:
        frontmatter += f"remediated_date: {remediated_date}\n"
    frontmatter += "---\n"

    with open(fpath, "w", encoding="utf-8") as f:
        f.write(frontmatter + content)

    updated += 1
    status = "REMEDIATED" if is_remediated else "pending"
    print(f"  [{status}] {fname}")

print(f"\nDone: {updated} files updated, {skipped} already had frontmatter")
