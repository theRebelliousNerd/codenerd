# Coder Shard Policy - Workflow
#
# This file used to hold the whole coder workflow in one place: a state machine
# of next_coder_action rules over coder_state, the context rules (now
# coder_context.mg), the campaign rules (coder_campaign.mg) and observability.
#
# The state machine is retired. It stepped a CoderShard through read, generate,
# apply, build and test by asserting coder_state, and no such shard runs: the
# session executor loops in Go and judges a turn by its own evidence
# (turn_evidence and the turn_* verdict rules in coder_safety.mg). Nothing ever
# asserted coder_state, so no rule here could fire. What the coder is told and
# which edits it may not make are coder_guidance.mg and coder_edit_bar.mg.
#
# The file stays because policy configurations list it by name.

# Helper: file has content loaded (for safe negation)
Decl has_file_content(File).
has_file_content(File) :-
    file_content(File, _).
