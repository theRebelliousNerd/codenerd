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

# What reads the answer. A protocol atom that teaches a Piggyback envelope field
# is gated on the need for it, so a compile gets it only when the code that will
# read the answer acts on that field. Go names the consumer -- a constant: the
# path that will read this compile's answer -- and asks consumer_need(Consumer,
# Need); this file decides.
#
# Measured 2026-09-23 (CTX-B7): the tool and knowledge request protocols, and
# the knowledge-discovery atom that says "do not say you don't know -- ask a
# specialist", were mandatory with no selector, so every compile carried them.
# Envelope tool_requests run only on the session executor's text channel
# (generateResponseWithPiggybackTools, for a client that answers
# ShouldUsePiggybackTools; the native channel's promotePiggybackToolRequests is
# a safety net, not a protocol to teach). Envelope knowledge_requests run only
# in the chat turn's articulation (cmd/nerd/chat/process.go). A delegated shard
# on a native channel was taught both, and told to ask specialists nothing
# would ever consult.
Decl consumer_need(Consumer, Need) bound [/name, /name].

consumer_need(/executor_text_channel, /envelope_tool_requests).
consumer_need(/chat_articulation, /envelope_knowledge_requests).
