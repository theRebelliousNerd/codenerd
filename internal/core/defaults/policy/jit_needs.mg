# JIT needs: what the next prompt compile will need, derived by the kernel.
#
# The JIT serves an atom gated on a world state (`world_states: [...]` in its
# YAML) exactly when that state holds for the compile, and never otherwise:
# the selector treats those gates as fail-closed. The MEASURED states
# (failing_tests, diagnostics, no_tool_call_retry, ...) come from what the
# executor observed. The states here are PREDICTED: what a compile aimed at a
# given kind of file is going to need before the model has to go looking for
# it. The session executor asks at every compile boundary -- the turn, and each
# planned step when it starts -- with the target's language bound as a
# constant (`target_need(/mangle, Need)`), and copies the answer into the
# compilation context. Go does not decide a need; this file does.
#
# Observed 2026-09-18: a coder compile aimed at a .mg file (language /mangle)
# selected 41 atoms and not one of the corpus's 119 /mangle atoms. Language
# only unblocks them; a vector hit is the only thing that admits one, and
# nothing predicted that a step editing policy needs the engine's rules in
# front of it before it writes the first clause.
#
# Keep each need narrow: it admits atoms into the skeleton on every compile it
# holds for, so it is paid in tokens every time. The rest of a language's
# corpus stays relevance-admitted (vector search) once the language unblocks it.

Decl target_need(Language, Need) bound [/name, /name].

# Writing Mangle: serve the language core and the verified engine truths
# (language/mangle/core, language/mangle/engine_truths_pinned).
target_need(/mangle, /authoring_mangle).
