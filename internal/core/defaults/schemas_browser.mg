# Browser DOM Schemas
# codeNERD Browser Semantic Layer

# DOM Elements
Decl element(ID, Tag, Parent).
Decl css_property(Elem, Prop, Value).
Decl computed_style(ID, Prop, Val).
Decl position(Elem, X, Y, Width, Height).
Decl attribute(Elem, Name, Value).
Decl link(Elem, Href).
Decl visible(Elem).

# Spatial and Interaction Logic
Decl left_of(A, B).
Decl above(A, B).
Decl honeypot_detected(ID).
Decl safe_interactable(ID).
Decl target_checkbox(CheckID, LabelText).

# Honeypot Intermediate Predicates
Decl honeypot_css_hidden(Elem).
Decl honeypot_css_invisible(Elem).
Decl honeypot_opacity_hidden(Elem).
Decl honeypot_offscreen(Elem).
Decl honeypot_zero_size(Elem).
Decl honeypot_aria_hidden(Elem).
Decl honeypot_no_keyboard(Elem).
Decl honeypot_pointer_events_none(Elem).
Decl honeypot_suspicious_url(Elem).
Decl is_honeypot(Elem).
Decl high_confidence_honeypot(Elem).

# DOM Tree Extended
Decl dom_node(ID, Tag, Text, Parent).
Decl dom_text(ID, Text).
Decl dom_attr(ID, Key, Value).
Decl dom_layout(ID, X, Y, Width, Height, Visible).

# React Fiber
Decl react_component(FiberID, Name, Parent).
Decl react_prop(FiberID, Key, Value).
Decl react_state(FiberID, Index, Value).
Decl dom_mapping(FiberID, DomID).

# Network
Decl net_request(ReqID, Method, URL, InitType, Timestamp).
Decl net_response(ReqID, Status, Latency, Duration).
Decl net_header(ReqID, Direction, Key, Value).
Decl request_initiator(ReqID, InitType, ParentRef).

# Events
Decl navigation_event(SessionID, URL, Timestamp).
Decl current_url(SessionID, URL).
Decl console_event(Level, Message, Timestamp).
Decl click_event(ElemID, Timestamp).
Decl input_event(ElemID, Value, Timestamp).
Decl state_change(Name, Value, Timestamp).

# Interactive elements
Decl interactable(ID, Type).
Decl geometry(ID, X, Y, Width, Height).
