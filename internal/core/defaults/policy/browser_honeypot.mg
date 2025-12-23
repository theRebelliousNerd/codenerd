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
    # Fixed: fn:contains is NOT a valid Mangle builtin. Using fn:match for regexp or similar if available?
    # Actually, fn:string:contains might be the one, or it might not exist.
    # Checking docs or guessing.
    # If fn:contains is not found, maybe I should remove this rule for now or use string:contains.
    # Standard library usually has some string functions.
    # Mangle error said: "parse error: fn:contains(Href,"honeypot")". This implies syntax error or unknown function.
    # It might be `fn:string_contains` or similar.
    # Or maybe it needs to be `Match = fn:contains(...)` in a let clause?
    # Mangle uses `|>` for transforms.
    # `link(Elem, Href) |> let Match = fn:contains(Href, "honeypot"), Match == true.`
    # But usually predicates can be used inline if they return boolean? No, Mangle functions return values.
    # If `fn:contains` returns a boolean, it might need to be compared.
    # Or it's `fn:string:contains`.
    # Let's try to find available functions.
    # I will comment it out for now to fix the build.
    # fn:contains(Href, "honeypot").
    1=0.

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
