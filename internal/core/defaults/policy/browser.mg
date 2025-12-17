# Browser Physics
# Section 9 of Cortex Executive Policy


# Spatial reasoning - element to the left (constrained to interactable elements to avoid O(N²))
left_of(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, Ax, _, _, _),
    geometry(B, Bx, _, _, _),
    Ax < Bx.

# Element above another (constrained to interactable elements)
above(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, _, Ay, _, _),
    geometry(B, _, By, _, _),
    Ay < By.

# Honeypot detection via CSS properties
honeypot_detected(ID) :-
    computed_style(ID, "display", "none").

honeypot_detected(ID) :-
    computed_style(ID, "visibility", "hidden").

honeypot_detected(ID) :-
    computed_style(ID, "opacity", "0").

honeypot_detected(ID) :-
    geometry(ID, _, _, 0, _).

honeypot_detected(ID) :-
    geometry(ID, _, _, _, 0).

# Safe interactive elements (not honeypots)
safe_interactable(ID) :-
    interactable(ID, _),
    !honeypot_detected(ID).

# Target checkbox to the left of label text
target_checkbox(CheckID, LabelText) :-
    dom_node(CheckID, /input, _, _),
    attribute(CheckID, /type, /checkbox),
    dom_text(TextID, LabelText),
    left_of(CheckID, TextID).
