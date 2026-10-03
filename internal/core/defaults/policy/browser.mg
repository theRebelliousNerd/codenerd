# Browser Physics
# Section 9 of Cortex Executive Policy
# Capture, action ranking and diagnosis are adapted from BrowserNERD's
# Apache-2.0 browser contract; the live Cortex remains the executive.


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
config_param_required(/browser, /browser_slow_api_ms).
config_param_required(/browser, /browser_causal_bucket_ms).
config_param_required(/browser, /browser_unhydrated_age_ms).
config_param_required(/browser, /browser_attended_navigation_window_ms).
config_param_required(/browser, /browser_priority_primary).

asset_resource_type("Image").
asset_resource_type("Font").
asset_resource_type("Stylesheet").
asset_resource_type("Media").
asset_resource_type("Manifest").
asset_resource_type("TextTrack").
asset_resource_type("Other").

asset_http_error(S, R) :-
    net_http_error(S, R, _, _, Type, _),
    asset_resource_type(Type).

failed_request_done(S, R, URL, Status, T) :-
    net_http_error(S, R, URL, Status, _, T),
    Status >= 500.
failed_request_done(S, R, URL, Status, T) :-
    net_http_error(S, R, URL, Status, Type, T),
    Status >= 400,
    Status < 500,
    !asset_resource_type(Type).

failed_request(S, R, URL, Status) :-
    failed_request_done(S, R, URL, Status, _).
failed_request(SessionID, ReqID, URL, Status) :-
    net_response(SessionID, ReqID, Status, _, _),
    Status >= 500,
    net_request(SessionID, ReqID, _, URL, _, _).
failed_request(SessionID, ReqID, URL, Status) :-
    net_response(SessionID, ReqID, Status, _, _),
    Status >= 400,
    Status < 500,
    net_request(SessionID, ReqID, _, URL, _, _),
    !asset_http_error(SessionID, ReqID).

failed_request_at(SessionID, ReqID, URL, Status, Timestamp) :-
    net_request(SessionID, ReqID, _, URL, _, Timestamp),
    failed_request(SessionID, ReqID, URL, Status).

# Retention may remove the request while keeping its failure witness.
browser_has_request(S, R) :- net_request(S, R, _, _, _, _).
failed_request_at(S, R, URL, Status, T) :-
    failed_request_done(S, R, URL, Status, T),
    !browser_has_request(S, R).

browser_has_loading_failure(S, R) :- net_loading_failed(S, R, _, _, _).
network_failure(S, R, URL, ErrorText, T) :-
    net_loading_failed(S, R, ErrorText, "false", T),
    net_request(S, R, _, URL, _, _).
# A retained transport failure must survive eviction of its request metadata.
network_failure(S, R, "", ErrorText, T) :-
    net_loading_failed(S, R, ErrorText, "false", T),
    !browser_has_request(S, R).
network_failure(S, R, URL, ErrorText, T) :-
    net_failure(S, R, ErrorText, _, T),
    ErrorText != "net::ERR_ABORTED",
    net_request(S, R, _, URL, _, _),
    !browser_has_loading_failure(S, R).

slow_api(SessionID, ReqID, URL, Duration) :-
    config_param(/browser_slow_api_ms, Threshold),
    net_response(SessionID, ReqID, _, _, Duration),
    Duration >= Threshold,
    net_request(SessionID, ReqID, _, URL, _, _).

slow_api_at(SessionID, ReqID, URL, Duration, Timestamp) :-
    net_request(SessionID, ReqID, _, URL, _, Timestamp),
    slow_api(SessionID, ReqID, URL, Duration).

# Buckets bound the join to nearby events before the time comparison runs.
console_error_bucket(S, Bucket, Message, T) :-
    config_param(/browser_causal_bucket_ms, Width),
    Width > 0,
    console_event(S, "error", Message, T),
    Bucket = fn:div(T, Width).
request_failure_bucket(S, Bucket, R, T) :-
    config_param(/browser_causal_bucket_ms, Width),
    Width > 0,
    failed_request_done(S, R, _, _, T),
    Bucket = fn:div(T, Width).
request_failure_bucket(S, Bucket, R, T) :-
    config_param(/browser_causal_bucket_ms, Width),
    Width > 0,
    network_failure(S, R, _, _, T),
    Bucket = fn:div(T, Width).

caused_by_candidate(S, ErrorTs, Message, R, FailTs) :-
    console_error_bucket(S, Bucket, Message, ErrorTs),
    request_failure_bucket(S, Bucket, R, FailTs),
    FailTs <= ErrorTs,
    Delta = fn:minus(ErrorTs, FailTs),
    config_param(/browser_causal_bucket_ms, Width),
    Delta < Width.
caused_by_candidate(S, ErrorTs, Message, R, FailTs) :-
    console_error_bucket(S, Bucket, Message, ErrorTs),
    Previous = fn:minus(Bucket, 1),
    request_failure_bucket(S, Previous, R, FailTs),
    FailTs <= ErrorTs,
    Delta = fn:minus(ErrorTs, FailTs),
    config_param(/browser_causal_bucket_ms, Width),
    Delta < Width.
caused_by_nearest(S, ErrorTs, Message, Nearest) :-
    caused_by_candidate(S, ErrorTs, Message, R, FailTs)
    |> do fn:group_by(S, ErrorTs, Message), let Nearest = fn:max(FailTs).
caused_by(S, Message, R) :-
    caused_by_nearest(S, ErrorTs, Message, FailTs),
    caused_by_candidate(S, ErrorTs, Message, R, FailTs).

browser_console_cause(S, Message, ErrorTs) :-
    caused_by_nearest(S, ErrorTs, Message, _).
root_cause_at(S, Message, "network", URL, ErrorTs) :-
    caused_by_nearest(S, ErrorTs, Message, FailTs),
    caused_by_candidate(S, ErrorTs, Message, R, FailTs),
    failed_request_done(S, R, URL, _, FailTs).
root_cause_at(S, Message, "network", ErrorText, ErrorTs) :-
    caused_by_nearest(S, ErrorTs, Message, FailTs),
    caused_by_candidate(S, ErrorTs, Message, R, FailTs),
    network_failure(S, R, _, ErrorText, FailTs).
root_cause_at(SessionID, URL, "network", "http_status", Timestamp) :-
    failed_request_at(SessionID, _, URL, _, Timestamp).
root_cause_at(SessionID, Message, "console", "console_error", Timestamp) :-
    console_event(SessionID, "error", Message, Timestamp),
    !browser_console_cause(SessionID, Message, Timestamp).
root_cause_at(SessionID, ErrorText, "network", "request_failed", Timestamp) :-
    network_failure(SessionID, _, _, ErrorText, Timestamp).
root_cause_at(S, URL, "navigation", ErrorText, T) :-
    page_load_failed(S, URL, ErrorText, T).
root_cause_at(S, URL, "websocket", Detail, T) :-
    ws_event(S, _, URL, "error", Detail, T).
root_cause_at(S, Text, "browser", Source, T) :-
    browser_log(S, Source, "error", Text, _, T).
root_cause(S, Message, Source, Cause) :-
    root_cause_at(S, Message, Source, Cause, _).

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

# A session clock is measured, never an inferred wall-clock deadline for a run.
browser_clock_sample(S, T) :- browser_observed_at(S, T).
browser_clock_sample(S, T) :- browser_page_state(S, _, _, _, T).
browser_clock_sample(S, T) :- attended(S, T).
browser_clock_sample(S, T) :- act_batch(S, _, T).
browser_clock_sample(S, T) :- navigation_event(S, _, T).
browser_clock_sample(S, T) :- console_event(S, _, _, T).
browser_clock_sample(S, T) :- net_http_error(S, _, _, _, _, T).
browser_session_clock(S, Now) :-
    browser_clock_sample(S, T)
    |> do fn:group_by(S), let Now = fn:max(T).

auth_expired(S, Store, Key) :-
    storage_entry(S, Store, Key, _, Exp),
    Exp > 0,
    browser_session_clock(S, Now),
    Exp <= Now.
auth_failed_request(S, R, Store, Key) :-
    failed_request(S, R, _, 401),
    auth_expired(S, Store, Key).
auth_failed_request(S, R, Store, Key) :-
    failed_request(S, R, _, 403),
    auth_expired(S, Store, Key).
root_cause_at(S, URL, "authentication", Key, T) :-
    auth_failed_request(S, R, _, Key),
    failed_request_at(S, R, URL, _, T).

unhydrated_page(S, URL) :-
    page_framework(S, URL, StartedMs),
    browser_session_clock(S, Now),
    Age = fn:minus(Now, StartedMs),
    config_param(/browser_unhydrated_age_ms, Threshold),
    Age >= Threshold,
    !page_hydrated(S, URL).
dead_control(S, Ref, Label) :-
    interactive(S, Ref, Type, Label, _),
    Type != "link",
    element_hydration(S, Ref, "dead").
dead_control(S, Ref, Label) :-
    interactive(S, Ref, Type, Label, _),
    Type != "link",
    element_hydration(S, Ref, "unhydrated"),
    current_url(S, URL),
    unhydrated_page(S, URL).
foreign_control(S, Ref, Label) :-
    interactive(S, Ref, _, Label, _),
    element_hydration(S, Ref, "foreign").
browser_dead_ref(S, Ref) :- dead_control(S, Ref, _).
browser_disabled_ref(S, Ref) :- element_enabled(S, Ref, "false").

# The table owns the vocabulary; configured facts own every priority number.
browser_action_rank("button", "click", /browser_priority_button, "enabled_button").
browser_action_rank("input", "type", /browser_priority_input, "enabled_input").
browser_action_rank("select", "select", /browser_priority_select, "enabled_select").
browser_action_rank("checkbox", "toggle", /browser_priority_checkbox, "toggle_control").
browser_action_rank("radio", "toggle", /browser_priority_radio, "radio_control").
browser_action_rank("link", "click", /browser_priority_link, "link_click").
browser_action_rank("tab", "click", /browser_priority_tab, "tab").
browser_action_rank("combobox", "click", /browser_priority_combobox_click, "open_combobox").
browser_action_rank("combobox", "type", /browser_priority_combobox_type, "enabled_combobox_input").
browser_action_rank("menuitem", "click", /browser_priority_menuitem, "menu_item").
browser_action_rank("option", "click", /browser_priority_option, "option").
browser_action_rank("clickable", "click", /browser_priority_clickable, "clickable_element").
config_param_required(/browser, Key) :- browser_action_rank(_, _, Key, _).

action_candidate(S, Ref, Label, Action, Priority, Reason) :-
    interactive(S, Ref, Type, Label, Action),
    element_enabled(S, Ref, "true"),
    browser_action_rank(Type, Action, Key, Reason),
    config_param(Key, Priority),
    Qualified = fn:string:concat(S, ":", Ref),
    !browser_dead_ref(S, Ref),
    !browser_disabled_ref(S, Ref),
    !honeypot_detected(Qualified),
    !high_confidence_honeypot(Qualified).
action_candidate(S, Ref, Label, "click", Priority, "primary_action") :-
    interactive(S, Ref, "button", Label, "click"),
    browser_control_attribute(S, Ref, "type", "submit"),
    element_enabled(S, Ref, "true"),
    config_param(/browser_priority_primary, Priority),
    Qualified = fn:string:concat(S, ":", Ref),
    !browser_dead_ref(S, Ref),
    !browser_disabled_ref(S, Ref),
    !honeypot_detected(Qualified),
    !high_confidence_honeypot(Qualified).

# Effects use completion timestamps: a request may start before the batch.
act_http_effect(S, Batch, R, URL, Status, T) :-
    act_batch(S, Batch, StartedMs),
    failed_request_done(S, R, URL, Status, T),
    T > StartedMs.
act_effect(S, Batch, "http_error", R, URL, T) :-
    act_http_effect(S, Batch, R, URL, _, T).
act_effect(S, Batch, "network_failure", R, ErrorText, T) :-
    act_batch(S, Batch, StartedMs),
    network_failure(S, R, _, ErrorText, T),
    T > StartedMs.
act_effect(S, Batch, "console_error", "console", Message, T) :-
    act_batch(S, Batch, StartedMs),
    console_event(S, "error", Message, T),
    T > StartedMs.
act_effect(S, Batch, "browser_error", Source, Text, T) :-
    act_batch(S, Batch, StartedMs),
    browser_log(S, Source, "error", Text, _, T),
    T > StartedMs.
act_effect(S, Batch, "dialog", Type, Message, T) :-
    act_batch(S, Batch, StartedMs),
    js_dialog(S, Type, Message, _, _, T),
    T > StartedMs.
act_effect(S, Batch, "download", Guid, Name, T) :-
    act_batch(S, Batch, StartedMs),
    download(S, Guid, _, Name, _, T),
    T > StartedMs.
act_effect(S, Batch, "toast", Source, Text, T) :-
    act_batch(S, Batch, StartedMs),
    toast_notification(S, Text, _, Source, T),
    T > StartedMs.

# Conflicting policy facts fail closed. An answer is not a permission grant.
browser_dialog_dismiss(S) :- dialog_policy(S, "dismiss").
browser_dialog_accept(S) :-
    dialog_policy(S, "accept"),
    !browser_dialog_dismiss(S).
dialog_answer(S, "accept") :-
    browser_dialog_accept(S).
dialog_answer(S, "dismiss") :-
    browser_session_clock(S, _),
    !browser_dialog_accept(S).
dialog_answer(S, "dismiss") :-
    js_dialog(S, _, _, _, _, _),
    !browser_dialog_accept(S).
dialog_answer(S, "dismiss") :-
    dialog_policy(S, _),
    !browser_dialog_accept(S).

browser_last_attended(S, Last) :-
    attended(S, T) |> do fn:group_by(S), let Last = fn:max(T).
browser_has_attended(S) :- attended(S, _).
unattended_navigation(S, URL, T) :-
    navigation_event(S, URL, T),
    browser_last_attended(S, Last),
    Delta = fn:minus(T, Last),
    config_param(/browser_attended_navigation_window_ms, Window),
    Delta > Window.
unattended_navigation(S, URL, T) :-
    navigation_event(S, URL, T),
    !browser_has_attended(S).

# Retention reads this declaration rather than maintaining a Go failure list.
failure_evidence_predicate("net_http_error").
failure_evidence_predicate("net_loading_failed").
failure_evidence_predicate("net_failure").
failure_evidence_predicate("net_failure_body").
failure_evidence_predicate("console_event").
failure_evidence_predicate("browser_log").
failure_evidence_predicate("ws_event").
failure_evidence_predicate("page_load_failed").
failure_evidence_predicate("js_dialog").
failure_evidence_predicate("download").
failure_evidence_predicate("toast_notification").

