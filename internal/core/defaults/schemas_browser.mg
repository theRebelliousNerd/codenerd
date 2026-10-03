# Browser DOM Schemas
# codeNERD Browser Semantic Layer
# Fixed Syntax by Logos 🏛️

# DOM Elements
Decl element(ID, Tag, Parent) bound [/string, /string, /string].
Decl css_property(Elem, Prop, Value) bound [/string, /string, /string].
Decl computed_style(ID, Prop, Val) bound [/string, /string, /string].
Decl position(Elem, X, Y, Width, Height) bound [/string, /number, /number, /number, /number].
Decl attribute(Elem, Name, Value) bound [/string, /string, /string].
Decl link(Elem, Href) bound [/string, /string].
Decl visible(Elem) bound [/string].

# Spatial and Interaction Logic
Decl left_of(A, B) bound [/string, /string].
Decl above(A, B) bound [/string, /string].
Decl honeypot_detected(ID) bound [/string].
Decl safe_interactable(ID) bound [/string].
Decl target_checkbox(CheckID, LabelText) bound [/string, /string].

# Honeypot Intermediate Predicates
Decl honeypot_css_hidden(Elem) bound [/string].
Decl honeypot_css_invisible(Elem) bound [/string].
Decl honeypot_opacity_hidden(Elem) bound [/string].
Decl honeypot_offscreen(Elem) bound [/string].
Decl honeypot_zero_size(Elem) bound [/string].
Decl honeypot_aria_hidden(Elem) bound [/string].
Decl honeypot_no_keyboard(Elem) bound [/string].
Decl honeypot_pointer_events_none(Elem) bound [/string].
Decl honeypot_suspicious_url(Elem) bound [/string].
Decl is_honeypot(Elem) bound [/string].
Decl high_confidence_honeypot(Elem) bound [/string].

# DOM Tree Extended
Decl dom_node(ID, Tag, Text, Parent) bound [/string, /string, /string, /string].
Decl dom_text(ID, Text) bound [/string, /string].
Decl dom_attr(ID, Key, Value) bound [/string, /string, /string].
Decl dom_layout(ID, X, Y, Width, Height, Visible) bound [/string, /number, /number, /number, /number, /name].

# React Fiber
Decl react_component(FiberID, Name, Parent) bound [/string, /string, /string].
Decl react_prop(FiberID, Key, Value) bound [/string, /string, /string].
Decl react_state(FiberID, HookIndex, Value) bound [/string, /number, /string].
Decl dom_mapping(FiberID, DomID) bound [/string, /string].

# Network (session-scoped so concurrent tabs cannot contaminate diagnosis)
Decl net_request(SessionID, ReqID, Method, URL, InitType, Timestamp) bound [/string, /string, /string, /string, /string, /number].
Decl net_response(SessionID, ReqID, Status, Latency, Duration) bound [/string, /string, /number, /number, /number].
Decl net_header(SessionID, ReqID, Direction, Key, Value) bound [/string, /string, /string, /string, /string].
Decl request_initiator(SessionID, ReqID, InitType, ParentRef) bound [/string, /string, /string, /string].
Decl net_failure(SessionID, ReqID, ErrorText, BlockedReason, Timestamp) bound [/string, /string, /string, /string, /number].

# Capture shapes adapted from BrowserNERD's Apache-2.0 browser contract.
# Bodies carry structural summaries, never response values or credentials.
Decl net_http_error(SessionID, ReqID, URL, Status, ResourceType, Timestamp) bound [/string, /string, /string, /number, /string, /number].
Decl net_loading_failed(SessionID, ReqID, ErrorText, Canceled, Timestamp) bound [/string, /string, /string, /string, /number].
Decl net_failure_body(SessionID, ReqID, Shape) bound [/string, /string, /string].
Decl ws_event(SessionID, WsID, URL, Event, Detail, Timestamp) bound [/string, /string, /string, /string, /string, /number].
Decl browser_log(SessionID, Source, Level, Text, URL, Timestamp) bound [/string, /string, /string, /string, /string, /number].
Decl js_dialog(SessionID, Type, Message, Accepted, HandledBy, Timestamp) bound [/string, /string, /string, /string, /string, /number].
Decl download(SessionID, Guid, URL, SuggestedName, State, Timestamp) bound [/string, /string, /string, /string, /string, /number].
Decl page_load_failed(SessionID, URL, ErrorText, Timestamp) bound [/string, /string, /string, /number].

# Events
Decl navigation_event(SessionID, URL, Timestamp) bound [/string, /string, /number].
Decl current_url(SessionID, URL) bound [/string, /string].
Decl console_event(SessionID, Level, Message, Timestamp) bound [/string, /string, /string, /number].
Decl click_event(SessionID, ElemID, Timestamp) bound [/string, /string, /number].
Decl input_event(SessionID, ElemID, Value, Timestamp) bound [/string, /string, /string, /number].
Decl state_change(SessionID, Name, Value, Timestamp) bound [/string, /string, /string, /number].
Decl dom_updated(SessionID, Timestamp) bound [/string, /number].
Decl toast_notification(SessionID, Text, Level, Source, Timestamp) bound [/string, /string, /string, /string, /number].
Decl browser_page_state(SessionID, URL, Loading, HasDialog, Timestamp) bound [/string, /string, /name, /name, /number].

# Bounded browser diagnosis derived by the live Cortex
Decl failed_request(SessionID, ReqID, URL, Status) bound [/string, /string, /string, /number].
Decl failed_request_at(SessionID, ReqID, URL, Status, Timestamp) bound [/string, /string, /string, /number, /number].
Decl slow_api(SessionID, ReqID, URL, Duration) bound [/string, /string, /string, /number].
Decl slow_api_at(SessionID, ReqID, URL, Duration, Timestamp) bound [/string, /string, /string, /number, /number].
Decl root_cause(SessionID, Message, Source, Cause) bound [/string, /string, /string, /string].
Decl root_cause_at(SessionID, Message, Source, Cause, Timestamp) bound [/string, /string, /string, /string, /number].
Decl user_visible_error(SessionID, Source, Message, Timestamp) bound [/string, /string, /string, /number].
Decl interaction_blocked(SessionID, Reason) bound [/string, /string].
Decl interaction_blocked_at(SessionID, Reason, Timestamp) bound [/string, /string, /number].

# Response-time evidence is distinct from the existing request-time interface.
Decl asset_resource_type(Type) bound [/string].
Decl asset_http_error(SessionID, ReqID) bound [/string, /string].
Decl failed_request_done(SessionID, ReqID, URL, Status, Timestamp) bound [/string, /string, /string, /number, /number].
Decl network_failure(SessionID, ReqID, URL, ErrorText, Timestamp) bound [/string, /string, /string, /string, /number].
Decl browser_has_request(SessionID, ReqID) bound [/string, /string].
Decl browser_has_loading_failure(SessionID, ReqID) bound [/string, /string].
Decl console_error_bucket(SessionID, Bucket, Message, Timestamp) bound [/string, /number, /string, /number].
Decl request_failure_bucket(SessionID, Bucket, ReqID, Timestamp) bound [/string, /number, /string, /number].
Decl caused_by_candidate(SessionID, ErrorTs, ConsoleErr, ReqID, FailTs) bound [/string, /number, /string, /string, /number].
Decl caused_by_nearest(SessionID, ErrorTs, ConsoleErr, FailTs) bound [/string, /number, /string, /number].
Decl caused_by(SessionID, ConsoleErr, ReqID) bound [/string, /string, /string].
Decl browser_console_cause(SessionID, Message, Timestamp) bound [/string, /string, /number].

# All browser clocks and expiry metadata use epoch milliseconds.
Decl storage_entry(SessionID, Store, Key, Kind, Exp) bound [/string, /string, /string, /string, /number].
Decl browser_observed_at(SessionID, Timestamp) bound [/string, /number].
Decl browser_clock_sample(SessionID, Timestamp) bound [/string, /number].
Decl browser_session_clock(SessionID, Timestamp) bound [/string, /number].
Decl auth_expired(SessionID, Store, Key) bound [/string, /string, /string].
Decl auth_failed_request(SessionID, ReqID, Store, Key) bound [/string, /string, /string, /string].

# Go measures control ownership; policy decides eligibility and rank.
Decl interactive(SessionID, Ref, Type, Label, Action) bound [/string, /string, /string, /string, /string].
Decl element_enabled(SessionID, Ref, Enabled) bound [/string, /string, /string].
Decl element_hydration(SessionID, Ref, State) bound [/string, /string, /string].
Decl browser_control_attribute(SessionID, Ref, Key, Value) bound [/string, /string, /string, /string].
Decl page_framework(SessionID, URL, StartedMs) bound [/string, /string, /number].
Decl page_hydrated(SessionID, URL) bound [/string, /string].
Decl dead_control(SessionID, Ref, Label) bound [/string, /string, /string].
Decl foreign_control(SessionID, Ref, Label) bound [/string, /string, /string].
Decl unhydrated_page(SessionID, URL) bound [/string, /string].
Decl browser_dead_ref(SessionID, Ref) bound [/string, /string].
Decl browser_disabled_ref(SessionID, Ref) bound [/string, /string].
Decl browser_action_rank(Type, Action, ConfigKey, Reason) bound [/string, /string, /name, /string].
Decl action_candidate(SessionID, Ref, Label, Action, Priority, Reason) bound [/string, /string, /string, /string, /number, /string].

Decl act_batch(SessionID, Batch, StartedMs) bound [/string, /string, /number].
Decl act_effect(SessionID, Batch, Kind, Subject, Detail, Timestamp) bound [/string, /string, /string, /string, /string, /number].
Decl act_http_effect(SessionID, Batch, ReqID, URL, Status, Timestamp) bound [/string, /string, /string, /string, /number, /number].
Decl dialog_policy(SessionID, Answer) bound [/string, /string].
Decl browser_dialog_accept(SessionID) bound [/string].
Decl browser_dialog_dismiss(SessionID) bound [/string].
Decl dialog_answer(SessionID, Answer) bound [/string, /string].
Decl attended(SessionID, Timestamp) bound [/string, /number].
Decl browser_last_attended(SessionID, Timestamp) bound [/string, /number].
Decl browser_has_attended(SessionID) bound [/string].
Decl unattended_navigation(SessionID, URL, Timestamp) bound [/string, /string, /number].
Decl failure_evidence_predicate(Predicate) bound [/string].

# Interactive elements
Decl interactable(ID, ElemType) bound [/string, /name].
Decl geometry(ID, X, Y, Width, Height) bound [/string, /number, /number, /number, /number].

# Honeypot evidence measured in Go so the verdict stays in Mangle.
# css_clip_rect carries the parsed `clip: rect(t,r,b,l)` window; link_url_pattern
# carries the shape of an href. Go never decides whether either is a trap - the
# collapse threshold and the suspicious pattern set live in browser_honeypot.mg.
Decl css_clip_rect(Elem, Top, Right, Bottom, Left) bound [/string, /number, /number, /number, /number].
Decl link_url_pattern(Elem, Pattern) bound [/string, /name].
Decl honeypot_clip_hidden(Elem) bound [/string].
Decl honeypot_overflow_hidden(Elem) bound [/string].

# Reason vocabulary. Go used to keep a private predicate->prose checklist that
# silently drifted from the rule file; honeypot_reason makes the rule file the
# single source of which evidence codes exist.
Decl honeypot_reason(Elem, Code) bound [/string, /name].

# Event-stream epoch watermarks. Every navigation retires an epoch: the prior
# epoch's DOM, network, and interaction facts describe a page that no longer
# exists, so a consumer may scope queries to the live epoch and collect older
# ones. browser_stream_saturated records that the manager stopped asserting
# because the per-epoch fact budget was exhausted.
Decl browser_epoch(SessionID, Epoch, StartedMS) bound [/string, /number, /number].
Decl browser_stream_saturated(SessionID, Epoch, Budget, Timestamp) bound [/string, /number, /number, /number].

# Audit (session-scoped so concurrent audits cannot contaminate diagnosis)
# Kind carries the finding classification so logic can distinguish an
# observation from an inference, and Path is repository-relative so no
# absolute path enters the kernel.
Decl audit_finding(SessionID, Kind, Subject, Detail, Timestamp) bound [/string, /string, /string, /string, /number].
Decl audit_needle(SessionID, Needle) bound [/string, /string].
Decl audit_source(SessionID, Subject, Path, Line) bound [/string, /string, /string, /number].
Decl audit_hazard(SessionID, Subject) bound [/string, /string].
