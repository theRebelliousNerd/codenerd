# Honeypot Detection Rules
# These rules derive is_honeypot(ElemID) based on CSS and attribute patterns
# Extracted from internal/browser/honeypot.go

# CSS-based hiding
Decl honeypot_css_hidden(Elem).
honeypot_css_hidden(Elem) :- css_property(Elem, "display", "none").

Decl honeypot_css_invisible(Elem).
honeypot_css_invisible(Elem) :- css_property(Elem, "visibility", "hidden").

Decl honeypot_opacity_hidden(Elem).
honeypot_opacity_hidden(Elem) :- css_property(Elem, "opacity", "0").

# Position-based hiding (off-screen)
Decl honeypot_offscreen(Elem).
honeypot_offscreen(Elem) :-
    position(Elem, X, _, _, _),
    X < -1000.
honeypot_offscreen(Elem) :-
    position(Elem, _, Y, _, _),
    Y < -1000.

# Zero or near-zero size
Decl honeypot_zero_size(Elem).
honeypot_zero_size(Elem) :-
    position(Elem, _, _, W, H),
    W < 2,
    H < 2.

# ARIA hidden
Decl honeypot_aria_hidden(Elem).
honeypot_aria_hidden(Elem) :- attribute(Elem, "aria-hidden", "true").

# Negative tabindex (not keyboard accessible)
Decl honeypot_no_keyboard(Elem).
honeypot_no_keyboard(Elem) :- attribute(Elem, "tabindex", "-1").

# Pointer events disabled
Decl honeypot_pointer_events_none(Elem).
honeypot_pointer_events_none(Elem) :- css_property(Elem, "pointerEvents", "none").

# Suspicious URL patterns
Decl honeypot_suspicious_url(Elem).
honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:contains(Href, "honeypot").
honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:contains(Href, "trap").
honeypot_suspicious_url(Elem) :-
    link(Elem, Href),
    fn:contains(Href, "captcha").

# Main honeypot derivation
Decl is_honeypot(Elem).
is_honeypot(Elem) :- honeypot_css_hidden(Elem).
is_honeypot(Elem) :- honeypot_css_invisible(Elem).
is_honeypot(Elem) :- honeypot_opacity_hidden(Elem).
is_honeypot(Elem) :- honeypot_offscreen(Elem).
is_honeypot(Elem) :- honeypot_zero_size(Elem).
is_honeypot(Elem) :- honeypot_aria_hidden(Elem).
is_honeypot(Elem) :- honeypot_pointer_events_none(Elem).
is_honeypot(Elem) :- honeypot_suspicious_url(Elem).

# High confidence honeypot (multiple indicators)
Decl high_confidence_honeypot(Elem).
high_confidence_honeypot(Elem) :-
    honeypot_css_hidden(Elem),
    honeypot_zero_size(Elem).
high_confidence_honeypot(Elem) :-
    honeypot_offscreen(Elem),
    honeypot_no_keyboard(Elem).
