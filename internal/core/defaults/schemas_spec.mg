# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: SPEC PAIRING
# Sections: 35

# =============================================================================
# SECTION 35: SPEC PAIRING (Markdown sections <-> code elements)
# =============================================================================
# A spec is a Markdown section, addressed like a code element:
# "Docs/x.md:Install > Linux" (heading path, "#2" for a duplicate heading). A
# code element is addressed by the structure index's ref
# ("internal/core.RealKernel.Evaluate"). internal/specs reads the edges between
# them (prose citations, "spec:" lines in doc comments, implements annotations)
# and the world scan projects them here (internal/world/spec_facts.go), replaced
# whole whenever a scan measures a changed pairing. Judgment is policy/spec_pairing.mg.
#
# Times are unix seconds of the file holding the section or the element: the
# world keeps no per-element history, so drift is judged at file granularity.

# -----------------------------------------------------------------------------
# 35.1 Measured pairing (EDB, world scan)
# -----------------------------------------------------------------------------

# spec_section(Ref, Path, Heading, LastModified) - a section some link reaches.
Decl spec_section(Ref, Path, Heading, LastModified) bound [/string, /string, /string, /number].

# spec_element(Ref, Dir, File, Kind, Exported, LastModified) - a code element the
# pairing speaks of: a link target, an exported declaration of a package a link
# reaches (the universe unspecced_element is judged over), or a requirement target.
# Kind: /function, /method, /struct, /interface, /type, /class, /header, /decl ...
# Exported: /true or /false
Decl spec_element(Ref, Dir, File, Kind, Exported, LastModified) bound [/string, /string, /string, /name, /name, /number].

# spec_link(SpecRef, CodeRef, Relation, Source) - one edge between a section and an
# element. A citation of an element that no longer exists keeps the ref as written,
# so it can be flagged.
# Relation: /cites (prose names the element), /specifies (the element's doc comment
#   names the section), /implements (an explicit annotation in the section)
# Source: /doc_citation, /code_comment, /annotation
Decl spec_link(SpecRef, CodeRef, Relation, Source) bound [/string, /string, /name, /name].

# spec_element_property(CodeRef, Property) - measured for the elements requirements
# name only (a test search and a call graph are not free).
# Property: /has_test, /has_doc_comment
Decl spec_element_property(CodeRef, Property) bound [/string, /name].

# spec_element_calls(CodeRef, Callee) - a direct call of a requirement target.
Decl spec_element_calls(CodeRef, Callee) bound [/string, /string].

# -----------------------------------------------------------------------------
# 35.2 Machine-checkable specs (EDB, Docs/specs/*.mg and .nerd/specs/*.mg)
# -----------------------------------------------------------------------------
# Spec files hold these three predicates as ground facts and nothing else
# (internal/specs.ParseSpecFile refuses a rule, a Decl or any other predicate).

# spec_requirement(Id, Text) - one stated obligation.
Decl spec_requirement(Id, Text) bound [/name, /string].

# requires(Id, CodeRef, Property) - the element must have the property.
# Property: /exists, /exported, /has_test, /has_doc_comment
Decl requires(Id, CodeRef, Property) bound [/name, /string, /name].

# requires_calls(Id, CodeRef, Other) - the element must call the element Other.
Decl requires_calls(Id, CodeRef, Other) bound [/name, /string, /string].

# -----------------------------------------------------------------------------
# 35.3 Derived judgments (policy/spec_pairing.mg)
# -----------------------------------------------------------------------------

# spec_drift(SpecRef, CodeRef) - the element's file changed after the section's.
Decl spec_drift(SpecRef, CodeRef) bound [/string, /string].

# dangling_spec_link(SpecRef, CodeRef) - one end of the link no longer exists.
Decl dangling_spec_link(SpecRef, CodeRef) bound [/string, /string].

# unspecced_element(CodeRef) - an exported declaration, in a package some spec
# reaches, that no spec reaches.
Decl unspecced_element(CodeRef) bound [/string].

# spec_coverage(Dir, Specced, Total) - the exported declarations of a package
# some spec reaches, and how many of them a spec reaches.
Decl spec_coverage(Dir, Specced, Total) bound [/string, /number, /number].

# requirement_violated(Id, CodeRef, Property) - a requirement the code model does
# not meet. Property is /calls for a requires_calls row.
Decl requirement_violated(Id, CodeRef, Property) bound [/name, /string, /name].

# Helpers of the judgments above.
Decl spec_element_known(Ref) bound [/string].
Decl spec_section_known(Ref) bound [/string].
Decl spec_linked(CodeRef) bound [/string].
Decl spec_subject_kind(Kind) bound [/name].
Decl spec_subject(CodeRef, Dir) bound [/string, /string].
Decl spec_package(Dir) bound [/string].
Decl spec_package_total(Dir, Total) bound [/string, /number].
Decl spec_package_specced(Dir, Specced) bound [/string, /number].
Decl spec_package_has_specced(Dir) bound [/string].
