# Browser Physics
# Section 9 of Cortex Executive Policy


# Spatial reasoning - element to the left (constrained to interactable elements to avoid O(N²))
left_of(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, Ax, _, _, _),
    geometry(B, Bx, _, _, _),
    Ax < Bx.

# Element above another (constrained to interactable elements)
above(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, _, Ay, _, _),
    geometry(B, _, By, _, _),
    Ay < By.

# Honeypot detection via CSS properties
honeypot_detected(ID) :-
    computed_style(ID, "display", "none").

honeypot_detected(ID) :-
    computed_style(ID, "visibility", "hidden").

honeypot_detected(ID) :-
    computed_style(ID, "opacity", "0").

honeypot_detected(ID) :-
    geometry(ID, _, _, 0, _).

honeypot_detected(ID) :-
    geometry(ID, _, _, _, 0).

# Safe interactive elements (not honeypots)
safe_interactable(ID) :-
    interactable(ID, _),
    !honeypot_detected(ID).

# Target checkbox to the left of label text.
#
# Tag names, attribute names and attribute values are /string, not atoms:
# dom_node and attribute are both bound [/string, ...] and are fed straight from
# the page by SessionManager.buildDOMFacts. The atom form these three literals
# used to carry could never unify with a live fact.
target_checkbox(CheckID, LabelText) :-
    dom_node(CheckID, "input", _, _),
    attribute(CheckID, "type", "checkbox"),
    dom_text(TextID, LabelText),
    left_of(CheckID, TextID).

# Session-scoped browser diagnosis. These rules operate in the same live
# Cortex that authorizes tools; there is no private browser reasoning engine.
failed_request(SessionID, ReqID, URL, Status) :-
    net_request(SessionID, ReqID, _, URL, _, _),
    net_response(SessionID, ReqID, Status, _, _),
    Status >= 400.

failed_request_at(SessionID, ReqID, URL, Status, Timestamp) :-
    net_request(SessionID, ReqID, _, URL, _, Timestamp),
    net_response(SessionID, ReqID, Status, _, _),
    Status >= 400.

slow_api(SessionID, ReqID, URL, Duration) :-
    net_request(SessionID, ReqID, _, URL, _, _),
    net_response(SessionID, ReqID, _, _, Duration),
    Duration >= 1000.

slow_api_at(SessionID, ReqID, URL, Duration, Timestamp) :-
    net_request(SessionID, ReqID, _, URL, _, Timestamp),
    net_response(SessionID, ReqID, _, _, Duration),
    Duration >= 1000.

root_cause(SessionID, URL, "network", "http_status") :-
    failed_request(SessionID, _, URL, _).
root_cause_at(SessionID, URL, "network", "http_status", Timestamp) :-
    failed_request_at(SessionID, _, URL, _, Timestamp).

root_cause(SessionID, Message, "console", "console_error") :-
    console_event(SessionID, "error", Message, _).
root_cause_at(SessionID, Message, "console", "console_error", Timestamp) :-
    console_event(SessionID, "error", Message, Timestamp).
root_cause(SessionID, ErrorText, "network", "request_failed") :-
    net_failure(SessionID, _, ErrorText, _, _).
root_cause_at(SessionID, ErrorText, "network", "request_failed", Timestamp) :-
    net_failure(SessionID, _, ErrorText, _, Timestamp).

user_visible_error(SessionID, "console", Message, Timestamp) :-
    console_event(SessionID, "error", Message, Timestamp).
user_visible_error(SessionID, "toast", Message, Timestamp) :-
    toast_notification(SessionID, Message, "error", _, Timestamp).

interaction_blocked(SessionID, "modal_or_dialog") :-
    browser_page_state(SessionID, _, _, /true, _).
interaction_blocked_at(SessionID, "modal_or_dialog", Timestamp) :-
    browser_page_state(SessionID, _, _, /true, Timestamp).

# A control that needs constitutional approval is a hazard by derivation:
# Go asserts what it found, logic decides what that means.
audit_hazard(S, Subject) :-
    audit_finding(S, "approval_required", Subject, _, _).

