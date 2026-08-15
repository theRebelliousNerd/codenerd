# Regression Battery Policy (internal/regression)
#
# DECISION: /run_regression_battery is NOT a safe_action, and no VirtualStore
# handler routes it today.
#
# The reasoning, recorded here because the absence of a fact is invisible:
# a battery is a YAML file of shell commands, and the constitution's content
# gate (dangerous_content/2) can only see a pending_action's Target and Payload.
# A battery action's Target is a path. Writing files is already permitted
# (safe_action(/write_file)), so an agent granted /run_regression_battery as a
# safe_action could write `.nerd/regression/battery.yaml` containing "rm -rf /"
# or "git push --force" and run it — every blocked_pattern laundered through a
# file the gate never reads. That makes the battery strictly more powerful than
# /exec_cmd, which carries thirty content rules. An action that grants more
# than the thing it wraps does not belong on the allowlist.
#
# So the action is registered as requiring permission (below), which the
# constitution turns into dangerous_action/1 — default deny, and permitted/3
# only via signed_approval plus admin_override.
#
# That alone is not enough, because an override authorizes running *a* battery,
# not laundering a blocked command through one. The rules below make the
# battery's contents visible to the kernel: a host projects one
# regression_battery_task fact per task (internal/regression.PolicyFacts) and
# must query regression_battery_permitted/1 in ADDITION to permitted/3. Both
# have to hold. A host that skips the projection derives nothing and is denied,
# which is the correct failure direction.

requires_permission(/run_regression_battery).

# A task is forbidden when its command contains any constitutionally blocked
# pattern. Pattern is bound by blocked_pattern before :string:contains sees it,
# so both arguments are ground — the same shape the constitution's own
# dangerous_content rules use.
regression_task_forbidden(TaskID, Pattern) :-
    regression_battery_task(Path, TaskID, Command),
    blocked_pattern(Pattern),
    :string:contains(Command, Pattern).

# Bound-negation helpers (SECTION 11C in schemas_safety.mg): a negated literal
# containing an anonymous wildcard excludes nothing in this Mangle build, so the
# wildcards are projected away into single-argument helpers here.
regression_battery_has_task(Path) :-
    regression_battery_task(Path, TaskID, Command).

regression_battery_has_forbidden_task(Path) :-
    regression_battery_task(Path, TaskID, Command),
    regression_task_forbidden(TaskID, Pattern).

# The gate. An empty battery is refused rather than vacuously permitted: a suite
# that passes because it contains nothing is the worst possible signal, and the
# same policy the YAML loader enforces (Battery.Validate) has to hold here or
# the two disagree.
regression_battery_permitted(Path) :-
    regression_battery_declared(Path),
    regression_battery_has_task(Path),
    !regression_battery_has_forbidden_task(Path).

# Refusals are derived, not merely absent, so a host can tell the operator which
# of the two conditions failed.
regression_battery_refused(Path, "task command matches a blocked pattern") :-
    regression_battery_declared(Path),
    regression_battery_has_forbidden_task(Path).

regression_battery_refused(Path, "battery declares no tasks") :-
    regression_battery_declared(Path),
    !regression_battery_has_task(Path).
