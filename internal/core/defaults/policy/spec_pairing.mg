# Spec Pairing: which section specifies which element, and what has moved since.
# Version: 1.0.0
#
# Inputs are the world scan's measurements (schemas_spec.mg section 35). Go
# reads Markdown and code and measures; every judgment here is derived.

# -----------------------------------------------------------------------------
# Drift: the code changed after the section that specifies it.
# -----------------------------------------------------------------------------
spec_drift(SpecRef, CodeRef) :-
    spec_link(SpecRef, CodeRef, _, _),
    spec_section(SpecRef, _, _, SectionTime),
    spec_element(CodeRef, _, _, _, _, CodeTime),
    CodeTime > SectionTime.

# -----------------------------------------------------------------------------
# Dangling: a link with an end that is not in the world any more. A citation of a
# removed element keeps the ref as written, which no spec_element carries.
# -----------------------------------------------------------------------------
spec_element_known(Ref) :- spec_element(Ref, _, _, _, _, _).
spec_section_known(Ref) :- spec_section(Ref, _, _, _).

dangling_spec_link(SpecRef, CodeRef) :-
    spec_link(SpecRef, CodeRef, _, _),
    !spec_element_known(CodeRef).

dangling_spec_link(SpecRef, CodeRef) :-
    spec_link(SpecRef, CodeRef, _, _),
    !spec_section_known(SpecRef).

# -----------------------------------------------------------------------------
# Coverage: the exported declarations of every package a spec reaches.
# -----------------------------------------------------------------------------
spec_linked(CodeRef) :- spec_link(_, CodeRef, _, _).

spec_subject_kind(/function).
spec_subject_kind(/method).
spec_subject_kind(/struct).
spec_subject_kind(/interface).
spec_subject_kind(/type).
spec_subject_kind(/class).

spec_subject(CodeRef, Dir) :-
    spec_element(CodeRef, Dir, _, Kind, /true, _),
    spec_subject_kind(Kind).

# A package is reached when any link ends in one of its elements, a file-level
# citation (its header) included.
spec_package(Dir) :-
    spec_link(_, CodeRef, _, _),
    spec_element(CodeRef, Dir, _, _, _, _).

unspecced_element(CodeRef) :-
    spec_subject(CodeRef, Dir),
    spec_package(Dir),
    !spec_linked(CodeRef).

spec_package_total(Dir, Total) :-
    spec_package(Dir),
    spec_subject(CodeRef, Dir)
    |> do fn:group_by(Dir), let Total = fn:count().

spec_package_has_specced(Dir) :-
    spec_subject(CodeRef, Dir),
    spec_linked(CodeRef).

spec_package_specced(Dir, Specced) :-
    spec_package(Dir),
    spec_subject(CodeRef, Dir),
    spec_linked(CodeRef)
    |> do fn:group_by(Dir), let Specced = fn:count().

spec_package_specced(Dir, 0) :-
    spec_package(Dir),
    !spec_package_has_specced(Dir).

spec_coverage(Dir, Specced, Total) :-
    spec_package_total(Dir, Total),
    spec_package_specced(Dir, Specced).

# -----------------------------------------------------------------------------
# Requirements: obligations stated in Docs/specs/*.mg and .nerd/specs/*.mg,
# checked against the code model. A property of an element that is not there is
# violated, whatever the property.
# -----------------------------------------------------------------------------
requirement_violated(Id, CodeRef, Property) :-
    requires(Id, CodeRef, Property),
    !spec_element_known(CodeRef).

requirement_violated(Id, CodeRef, /exported) :-
    requires(Id, CodeRef, /exported),
    spec_element(CodeRef, _, _, _, /false, _).

requirement_violated(Id, CodeRef, /has_test) :-
    requires(Id, CodeRef, /has_test),
    spec_element_known(CodeRef),
    !spec_element_property(CodeRef, /has_test).

requirement_violated(Id, CodeRef, /has_doc_comment) :-
    requires(Id, CodeRef, /has_doc_comment),
    spec_element_known(CodeRef),
    !spec_element_property(CodeRef, /has_doc_comment).

requirement_violated(Id, CodeRef, /calls) :-
    requires_calls(Id, CodeRef, _),
    !spec_element_known(CodeRef).

requirement_violated(Id, CodeRef, /calls) :-
    requires_calls(Id, CodeRef, Other),
    spec_element_known(CodeRef),
    !spec_element_calls(CodeRef, Other).
