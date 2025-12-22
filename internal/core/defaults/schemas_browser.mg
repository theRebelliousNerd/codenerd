# Browser DOM Schemas
# codeNERD Browser Semantic Layer

# DOM Elements
Decl element(ID.Type<string>, Tag.Type<string>, Parent.Type<string>).
Decl css_property(Elem.Type<string>, Prop.Type<string>, Value.Type<string>).
Decl computed_style(ID.Type<string>, Prop.Type<string>, Val.Type<string>).
Decl position(Elem.Type<string>, X.Type<string>, Y.Type<string>, Width.Type<string>, Height.Type<string>).
Decl attribute(Elem.Type<string>, Name.Type<string>, Value.Type<string>).
Decl link(Elem.Type<string>, Href.Type<string>).
Decl visible(Elem.Type<string>).

# Spatial and Interaction Logic
Decl left_of(A.Type<string>, B.Type<string>).
Decl above(A.Type<string>, B.Type<string>).
Decl honeypot_detected(ID.Type<string>).
Decl safe_interactable(ID.Type<string>).
Decl target_checkbox(CheckID.Type<string>, LabelText.Type<string>).

# Honeypot Intermediate Predicates
Decl honeypot_css_hidden(Elem.Type<string>).
Decl honeypot_css_invisible(Elem.Type<string>).
Decl honeypot_opacity_hidden(Elem.Type<string>).
Decl honeypot_offscreen(Elem.Type<string>).
Decl honeypot_zero_size(Elem.Type<string>).
Decl honeypot_aria_hidden(Elem.Type<string>).
Decl honeypot_no_keyboard(Elem.Type<string>).
Decl honeypot_pointer_events_none(Elem.Type<string>).
Decl honeypot_suspicious_url(Elem.Type<string>).
Decl is_honeypot(Elem.Type<string>).
Decl high_confidence_honeypot(Elem.Type<string>).

# DOM Tree Extended
Decl dom_node(ID.Type<string>, Tag.Type<string>, Text.Type<string>, Parent.Type<string>).
Decl dom_text(ID.Type<string>, Text.Type<string>).
Decl dom_attr(ID.Type<string>, Key.Type<string>, Value.Type<string>).
Decl dom_layout(ID.Type<string>, X.Type<float64>, Y.Type<float64>, Width.Type<float64>, Height.Type<float64>, Visible.Type<string>).

# React Fiber
Decl react_component(FiberID.Type<string>, Name.Type<string>, Parent.Type<string>).
Decl react_prop(FiberID.Type<string>, Key.Type<string>, Value.Type<string>).
Decl react_state(FiberID.Type<string>, Index.Type<int64>, Value.Type<string>).
Decl dom_mapping(FiberID.Type<string>, DomID.Type<string>).

# Network
Decl net_request(ReqID.Type<string>, Method.Type<string>, URL.Type<string>, InitType.Type<string>, Timestamp.Type<int64>).
Decl net_response(ReqID.Type<string>, Status.Type<int64>, Latency.Type<int64>, Duration.Type<int64>).
Decl net_header(ReqID.Type<string>, Direction.Type<string>, Key.Type<string>, Value.Type<string>).
Decl request_initiator(ReqID.Type<string>, InitType.Type<string>, ParentRef.Type<string>).

# Events
Decl navigation_event(SessionID.Type<string>, URL.Type<string>, Timestamp.Type<int64>).
Decl current_url(SessionID.Type<string>, URL.Type<string>).
Decl console_event(Level.Type<string>, Message.Type<string>, Timestamp.Type<int64>).
Decl click_event(ElemID.Type<string>, Timestamp.Type<int64>).
Decl input_event(ElemID.Type<string>, Value.Type<string>, Timestamp.Type<int64>).
Decl state_change(Name.Type<string>, Value.Type<string>, Timestamp.Type<int64>).

# Interactive elements
Decl interactable(ID.Type<string>, Type.Type<string>).
Decl geometry(ID.Type<string>, X.Type<int64>, Y.Type<int64>, Width.Type<int64>, Height.Type<int64>).
