# Honeypot Detection Rules
# These rules derive is_honeypot(ElemID) based on CSS and attribute patterns
# Extracted from internal/browser/honeypot.go
# NOTE: All Decl statements are in schemas_browser.mg - do not duplicate here

# CSS-based hiding.
#
# Every key and value here is a /string, never an /atom. css_property is declared
# bound [/string, /string, /string] and that is what the page actually delivers:
# HoneypotDetector.emitElementEvidence pushes the raw window.getComputedStyle
# map (internal/browser/honeypot.go), and SessionManager.buildDOMFacts pushes the
# raw n.Styles map (internal/browser/session_manager_dom.go). Those are arbitrary
# attacker-authored declarations, not a closed vocabulary, so there is no atom
# form to support - a duplicate `/display, /none` arm derived nothing on a live
# page and only existed to prop up hand-written EDB fixtures.
honeypot_css_hidden(Elem) :- css_property(Elem, "display", "none").
honeypot_css_invisible(Elem) :- css_property(Elem, "visibility", "hidden").
honeypot_opacity_hidden(Elem) :- css_property(Elem, "opacity", "0").

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

# ARIA hidden. attribute/3 is bound [/string, /string, /string] for the same
# reason css_property is: the name and value are whatever the page's HTML says.
honeypot_aria_hidden(Elem) :- attribute(Elem, "aria-hidden", "true").

# Negative tabindex (not keyboard accessible)
honeypot_no_keyboard(Elem) :- attribute(Elem, "tabindex", "-1").

# Pointer events disabled
honeypot_pointer_events_none(Elem) :- css_property(Elem, "pointerEvents", "none").

# Clip-based hiding. Go parses `clip: rect(t, r, b, l)` into css_clip_rect and
# nothing more; what counts as "collapsed" is policy. A clip window whose right
# and bottom edges are under 2px shows at most one pixel, which covers both the
# rect(0,0,0,0) trap and the rect(1,1,1,1) screen-reader idiom that bots follow.
honeypot_clip_hidden(Elem) :-
    css_clip_rect(Elem, _, R, B, _),
    R < 2,
    B < 2.

# overflow:hidden on its own is one of the most common declarations on the web
# and flagging it alone would make every carousel a honeypot. It is evidence
# only when the box it clips has already collapsed to nothing.
honeypot_overflow_hidden(Elem) :-
    css_property(Elem, "overflow", "hidden"),
    honeypot_zero_size(Elem).

# Suspicious URL patterns
# NOTE: String matching (fn:contains) is not available in Mangle, so Go
# classifies the href shape and asserts link_url_pattern. Which shapes are
# suspicious stays here: /empty_js_target is recorded because it is useful
# evidence, but "#" and javascript:void(0) are not traps on their own.
honeypot_suspicious_url(Elem) :- link_url_pattern(Elem, /bait_path).
honeypot_suspicious_url(Elem) :- link_url_pattern(Elem, /trap_query).

# Main honeypot derivation
is_honeypot(Elem) :- honeypot_css_hidden(Elem).
is_honeypot(Elem) :- honeypot_css_invisible(Elem).
is_honeypot(Elem) :- honeypot_opacity_hidden(Elem).
is_honeypot(Elem) :- honeypot_offscreen(Elem).
is_honeypot(Elem) :- honeypot_zero_size(Elem).
is_honeypot(Elem) :- honeypot_aria_hidden(Elem).
is_honeypot(Elem) :- honeypot_pointer_events_none(Elem).
is_honeypot(Elem) :- honeypot_suspicious_url(Elem).

is_honeypot(Elem) :- honeypot_clip_hidden(Elem).
is_honeypot(Elem) :- honeypot_overflow_hidden(Elem).

# honeypot_no_keyboard is deliberately absent from is_honeypot. tabindex="-1" is
# routine on programmatically focused dialogs and skip targets, so on its own it
# is evidence, not a verdict; it only decides anything through
# high_confidence_honeypot below.

# Reason vocabulary. Go reports what Mangle derived instead of re-deriving it:
# every code here is one the detector can surface, and a code with no rule
# cannot appear in a report.
honeypot_reason(Elem, /css_hidden) :- honeypot_css_hidden(Elem).
honeypot_reason(Elem, /css_invisible) :- honeypot_css_invisible(Elem).
honeypot_reason(Elem, /opacity_hidden) :- honeypot_opacity_hidden(Elem).
honeypot_reason(Elem, /offscreen) :- honeypot_offscreen(Elem).
honeypot_reason(Elem, /zero_size) :- honeypot_zero_size(Elem).
honeypot_reason(Elem, /aria_hidden) :- honeypot_aria_hidden(Elem).
honeypot_reason(Elem, /no_keyboard) :- honeypot_no_keyboard(Elem).
honeypot_reason(Elem, /suspicious_url) :- honeypot_suspicious_url(Elem).
honeypot_reason(Elem, /pointer_events_none) :- honeypot_pointer_events_none(Elem).
honeypot_reason(Elem, /clip_hidden) :- honeypot_clip_hidden(Elem).
honeypot_reason(Elem, /overflow_hidden) :- honeypot_overflow_hidden(Elem).

# High confidence honeypot (multiple indicators)
high_confidence_honeypot(Elem) :-
    honeypot_css_hidden(Elem),
    honeypot_zero_size(Elem).
high_confidence_honeypot(Elem) :-
    honeypot_offscreen(Elem),
    honeypot_no_keyboard(Elem).

# An interactable element the kernel has not flagged. The negated literal is
# fully bound by interactable/2 - a wildcard inside the negation (!is_honeypot(_))
# would exclude nothing at all in this Mangle build. See
# internal/core/bound_negation_test.go.
safe_interactable(ID) :-
    interactable(ID, _),
    !is_honeypot(ID).
