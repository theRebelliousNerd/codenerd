package tools

import (
	"context"

	"codenerd/internal/world/codemodel"
)

// ChangedElements parses before and after with codemodel — the parser
// edit_element already uses — and returns every element whose own bytes
// differ, plus elements that appeared or disappeared.
//
// The key is the address RecordEdit stores, and the receiver is the base type
// (codemodel.ReceiverTypeName unwraps *, parentheses and type arguments), so
// worldRefOf spells fn:pkg.Box.Get the way go_parser.buildRef spells the
// code_element for func (b *Box[T]) Get.
//
// Identity is that key plus the hash of the element's own span
// (codemodel.Revision), not a line overlap and not a text search. A line
// inserted between two functions changes neither span, so neither is
// recorded. A package-clause edit changes the header only: function bodies
// keep their bytes.
//
// before empty is a new file: every element, none removed. after empty is a
// deleted file: every element removed. The empty side is not parsed — an
// empty buffer is a syntax_error element the file never had. Both empty
// returns nil. A language codemodel does not model (Python, TypeScript,
// Rust, and anything else LanguageOf leaves blank) returns nil: there is no
// span to diff, and a guessed ref would not join code_element.
func ChangedElements(rel, before, after string) []EditedElement {
	if codemodel.LanguageOf(rel) == "" {
		return nil
	}
	if before == "" && after == "" {
		return nil
	}
	if after == "" {
		return elementsOf(rel, before, true)
	}
	if before == "" {
		return elementsOf(rel, after, false)
	}
	bf, ok := codemodel.Parse(rel, before)
	if !ok || bf == nil {
		return nil
	}
	af, ok := codemodel.Parse(rel, after)
	if !ok || af == nil {
		return nil
	}
	prev := indexElements(bf.Elements)
	var edits []EditedElement
	seen := make(map[string]bool, len(af.Elements))
	for i := range af.Elements {
		e := af.Elements[i]
		seen[e.Key] = true
		old, existed := prev[e.Key]
		if existed && old.Revision == e.Revision {
			continue
		}
		edits = append(edits, editedElement(rel, af, e, false))
	}
	for i := range bf.Elements {
		e := bf.Elements[i]
		if seen[e.Key] {
			continue
		}
		edits = append(edits, editedElement(rel, bf, e, true))
	}
	return edits
}

// RecordChangedSource records the elements a line or file edit changed and
// returns that same slice. abs is the contained path the tool wrote; the
// fact carries the workspace-relative name the element verbs use.
//
// The registry ignores the return and lets its completion sink assert the
// rows. VirtualStore's line handlers assert from the return, so both paths
// share this one diff instead of each naming the elements itself.
func RecordChangedSource(ctx context.Context, abs, before, after string) []EditedElement {
	edits := ChangedElements(WorkspaceDisplayPath(ctx, abs), before, after)
	RecordEdit(ctx, edits...)
	return edits
}

func elementsOf(rel, src string, removed bool) []EditedElement {
	f, ok := codemodel.Parse(rel, src)
	if !ok || f == nil {
		return nil
	}
	edits := make([]EditedElement, 0, len(f.Elements))
	for i := range f.Elements {
		edits = append(edits, editedElement(rel, f, f.Elements[i], removed))
	}
	return edits
}

func indexElements(els []codemodel.Element) map[string]codemodel.Element {
	m := make(map[string]codemodel.Element, len(els))
	for i := range els {
		if _, ok := m[els[i].Key]; ok {
			continue
		}
		m[els[i].Key] = els[i]
	}
	return m
}

func editedElement(rel string, f *codemodel.File, e codemodel.Element, removed bool) EditedElement {
	return EditedElement{
		File: rel, Language: f.Language, Package: f.Package,
		Kind: string(e.Kind), Name: e.Name, Receiver: e.Receiver, Key: e.Key,
		Removed: removed,
	}
}
