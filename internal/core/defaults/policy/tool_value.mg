# Tool value: which tools earn their place, from measurements.
#
# A tool earns its place by doing one or more of three things:
#   1. Capability: it does something the harness cannot do any other way.
#   2. Condensation: it consumes far more material than it returns.
#   3. Token leverage: the tokens it costs (its prompt share, its calls and
#      results, the verification it makes necessary) are far fewer than the
#      work it does or saves.
# Go measures; these rules classify. The measurements are the tool ledger
# (autopoiesis.ProfileStore), replayed as the EDB facts below by
# autopoiesis.ToolLedgerFacts; the thresholds are config_param rows from the
# tool_value section of .nerd/config.json (config.ToolValueConfig.Params).
# Jev's readings of generated tools arrive as tool_value_evidence
# (jev_learning.mg) and are read here beside the measurements.
#
# Nothing here removes a tool. tool_retirement_proposal is a proposal: a
# generated tool may be proposed for removal through the work-mode approval
# (autopoiesis.Orchestrator.RetireGeneratedTool); a built-in is only ever
# flagged.

# --- measurements (EDB, from the ledger) ---

# tool_origin(Tool, Origin): Origin is /builtin or /generated.
Decl tool_origin(Tool, Origin) bound [/string, /name].
# tool_capability_declared(Tool, Class): the reviewed capability declaration,
# /unique (no other tool reaches this end) or /overlapping. A tool with no
# declaration has no fact, which the rules read as not unique.
Decl tool_capability_declared(Tool, Class) bound [/string, /name].
# tool_use_count(Tool, Calls, Sessions): calls the model made over every
# window the ledger covers, and the distinct sessions that made them. Asserted
# for every tool the ledger has seen, zero included.
Decl tool_use_count(Tool, Calls, Sessions) bound [/string, /number, /number].
# tool_offer_count(Tool, Offers, Sessions): compiles that offered the tool and
# the distinct sessions they belonged to.
Decl tool_offer_count(Tool, Offers, Sessions) bound [/string, /number, /number].
# tool_cost(Tool, PromptTokens, CallTokens, ResultTokens, VerifyTokens): tokens
# spent on the tool, summed over the ledger. Prompt is its catalog entry plus
# the atoms that teach it, paid on every compile that offered it whether or not
# it was called; call is the model-emitted call; result is what it returned;
# verify is the verification calls attributed to it.
Decl tool_cost(Tool, PromptTokens, CallTokens, ResultTokens, VerifyTokens) bound [/string, /number, /number, /number, /number].
# tool_condensation(Tool, KnownCalls, ConsumedBytes, ReturnedBytes, CallTokens):
# over only the calls whose tool reported what it scanned, the bytes consumed,
# the bytes returned, and the call and result tokens those calls cost. A tool that reported nothing has no fact: its condensation is unknown,
# and no rule reads an absent fact as a small number.
Decl tool_condensation(Tool, KnownCalls, ConsumedBytes, ReturnedBytes, CallTokens) bound [/string, /number, /number, /number, /number].
# tool_channel_use(Tool, Channel, Calls): calls by channel (/native,
# /piggyback, /generated, /mcp).
Decl tool_channel_use(Tool, Channel, Calls) bound [/string, /name, /number].

config_param_required(/tool_value, /tool_value_min_sessions).
config_param_required(/tool_value, /tool_value_min_calls).
config_param_required(/tool_value, /tool_value_min_condensation_percent).
config_param_required(/tool_value, /tool_value_min_leverage_percent).
config_param_required(/tool_value, /tool_value_dead_leverage_percent).
config_param_required(/tool_value, /tool_value_bytes_per_token).

# --- derived measures ---

Decl tool_known(Tool) bound [/string].
Decl tool_unique(Tool) bound [/string].
# tool_condensation_percent(Tool, Percent): consumed bytes as a percent of
# returned bytes (one is added to the divisor so a call that returned nothing
# is finite rather than undefined).
Decl tool_condensation_percent(Tool, Percent) bound [/string, /number].
# tool_leverage_percent(Tool, Percent): tokens of material consumed (bytes over
# the estimator's bytes-per-token) as a percent of the tokens spent: the whole
# prompt share plus the call and result tokens of the calls that reported a
# scan. Verification is in tool_cost, not here: it is charged to the write that
# needed it, and a write reports no scan.
Decl tool_leverage_percent(Tool, Percent) bound [/string, /number].
# tool_has_value_evidence(Tool): enough calls with a reported scan to judge
# condensation and leverage.
Decl tool_has_value_evidence(Tool) bound [/string].
Decl tool_misses(Tool) bound [/string].

tool_known(Tool) :- tool_use_count(Tool, Calls, Sessions).

tool_unique(Tool) :- tool_capability_declared(Tool, /unique).

tool_condensation_percent(Tool, Percent) :-
    tool_condensation(Tool, Known, Consumed, Returned, CallTokens),
    Known > 0,
    Percent = fn:div(fn:mult(Consumed, 100), fn:plus(Returned, 1)).

tool_leverage_percent(Tool, Percent) :-
    tool_condensation(Tool, Known, Consumed, Returned, CallTokens),
    tool_cost(Tool, Prompt, Call, Result, Verify),
    config_param(/tool_value_bytes_per_token, BytesPerToken),
    Known > 0,
    Work = fn:div(Consumed, BytesPerToken),
    Spent = fn:plus(fn:plus(Prompt, CallTokens), 1),
    Percent = fn:div(fn:mult(Work, 100), Spent).

tool_has_value_evidence(Tool) :-
    tool_condensation(Tool, Known, Consumed, Returned, CallTokens),
    config_param(/tool_value_min_calls, Min),
    Known >= Min.

# Jev read this tool's calls and every judged call missed what its input asked
# for (jev_learning.mg).
tool_misses(Tool) :- tool_value_evidence(Tool, /misses_mark).

# --- dead ---

# tool_dead_reason(Tool, Reason): why the tool is dead.
#   /never_called        offered in at least min_sessions sessions, never called.
#   /cost_exceeds_value  a tool with no unique capability, over min_calls
#                        measured calls, spent more tokens than the material it
#                        consumed was worth (leverage under dead_leverage_percent).
#   /redundant           Jev judged that another tool, the harness or one shell
#                        command already does what it does.
#   /misses_results      Jev judged every call it read a miss, and the tool has
#                        no unique capability to fall back on.
Decl tool_dead_reason(Tool, Reason) bound [/string, /name].
Decl tool_is_dead(Tool) bound [/string].

tool_dead_reason(Tool, /never_called) :-
    tool_use_count(Tool, 0, Used),
    tool_offer_count(Tool, Offers, Sessions),
    config_param(/tool_value_min_sessions, Min),
    Sessions >= Min.

tool_dead_reason(Tool, /cost_exceeds_value) :-
    tool_known(Tool),
    !tool_unique(Tool),
    tool_has_value_evidence(Tool),
    tool_leverage_percent(Tool, Leverage),
    config_param(/tool_value_dead_leverage_percent, Floor),
    Leverage < Floor.

tool_dead_reason(Tool, /redundant) :- tool_value_evidence(Tool, /duplicate).
tool_dead_reason(Tool, /redundant) :- tool_value_evidence(Tool, /harness_covers).

tool_dead_reason(Tool, /misses_results) :-
    tool_known(Tool),
    tool_misses(Tool),
    !tool_unique(Tool).

tool_is_dead(Tool) :- tool_dead_reason(Tool, Reason).

# --- earning ---

# A tool earns by capability (unique, and used at all) or by measurement
# (enough calls with a reported scan, condensing and levering past both bars).
Decl tool_is_earning(Tool) bound [/string].
Decl tool_earned(Tool) bound [/string].

tool_is_earning(Tool) :-
    tool_unique(Tool),
    tool_use_count(Tool, Calls, Sessions),
    Calls > 0.

tool_is_earning(Tool) :-
    tool_has_value_evidence(Tool),
    tool_condensation_percent(Tool, Condensation),
    tool_leverage_percent(Tool, Leverage),
    config_param(/tool_value_min_condensation_percent, MinCondensation),
    config_param(/tool_value_min_leverage_percent, MinLeverage),
    Condensation >= MinCondensation,
    Leverage >= MinLeverage.

# Dead outranks earning, and a tool whose judged calls all missed does not earn.
tool_earned(Tool) :-
    tool_is_earning(Tool),
    !tool_is_dead(Tool),
    !tool_misses(Tool).

# --- class ---

# tool_value_class(Tool, Class): /earning, /marginal, /dead or /unknown.
#   /dead      a dead reason derives; outranks everything.
#   /earning   earned (above) and not dead.
#   /marginal  measured or judged, but not earning and not dead: it was
#              used enough to measure, or Jev read its calls, and it did not
#              clear the bars.
#   /unknown   not enough evidence either way: too few sessions or calls, or a
#              tool that reports no scan to measure.
Decl tool_value_class(Tool, Class) bound [/string, /name].
Decl tool_has_signal(Tool) bound [/string].

tool_has_signal(Tool) :- tool_has_value_evidence(Tool).
tool_has_signal(Tool) :- tool_misses(Tool).

tool_value_class(Tool, /dead) :-
    tool_known(Tool),
    tool_is_dead(Tool).

tool_value_class(Tool, /earning) :-
    tool_known(Tool),
    tool_earned(Tool).

tool_value_class(Tool, /marginal) :-
    tool_known(Tool),
    tool_has_signal(Tool),
    !tool_is_dead(Tool),
    !tool_earned(Tool).

tool_value_class(Tool, /unknown) :-
    tool_known(Tool),
    !tool_has_signal(Tool),
    !tool_is_dead(Tool),
    !tool_earned(Tool).

# --- retirement ---

# tool_retirement_proposal(Tool, Kind, Reason): Kind is /retire for a
# generated tool (it may be proposed for removal through the work-mode
# approval) and /flag for a built-in (only ever flagged, never removed).
# Nothing retires on its own; a tool of unknown origin is neither.
Decl tool_retirement_proposal(Tool, Kind, Reason) bound [/string, /name, /name].

tool_retirement_proposal(Tool, /retire, Reason) :-
    tool_dead_reason(Tool, Reason),
    tool_origin(Tool, /generated).

tool_retirement_proposal(Tool, /flag, Reason) :-
    tool_dead_reason(Tool, Reason),
    tool_origin(Tool, /builtin).
