Decl element(ID.Type<atom>, Tag.Type<atom>, Parent.Type<atom>).
Decl css_property(Elem.Type<atom>, Prop.Type<atom>, Value.Type<atom>).
Decl position(Elem.Type<atom>, X, Y, Width, Height).
Decl attribute(Elem.Type<atom>, Name.Type<atom>, Value).
Decl link(Elem.Type<atom>, Href).
Decl visible(Elem.Type<atom>).

# Spatial and Interaction Logic
Decl left_of(A.Type<atom>, B.Type<atom>).
Decl above(A.Type<atom>, B.Type<atom>).
Decl honeypot_detected(ID.Type<atom>).
Decl safe_interactable(ID.Type<atom>).
Decl target_checkbox(CheckID.Type<atom>, LabelText).

# Honeypot Intermediate Predicates
Decl honeypot_css_hidden(Elem.Type<atom>).
Decl honeypot_css_invisible(Elem.Type<atom>).
Decl honeypot_opacity_hidden(Elem.Type<atom>).
Decl honeypot_offscreen(Elem.Type<atom>).
Decl honeypot_zero_size(Elem.Type<atom>).
Decl honeypot_aria_hidden(Elem.Type<atom>).
Decl honeypot_no_keyboard(Elem.Type<atom>).
Decl honeypot_pointer_events_none(Elem.Type<atom>).
Decl honeypot_suspicious_url(Elem.Type<atom>).
Decl is_honeypot(Elem.Type<atom>).
Decl high_confidence_honeypot(Elem.Type<atom>).

# DOM Tree Extended
Decl dom_node(ID.Type<atom>, Tag.Type<atom>, Text, Parent.Type<atom>).
Decl dom_text(ID.Type<atom>, Text).
Decl dom_attr(ID.Type<atom>, Key.Type<atom>, Value).
Decl dom_layout(ID.Type<atom>, X, Y, Width, Height, Visible.Type<atom>).

# React Fiber
Decl react_component(FiberID.Type<atom>, Name.Type<atom>, Parent.Type<atom>).
Decl react_prop(FiberID.Type<atom>, Key.Type<atom>, Value).
Decl react_state(FiberID.Type<atom>, Index, Value).
Decl dom_mapping(FiberID.Type<atom>, DomID.Type<atom>).

# Network
Decl net_request(ReqID.Type<atom>, Method.Type<atom>, URL, InitType.Type<atom>, Timestamp).
Decl net_response(ReqID.Type<atom>, Status, Latency, Duration).
Decl net_header(ReqID.Type<atom>, Direction.Type<atom>, Key.Type<atom>, Value).
Decl request_initiator(ReqID.Type<atom>, InitType.Type<atom>, ParentRef.Type<atom>).

# Events
Decl navigation_event(SessionID.Type<atom>, URL, Timestamp).
Decl current_url(SessionID.Type<atom>, URL).
Decl console_event(Level.Type<atom>, Message, Timestamp).
Decl click_event(ElemID.Type<atom>, Timestamp).
Decl input_event(ElemID.Type<atom>, Value, Timestamp).
Decl state_change(Name.Type<atom>, Value, Timestamp).

# Interactive elements
Decl interactable(ID.Type<atom>, Type.Type<atom>).
Decl geometry(ID.Type<atom>, X, Y, Width, Height).
