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

# Interactive elements
Decl interactable(ID, ElemType) bound [/string, /name].
Decl geometry(ID, X, Y, Width, Height) bound [/string, /number, /number, /number, /number].
