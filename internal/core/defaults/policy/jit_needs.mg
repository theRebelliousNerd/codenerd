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
# constant (`target_need(/mangle, Need)`; `/undetected` when no language is
# known), and copies the answer into the compilation context. Go does not
# decide a need; this file does.
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

# Writing Go: serve the Go failure modes an agent reliably commits (loop
# capture, nil interfaces, defer, append, map races, error shortcuts, goroutine
# leaks) and the Go-only hallucination guards (suppressed errors, closure
# scope, deprecated idioms).
#
# Measured 2026-09-22 (context sweep, C4): these eleven atoms were
# `is_mandatory` with `languages: [/go]`, so every coder compile whose language
# was /go carried ~4.7k tokens of them. The language key alone is situational
# in jit_compiler.mg -- a compile with no language admits every language-tagged
# atom -- so the language kept them out of a Markdown turn only after .md got a
# row in coder_language.mg (c42f2e48), and a compile whose language nothing
# could tell got them all. The need is fail-closed: they are served when, and
# only when, the compile is aimed at Go.
target_need(/go, /authoring_go).

# Writing code: serve the guidance that exists because the turn produces
# source -- a mental compile check, self-correction before emitting code, the
# 7-phase coder protocol, the test-coverage mandate and the TDD rationale,
# import/API fabrication, scratch programs, AI-code debugging. A turn aimed at
# prose or data (Markdown, reStructuredText, AsciiDoc, text, YAML, JSON) writes
# no code; measured 2026-09-22 those atoms rode every round of a
# documentation campaign's tasks (coverage mandate alone ~570 tokens).
Decl code_language(Language) bound [/name].

code_language(/go).
code_language(/python).
code_language(/typescript).
code_language(/javascript).
code_language(/rust).
code_language(/java).
code_language(/csharp).
code_language(/ruby).
code_language(/php).
code_language(/cpp).
code_language(/c).
code_language(/kotlin).
code_language(/swift).
code_language(/mangle).
code_language(/sql).
code_language(/shell).
code_language(/powershell).

target_need(Language, /authoring_code) :- code_language(Language).

# A compile whose language nothing could tell -- no project language and a
# target without a known extension -- is asked as /undetected. It may be
# writing code, and the coverage mandate missing from a code turn costs more
# than ~2k tokens of code guidance on a prose turn, so it gets the
# code-authoring need; no language's own corpus (/authoring_go, ...) is
# predicted for it.
target_need(/undetected, /authoring_code).

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
