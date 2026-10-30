# Coder Shard Policy - Classification
# Description: Classify coding tasks to determine strategy and detect language.

# =============================================================================
# SECTION 2: TASK CLASSIFICATION
# =============================================================================

# REMOVED: a coder_task_complexity classification (simple / complex / critical)
# and its helpers. The live system derives a task's complexity per task, for
# expert selection (task_complexity/2 in shard_experts.mg, read by
# internal/experts), and a second 0-ary classification of the one current task
# said the same thing in another shape. The facts that made a change critical,
# a core file and many dependents, now reach the coder as impact guidance
# (coder_guidance.mg).

# -----------------------------------------------------------------------------
# 2.1 What each kind of change asks of the coder
# -----------------------------------------------------------------------------
# One sentence per coder action, told to the coder for the change in hand
# (coder_guidance.mg). The table is data, so a sentence is edited in one place.
# (A coder_strategy classification of the same actions had no reader and went.)
Decl coder_action_guidance(Action, Line) bound [/name, /string].
coder_action_guidance(/create, "A new file: keep it small and cohesive, and write its tests with it.").
coder_action_guidance(/modify, "Change the least that does the job and keep neighbouring behaviour intact.").
coder_action_guidance(/refactor, "Behaviour must not change: keep the tests green at every step.").
coder_action_guidance(/fix, "Reproduce the failure first, then make the smallest change that makes it pass.").
coder_action_guidance(/integrate, "Wire it at the existing boundary and delete the code it replaces.").
coder_action_guidance(/document, "Say what the code does now, not what it once did.").
