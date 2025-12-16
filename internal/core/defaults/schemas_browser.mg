# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: BROWSER
# Sections: 8, 17

# =============================================================================
# SECTION 8: BROWSER PHYSICS (§9.0)
# =============================================================================

# dom_node(ID, Tag, Parent)
Decl dom_node(ID, Tag, Parent).

# attr(ID, Key, Val)
Decl attr(ID, Key, Val).

# geometry(ID, X, Y, W, H)
Decl geometry(ID, X, Y, W, H).

# computed_style(ID, Prop, Val)
Decl computed_style(ID, Prop, Val).

# interactable(ID, Type)
# Type: /button, /input, /link, /select, /checkbox
Decl interactable(ID, Type).

# visible_text(ID, Text)
Decl visible_text(ID, Text).

# =============================================================================
# SECTION 17: BROWSER SPATIAL REASONING (§9.0)
# =============================================================================

# left_of(A, B) - derived predicate
Decl left_of(A, B).

# above(A, B) - derived predicate
Decl above(A, B).

# honeypot_detected(ID) - derived predicate
Decl honeypot_detected(ID).

# safe_interactable(ID) - derived predicate
Decl safe_interactable(ID).

# target_checkbox(CheckID, LabelText) - derived predicate
Decl target_checkbox(CheckID, LabelText).

