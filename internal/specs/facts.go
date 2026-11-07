package specs

import (
	"sort"

	"codenerd/internal/types"
)

// Predicates are the EDB predicates a Result projects (schemas_spec.mg). The
// world scan owns them as one group: a scan that measures replaces all of them.
var Predicates = []string{
	"spec_section",
	"spec_element",
	"spec_link",
	"spec_element_property",
	"spec_element_calls",
	"spec_requirement",
	"requires",
	"requires_calls",
}

func name(s string) types.MangleAtom { return types.MangleAtom("/" + s) }

func boolAtom(b bool) types.MangleAtom {
	if b {
		return name("true")
	}
	return name("false")
}

// Facts projects the result as kernel facts, in a stable order.
func (r Result) Facts() []types.Fact {
	var out []types.Fact
	for _, s := range r.Sections {
		out = append(out, types.Fact{Predicate: "spec_section",
			Args: []any{s.Ref, s.File, s.Heading, r.SectionModified[s.Ref]}})
	}
	for _, e := range r.Elements {
		out = append(out, types.Fact{Predicate: "spec_element",
			Args: []any{e.Ref, e.Dir, e.File, name(e.Kind), boolAtom(e.Exported), r.ElementModified[e.Ref]}})
	}
	for _, l := range r.Links {
		out = append(out, types.Fact{Predicate: "spec_link",
			Args: []any{l.SpecRef, l.CodeRef, name(string(l.Relation)), name(string(l.Source))}})
	}
	refs := make([]string, 0, len(r.Measured))
	for ref := range r.Measured {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		m := r.Measured[ref]
		if e, ok := r.element(ref); ok && e.HasDoc {
			out = append(out, propertyFact(ref, PropHasDocComment))
		}
		if m.HasTest {
			out = append(out, propertyFact(ref, PropHasTest))
		}
		for _, callee := range m.Calls {
			out = append(out, types.Fact{Predicate: "spec_element_calls", Args: []any{ref, callee}})
		}
	}
	for _, q := range r.Requirements {
		out = append(out, types.Fact{Predicate: "spec_requirement", Args: []any{types.MangleAtom(q.ID), q.Text}})
	}
	for _, q := range r.Requires {
		out = append(out, types.Fact{Predicate: "requires", Args: []any{types.MangleAtom(q.ID), q.CodeRef, name(string(q.Property))}})
	}
	for _, q := range r.Calls {
		out = append(out, types.Fact{Predicate: "requires_calls", Args: []any{types.MangleAtom(q.ID), q.CodeRef, q.Other}})
	}
	return out
}

func propertyFact(ref string, p Property) types.Fact {
	return types.Fact{Predicate: "spec_element_property", Args: []any{ref, name(string(p))}}
}

func (r Result) element(ref string) (Element, bool) {
	for _, e := range r.Elements {
		if e.Ref == ref {
			return e, true
		}
	}
	return Element{}, false
}
