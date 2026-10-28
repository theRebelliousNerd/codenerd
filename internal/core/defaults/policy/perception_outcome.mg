# Classification outcome: what happened when perception tried to read a turn.
#
# Perception reports an observation per turn, never a lane: whether the model
# produced an admitted interpretation, and when it did not, at which stage and
# for which provider failure class. This file derives the disposition. A failed
# classification is not an interpretation: its zero-confidence /explain is a
# compatibility projection (perception.degradedClassificationIntent), and
# without this file the routing below it read that projection as the user's
# request, so a rejected credential or an unreadable answer continued as an
# ordinary answer turn, and a user's genuine ambiguity (intent_unknown,
# /heuristic_low) was the only reason ever derived for it.
#
# Model confidence stays apart from outcome validity: a low-confidence
# interpretation is /admitted (semantic uncertainty, clarification.mg); an
# infrastructure failure is never evidence the user was unclear.
#
# EDB (asserted by Go, retract-before-assert, perception.PublishClassificationOutcome):
#   classification_outcome(IntentID, Disposition, Stage, Class, Method)
#     Disposition: /admitted /invalid_response /provider_failed /canceled
#     Stage:       /none /transport /output /decode /required_fields
#     Class:       /none /transient /rate_limited /auth /invalid_request
#                  /permanent /canceled   (perception.ProviderFailureClass)
#     Method:      /generic /schema /free_form /none
#   A turn whose intent was not classified by a model (a delegated task's
#   preset, an empty input) has no row: no observation is not an admission.
#
# Derived:
#   classification_failed(IntentID)
#   classification_action(IntentID, Action)  /retry /blocker /canceled
#   classification_notice(IntentID, Text)    the words for the user
#   classification_stop(IntentID, Action, Text)
#                                            the turn ends here, with Text
#
# Jev's judgments are independent evidence and never a substitute for the
# classification: a Jev "yes" does not make a failed classification admitted.
# The one lane that needs no interpretation of the request itself is the
# go-ahead on held work (approve_lane, jev_evidence.mg), which asks only
# whether the message approves what is already held; it keeps its precedence
# below. Stopping background work (stop_background_requested) reads Jev alone
# and is derived whether or not classification succeeded.

Decl classification_outcome(IntentID, Disposition, Stage, Class, Method) bound [/name, /name, /name, /name, /name].
Decl classification_failed(IntentID) bound [/name].
Decl classification_action(IntentID, Action) bound [/name, /name].
Decl classification_notice(IntentID, Text) bound [/name, /string].
Decl classification_stop(IntentID, Action, Text) bound [/name, /name, /string].

classification_failed(Intent) :-
    classification_outcome(Intent, Disposition, _, _, _),
    /admitted != Disposition.

# The same request may succeed later: the provider was unavailable or rate
# limiting after the adapter's own retries (provider_failure.go), or the
# model's answer could not be read as a classification.
classification_known(Intent, /retry) :-
    classification_outcome(Intent, /provider_failed, _, /transient, _).
classification_known(Intent, /retry) :-
    classification_outcome(Intent, /provider_failed, _, /rate_limited, _).
classification_known(Intent, /retry) :-
    classification_outcome(Intent, /invalid_response, _, _, _).

# An unchanged retry cannot help: the credentials, the request or the
# configured provider must change first.
classification_known(Intent, /blocker) :-
    classification_outcome(Intent, /provider_failed, _, /auth, _).
classification_known(Intent, /blocker) :-
    classification_outcome(Intent, /provider_failed, _, /invalid_request, _).
classification_known(Intent, /blocker) :-
    classification_outcome(Intent, /provider_failed, _, /permanent, _).

classification_known(Intent, /canceled) :-
    classification_outcome(Intent, /canceled, _, _, _).

classification_classified(Intent) :-
    classification_known(Intent, _).

classification_action(Intent, Action) :-
    classification_known(Intent, Action).

# An outcome this file does not know is a blocker, never a success.
classification_action(Intent, /blocker) :-
    classification_failed(Intent),
    !classification_classified(Intent).

classification_notice(Intent, "I couldn't reach the language model just now - it may be briefly overloaded. Please try again in a moment.") :-
    classification_outcome(Intent, /provider_failed, _, /transient, _).
classification_notice(Intent, "The language model provider is rate limiting requests. Please try again shortly.") :-
    classification_outcome(Intent, /provider_failed, _, /rate_limited, _).
classification_notice(Intent, "The language model provider rejected the configured credentials, so I could not read your request. Check the API key in .nerd/config.json, then send it again.") :-
    classification_outcome(Intent, /provider_failed, _, /auth, _).
classification_notice(Intent, "The language model provider rejected the request to read your message. Check the configured model and its limits in .nerd/config.json, then send it again.") :-
    classification_outcome(Intent, /provider_failed, _, /invalid_request, _).
classification_notice(Intent, "The language model call failed. Check the configured provider and model in .nerd/config.json, then send your request again.") :-
    classification_outcome(Intent, /provider_failed, _, /permanent, _).
classification_notice(Intent, "The language model's reading of your request could not be understood, so I have not acted on it. Please send it again.") :-
    classification_outcome(Intent, /invalid_response, _, _, _).
classification_notice(Intent, "Reading your request was canceled before it finished, so I have not acted on it.") :-
    classification_outcome(Intent, /canceled, _, _, _).
classification_notice(Intent, "I could not read your request, so I have not acted on it. Please send it again.") :-
    classification_failed(Intent),
    !classification_classified(Intent).

# A failed classification ends the turn with its notice. Held work's go-ahead
# keeps its precedence: it asks only whether this message approves what is
# already held (jev_evidence.mg), and needs no interpretation of a request.
classification_stop(Intent, Action, Text) :-
    classification_action(Intent, Action),
    classification_notice(Intent, Text),
    !approve_lane().

# A user is told what failed rather than asked to restate a request that was
# never read: the executive asks, in interrogative mode, only for the cases a
# retry or a configuration change can settle. A canceled read is the user's
# own act and asks nothing.
next_action(/interrogative_mode) :-
    classification_action(_, /retry),
    !any_awaiting_clarification(/yes).
next_action(/interrogative_mode) :-
    classification_action(_, /blocker),
    !any_awaiting_clarification(/yes).
