# Honeypot Detection Rules
# These rules derive is_honeypot(ElemID) based on CSS and attribute patterns
# Extracted from internal/browser/honeypot.go
# NOTE: All Decl statements are in schemas_browser.mg - do not duplicate here

# CSS-based hiding
honeypot_css_hidden(Elem) :- css_property(Elem, /display, /none).
honeypot_css_invisible(Elem) :- css_property(Elem, /visibility, /hidden).
honeypot_opacity_hidden(Elem) :- css_property(Elem, /opacity, "0").

# Position-based hiding (off-screen)
honeypot_offscreen(Elem) :-
    position(Elem, X, _, _, _),
    X < -1000.
honeypot_offscreen(Elem) :-
    position(Elem, _, Y, _, _),
    Y < -1000.

# Zero or near-zero size
honeypot_zero_size(Elem) :-
    position(Elem, _, _, W, H),
    W < 2,
    H < 2.

# ARIA hidden
honeypot_aria_hidden(Elem) :- attribute(Elem, "aria-hidden", /true).

# Negative tabindex (not keyboard accessible)
honeypot_no_keyboard(Elem) :- attribute(Elem, /tabindex, "-1").

# Pointer events disabled
honeypot_pointer_events_none(Elem) :- css_property(Elem, /pointerEvents, /none).

# Suspicious URL patterns
# NOTE: String matching (fn:contains) is not available in Mangle.
# These facts must be asserted from Go after URL analysis.
# See internal/browser/honeypot.go for the Go-side implementation.

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
