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

# Network
Decl net_request(ReqID, Method, URL, InitType, Timestamp) bound [/string, /name, /string, /name, /number].
Decl net_response(ReqID, Status, Latency, Duration) bound [/string, /name, /number, /number].
Decl net_header(ReqID, Direction, Key, Value) bound [/string, /name, /string, /string].
Decl request_initiator(ReqID, InitType, ParentRef) bound [/string, /name, /string].

# Events
Decl navigation_event(SessionID, URL, Timestamp) bound [/string, /string, /number].
Decl current_url(SessionID, URL) bound [/string, /string].
Decl console_event(Level, Message, Timestamp) bound [/name, /string, /number].
Decl click_event(ElemID, Timestamp) bound [/string, /number].
Decl input_event(ElemID, Value, Timestamp) bound [/string, /string, /number].
Decl state_change(Name, Value, Timestamp) bound [/string, /string, /number].

# Interactive elements
Decl interactable(ID, ElemType) bound [/string, /name].
Decl geometry(ID, X, Y, Width, Height) bound [/string, /number, /number, /number, /number].
