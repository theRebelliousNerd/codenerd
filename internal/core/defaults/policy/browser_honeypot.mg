# Browser Honeypot Detection Policy
# Domain: Security/Browser
#
# Detects deceptive UI patterns used to trick users or bots.
# Based on: css properties, positioning, and attribute patterns.

# --- CSS-based hiding ---

# Hidden via display:none
honeypot_css_hidden(Elem) :- css_property(Elem, "display", "none").
honeypot_css_hidden(Elem) :- css_property(Elem, "display", "hidden").

# Hidden via visibility:hidden
honeypot_css_invisible(Elem) :- css_property(Elem, "visibility", "hidden").

# Hidden via opacity:0
honeypot_opacity_hidden(Elem) :- css_property(Elem, "opacity", "0").
honeypot_opacity_hidden(Elem) :- css_property(Elem, "opacity", "0.0").

# --- Position-based hiding ---

# Positioned off-screen (negative coordinates)
honeypot_offscreen(Elem) :-
    position(Elem, X, _, _, _),
    fn:int64:lt(X, -1000).

honeypot_offscreen(Elem) :-
    position(Elem, _, Y, _, _),
    fn:int64:lt(Y, -1000).

# --- Size-based hiding ---

# Zero or near-zero size
honeypot_zero_size(Elem) :-
    position(Elem, _, _, W, H),
    fn:int64:lt(W, 2),
    fn:int64:lt(H, 2).

# --- Attribute-based hiding ---

# Marked as aria-hidden
honeypot_aria_hidden(Elem) :- attribute(Elem, "aria-hidden", "true").

# Negative tabindex (not keyboard accessible)
honeypot_no_keyboard(Elem) :- attribute(Elem, "tabindex", "-1").

# Pointer events disabled
honeypot_pointer_events_none(Elem) :- css_property(Elem, "pointerEvents", "none").

# --- Content-based heuristics ---

# Suspicious URL patterns
honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:string:contains(Href, "honeypot").

honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:string:contains(Href, "trap").

honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:string:contains(Href, "captcha").

# --- Aggregation Rules ---

# Main honeypot derivation
is_honeypot(Elem) :- honeypot_css_hidden(Elem).
is_honeypot(Elem) :- honeypot_css_invisible(Elem).
is_honeypot(Elem) :- honeypot_opacity_hidden(Elem).
is_honeypot(Elem) :- honeypot_offscreen(Elem).
is_honeypot(Elem) :- honeypot_zero_size(Elem).
is_honeypot(Elem) :- honeypot_aria_hidden(Elem).
is_honeypot(Elem) :- honeypot_pointer_events_none(Elem).
is_honeypot(Elem) :- honeypot_suspicious_url(Elem).

# High confidence honeypot (multiple indicators)
high_confidence_honeypot(Elem) :-
    honeypot_css_hidden(Elem),
    honeypot_zero_size(Elem).

high_confidence_honeypot(Elem) :-
    honeypot_offscreen(Elem),
    honeypot_no_keyboard(Elem).
