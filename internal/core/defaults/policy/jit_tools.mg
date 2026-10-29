# JIT tools: which of the persona's tools a turn does not get.
#
# A persona's envelope (internal/prompt/config_factory.go, per verb) is the
# ceiling on what a turn may call. It is a static list, so every turn of a
# persona carried every schema in it: measured 2026-09-22 on 481 tool-loop
# rounds, the coder's 31 tools cost ~5.3k tokens a round and 15 of them were
# never called. This file narrows the envelope for the turn at hand; nothing
# here can add a tool, and every call still has to derive permitted(...).
#
# The session executor measures the turn at its start -- the target's
# language, and what the turn lacks -- asks the two questions below with the
# measurement bound as a constant, and removes what they name from the
# envelope, for the compile (the atoms gated on requires_tools follow) and for
# the tool loop (the catalog and the allowlist). The catalog is decided once
# per turn and held: the provider caches the tools first, so a catalog that
# changes mid-turn re-bills the whole prefix. The mid-turn regime narrowing
# (working_set.mg: /commit, working_search_open) is a separate decision and
# stays where it is.

# --- What the target is ------------------------------------------------------

# A turn aimed at prose builds nothing and has no impact graph: the build and
# impacted-test tools act on a code build. run_tests stays -- the project may
# require a test run before any work is declared complete.
Decl target_withholds_tool(Language, Tool) bound [/name, /name].

Decl prose_language(Language) bound [/name].

prose_language(/markdown).
prose_language(/restructuredtext).
prose_language(/asciidoc).
prose_language(/text).

Decl code_build_tool(Tool) bound [/name].

code_build_tool(/run_build).
code_build_tool(/get_impacted_tests).
code_build_tool(/run_impacted_tests).

target_withholds_tool(Language, Tool) :-
    prose_language(Language),
    code_build_tool(Tool).

# --- What the turn lacks -----------------------------------------------------

# A tool that exists for something only some turns have. The executor names
# each thing the turn lacks (a constant) and asks absence_withholds_tool.
#
# /mcp_server: no MCP server is registered, so the five control-plane verbs
# have nothing to map, probe, call, expand or read.
# /subagent_return_handle: the turn's input carries no subagent-return handle
# (obs:sa:...), so subagent_expand has nothing to redeem. Such a handle is
# minted by a delegation before the turn that reads it, never inside it, so
# the turn-start measurement cannot miss one. (search_expand is not here: its
# handles are minted mid-turn by search_code, and it rides with search_code.)
Decl tool_requires_presence(Tool, Presence) bound [/name, /name].

tool_requires_presence(/mcp_map, /mcp_server).
tool_requires_presence(/mcp_probe, /mcp_server).
tool_requires_presence(/mcp_call, /mcp_server).
tool_requires_presence(/mcp_expand, /mcp_server).
tool_requires_presence(/mcp_context, /mcp_server).
tool_requires_presence(/subagent_expand, /subagent_return_handle).

Decl absence_withholds_tool(Absent, Tool) bound [/name, /name].

absence_withholds_tool(Absent, Tool) :-
    tool_requires_presence(Tool, Absent).
