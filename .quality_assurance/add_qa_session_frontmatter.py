"""Add YAML frontmatter to QA docs remediated in the 2026-05-28 session."""
import os

QA_DIR = os.path.dirname(os.path.abspath(__file__))
REMEDIATED_DATE = "2026-05-28"

# docs remediated this session, mapped to their subsystem
REMEDIATED = {
    "campaign/2026_05_23_decomposer_boundary_analysis.md": "campaign",
    "campaign/2026-05-24_04-26-EST_shard_advisory_board_boundary_analysis.md": "campaign",
    "core/2026-05-13_04-09-10-AM-EST_shadow_mode_boundary_analysis.md": "core",
    "core/2026-05-16_11-25-30-PM_EST_rule_court_boundary_analysis.md": "core",
    "core/2026-05-20_04-09-49-EST_self_healing_boundary_analysis.md": "core",
    "core/2026-05-22_00-21-EST_diff_boundary_analysis.md": "core",
    "core/2026-05-24_23-17-49-EST_diff_boundary_analysis.md": "core",
    "autopoiesis/2026-05-15_12-23-22-AM-EST_feedback_boundary_analysis.md": "autopoiesis",
    "perception/2026-05-16_00-21-45-EST_sparse_retriever_boundary_analysis.md": "perception",
    "perception/2026-05-18_11-05-48-PM-EST_transducer_llm_boundary_analysis.md": "perception",
    "prompt/2026-05-18_04-13-04-AM-EST_prompt_atoms_boundary_analysis.md": "prompt",
    "prompt/2026-05-27_00-09-54-EST_token_budget_manager_boundary_analysis.md": "prompt",
    "session/2026-05-13_04-04-24-EST_session_executor_clean_loop_integration_analysis.md": "session",
    "session/2026-05-21_04-07-57-AM-EST_session_executor_boundary_analysis.md": "session",
    "mcp/2026-05-14_05-00-00-AM-EST_mcp_client_manager_boundary_analysis.md": "mcp",
    "mcp/2026-05-21_04-08-52-AM-EST_mcp_client_manager_boundary_analysis.md": "mcp",
    "store/2026-05-26_tools_registry_boundary_analysis.md": "store",
}

updated = 0
for rel, subsystem in REMEDIATED.items():
    fpath = os.path.join(QA_DIR, rel)
    if not os.path.exists(fpath):
        print(f"  [MISSING] {rel}")
        continue
    with open(fpath, "r", encoding="utf-8") as f:
        content = f.read()
    if content.startswith("---"):
        # Already has frontmatter — replace
        end = content.index("---", 3)
        body = content[end + 3:].lstrip("\n")
    else:
        body = content
    frontmatter = (
        "---\n\n"
        "remediated: true\n"
        f"remediated_date: {REMEDIATED_DATE}\n"
        f"subsystem: {subsystem}\n"
        "---\n"
    )
    with open(fpath, "w", encoding="utf-8") as f:
        f.write(frontmatter + body)
    updated += 1
    print(f"  [REMEDIATED] {rel}")

print(f"\nDone: {updated} files updated.")
