# Future Capability Mappings (Warning Fixes)
# Section 51 of Cortex Executive Policy


# Mappings for advanced actions supported by VirtualStore but not yet
# actively used by the core loop. These ensure all registered actions
# are reachable via policy, satisfying the action_linter.

# Code DOM & Analysis
next_action(/analyze_impact) :- user_intent(/current_intent, _, /analyze, _, "impact").
next_action(/search_code) :- user_intent(/current_intent, _, /search, _, "code").
next_action(/get_elements) :- user_intent(/current_intent, _, /get, _, "elements").
next_action(/get_element) :- user_intent(/current_intent, _, /get, _, "element").
next_action(/close_scope) :- user_intent(/current_intent, _, /close, "scope", _).

# Advanced File Operations
next_action(/read_file) :- user_intent(/current_intent, _, /read, _, "raw").
next_action(/write_file) :- user_intent(/current_intent, _, /write, _, "raw").
next_action(/delete_file) :- user_intent(/current_intent, _, /delete, _, "file").
next_action(/edit_file) :- user_intent(/current_intent, _, /edit, _, "file").
next_action(/edit_lines) :- user_intent(/current_intent, _, /edit, _, "lines").
next_action(/insert_lines) :- user_intent(/current_intent, _, /insert, _, "lines").
next_action(/delete_lines) :- user_intent(/current_intent, _, /delete, _, "lines").

# Workflow & Execution
next_action(/build_project) :- user_intent(/current_intent, _, /build, _, _).
next_action(/exec_tool) :- user_intent(/current_intent, _, /exec, _, "tool").
next_action(/git_operation) :- user_intent(/current_intent, _, /git, _, _).
next_action(/browse) :- user_intent(/current_intent, _, /browse, _, _).
next_action(/research) :- user_intent(/current_intent, _, /research, _, _).
next_action(/delegate) :- user_intent(/current_intent, _, /delegate, _, _).
next_action(/escalate) :- user_intent(/current_intent, _, /escalate, _, _).

# Python Environment
next_action(/python_env_setup) :- user_intent(/current_intent, _, /python, "setup", _).
next_action(/python_env_exec) :- user_intent(/current_intent, _, /python, "exec", _).
next_action(/python_run_pytest) :- user_intent(/current_intent, _, /python, "test", _).
next_action(/python_apply_patch) :- user_intent(/current_intent, _, /python, "patch", _).
next_action(/python_snapshot) :- user_intent(/current_intent, _, /python, "snapshot", _).
next_action(/python_restore) :- user_intent(/current_intent, _, /python, "restore", _).
next_action(/python_teardown) :- user_intent(/current_intent, _, /python, "teardown", _).

# SWE-bench Integration
next_action(/swebench_setup) :- user_intent(/current_intent, _, /swebench, "setup", _).
next_action(/swebench_apply_patch) :- user_intent(/current_intent, _, /swebench, "patch", _).
next_action(/swebench_run_tests) :- user_intent(/current_intent, _, /swebench, "test", _).
next_action(/swebench_snapshot) :- user_intent(/current_intent, _, /swebench, "snapshot", _).
next_action(/swebench_restore) :- user_intent(/current_intent, _, /swebench, "restore", _).
next_action(/swebench_evaluate) :- user_intent(/current_intent, _, /swebench, "evaluate", _).
next_action(/swebench_teardown) :- user_intent(/current_intent, _, /swebench, "teardown", _).
