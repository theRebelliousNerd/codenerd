# Future Capability Mappings (Warning Fixes)
# Section 51 of Cortex Executive Policy


# Mappings for advanced actions supported by VirtualStore but not yet
# actively used by the core loop. These ensure all registered actions
# are reachable via policy, satisfying the action_linter.

# Code DOM & Analysis
# Guard: Only derive if intent hasn't been processed by executive (prevents infinite loop)
next_action(/analyze_impact) :- user_intent(/current_intent, _, /analyze, _, "impact"), !executive_processed_intent(/current_intent).
next_action(/search_code) :- user_intent(/current_intent, _, /search, _, "code"), !executive_processed_intent(/current_intent).
next_action(/get_elements) :- user_intent(/current_intent, _, /get, _, "elements"), !executive_processed_intent(/current_intent).
next_action(/get_element) :- user_intent(/current_intent, _, /get, _, "element"), !executive_processed_intent(/current_intent).
next_action(/close_scope) :- user_intent(/current_intent, _, /close, "scope", _), !executive_processed_intent(/current_intent).

# Advanced File Operations
next_action(/read_file) :- user_intent(/current_intent, _, /read, _, "raw"), !executive_processed_intent(/current_intent).
next_action(/write_file) :- user_intent(/current_intent, _, /write, _, "raw"), !executive_processed_intent(/current_intent).
next_action(/delete_file) :- user_intent(/current_intent, _, /delete, _, "file"), !executive_processed_intent(/current_intent).
next_action(/edit_file) :- user_intent(/current_intent, _, /edit, _, "file"), !executive_processed_intent(/current_intent).
next_action(/edit_lines) :- user_intent(/current_intent, _, /edit, _, "lines"), !executive_processed_intent(/current_intent).
next_action(/insert_lines) :- user_intent(/current_intent, _, /insert, _, "lines"), !executive_processed_intent(/current_intent).
next_action(/delete_lines) :- user_intent(/current_intent, _, /delete, _, "lines"), !executive_processed_intent(/current_intent).

# Workflow & Execution
next_action(/build_project) :- user_intent(/current_intent, _, /build, _, _), !executive_processed_intent(/current_intent).
next_action(/exec_tool) :- user_intent(/current_intent, _, /exec, _, "tool"), !executive_processed_intent(/current_intent).
next_action(/git_operation) :- user_intent(/current_intent, _, /git, _, _), !executive_processed_intent(/current_intent).
next_action(/browse) :- user_intent(/current_intent, _, /browse, _, _), !executive_processed_intent(/current_intent).
next_action(/research) :- user_intent(/current_intent, _, /research, _, _), !executive_processed_intent(/current_intent).
next_action(/delegate) :- user_intent(/current_intent, _, /delegate, _, _), !executive_processed_intent(/current_intent).
next_action(/escalate) :- user_intent(/current_intent, _, /escalate, _, _), !executive_processed_intent(/current_intent).

# Python Environment
next_action(/python_env_setup) :- user_intent(/current_intent, _, /python, "setup", _), !executive_processed_intent(/current_intent).
next_action(/python_env_exec) :- user_intent(/current_intent, _, /python, "exec", _), !executive_processed_intent(/current_intent).
next_action(/python_run_pytest) :- user_intent(/current_intent, _, /python, "test", _), !executive_processed_intent(/current_intent).
next_action(/python_apply_patch) :- user_intent(/current_intent, _, /python, "patch", _), !executive_processed_intent(/current_intent).
next_action(/python_snapshot) :- user_intent(/current_intent, _, /python, "snapshot", _), !executive_processed_intent(/current_intent).
next_action(/python_restore) :- user_intent(/current_intent, _, /python, "restore", _), !executive_processed_intent(/current_intent).
next_action(/python_teardown) :- user_intent(/current_intent, _, /python, "teardown", _), !executive_processed_intent(/current_intent).

# SWE-bench Integration
next_action(/swebench_setup) :- user_intent(/current_intent, _, /swebench, "setup", _), !executive_processed_intent(/current_intent).
next_action(/swebench_apply_patch) :- user_intent(/current_intent, _, /swebench, "patch", _), !executive_processed_intent(/current_intent).
next_action(/swebench_run_tests) :- user_intent(/current_intent, _, /swebench, "test", _), !executive_processed_intent(/current_intent).
next_action(/swebench_snapshot) :- user_intent(/current_intent, _, /swebench, "snapshot", _), !executive_processed_intent(/current_intent).
next_action(/swebench_restore) :- user_intent(/current_intent, _, /swebench, "restore", _), !executive_processed_intent(/current_intent).
next_action(/swebench_evaluate) :- user_intent(/current_intent, _, /swebench, "evaluate", _), !executive_processed_intent(/current_intent).
next_action(/swebench_teardown) :- user_intent(/current_intent, _, /swebench, "teardown", _), !executive_processed_intent(/current_intent).
