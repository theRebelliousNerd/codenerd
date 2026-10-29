"""Add subsystem tag to QA frontmatter based on component path references."""
import os
import re
import glob

QA_DIR = os.path.dirname(os.path.abspath(__file__))

# Mapping from filename keywords to internal/ subsystem
KEYWORD_MAP = {
    "intelligence_gatherer": "campaign",
    "tool_pregenerator": "campaign",
    "campaign_orchestrator": "campaign",
    "campaign_replanner": "campaign",
    "campaign_decomposer": "campaign",
    "decomposer": "campaign",
    "context_pager": "campaign",
    "orchestrator_task_handlers": "campaign",
    "orchestrator_failure": "campaign",
    "assault_tasks": "campaign",
    "risk_scoring": "campaign",
    "write_set_lock_manager": "campaign",
    "shard_advisory_board": "campaign",
    "replan": "campaign",
    "toolgen": "autopoiesis",
    "ouroboros": "autopoiesis",
    "thunderdome": "autopoiesis",
    "prompt_evolver": "autopoiesis",
    "dreamer": "core",
    "dream_plan": "core",
    "action_validator": "core",
    "kernel_validation": "core",
    "kernel_query": "core",
    "validator_exec": "core",
    "validator_syntax": "core",
    "validator_paranoid": "core",
    "virtual_store": "core",
    "tdd_loop": "core",
    "transaction_manager": "core",
    "api_scheduler": "core",
    "mangle_engine": "mangle",
    "understanding_adapter": "perception",
    "understanding_transducer": "perception",
    "semantic_classifier": "perception",
    "articulation": "articulation",
    "emitter": "articulation",
    "prompt_assembler": "prompt",
    "assembler": "prompt",
    "prompt_selector": "prompt",
    "selector": "prompt",
    "resolver": "prompt",
    "config_factory": "prompt",
    "token_budget": "prompt",
    "compiler": "prompt",
    "jit_compiler": "jit",
    "executor": "session",
    "task_executor": "session",
    "session_clean_loop": "session",
    "session_memory_compression": "session",
    "semantic_compressor": "session",
    "session_spawner": "session",
    "spawner": "session",
    "mcp_analyzer": "mcp",
    "mcp_store": "mcp",
    "local_graph": "store",
    "holographic": "world",
    "dataflow_extractor": "world",
    "world": "world",
    "dependency_resolver": "core",
    "edge_case_detector": "core",
    "northstar_guardian": "northstar",
    "atom_selector": "prompt",
    "tactile_legacy_executor": "tactile",
}

files = sorted(glob.glob(os.path.join(QA_DIR, "**", "*.md"), recursive=True))
updated = 0

for fpath in files:
    if "remediation" in fpath:
        continue
    fname = os.path.basename(fpath)
    if fname in ("JOURNAL.md", "add_frontmatter.py", "add_subsystem.py"):
        continue

    with open(fpath, "r", encoding="utf-8") as f:
        content = f.read()

    # Skip if already has subsystem
    if "subsystem:" in content[:200]:
        continue

    # Determine subsystem from filename
    fname_lower = fname.lower()
    subsystem = None
    # Try longest keyword match first
    for keyword in sorted(KEYWORD_MAP.keys(), key=len, reverse=True):
        if keyword in fname_lower:
            subsystem = KEYWORD_MAP[keyword]
            break

    # Fallback: scan content for internal/ path
    if not subsystem:
        match = re.search(r'internal/(\w+)/', content[:2000])
        if match:
            subsystem = match.group(1)

    if not subsystem:
        subsystem = "unknown"

    # Insert subsystem into existing frontmatter
    if content.startswith("---"):
        # Find closing ---
        end = content.index("---", 3)
        front = content[3:end].rstrip()
        rest = content[end:]
        new_content = f"---\n{front}\nsubsystem: {subsystem}\n{rest}"
    else:
        new_content = content

    with open(fpath, "w", encoding="utf-8") as f:
        f.write(new_content)

    updated += 1
    print(f"  [{subsystem:15s}] {fname}")

print(f"\nDone: {updated} files tagged")
