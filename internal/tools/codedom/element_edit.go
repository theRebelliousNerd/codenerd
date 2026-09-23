package codedom

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"
)

// =============================================================================
// ELEMENT-ADDRESSED EDITS
// =============================================================================
// The only code edits the model had were line-number edits (edit_lines and
// friends) and whole-file text tools. Line numbers are the first thing an edit
// invalidates -- stale ones silently duplicated and shredded declarations in a
// live run -- and the mandatory prompt atom spent twelve lines teaching the
// arithmetic to correct them. These verbs address an element by ref instead,
// and every call is validated before anything is written:
//
//   - the file parses afterwards (a file that already did not parse may not
//     parse worse);
//   - every element the call does not target keeps its ref and its bytes
//     (gofmt may realign a group's whitespace, nothing else);
//   - the touched declarations are gofmt'd, each on its own;
//   - imports are derived: a newly used package qualifier is imported, an
//     import the edit left unused is dropped;
//   - an optional rev precondition is the element's own revision, so an edit
//     elsewhere in the file does not invalidate it.
//
// The answer carries the edited element's new text and rev, so the model never
// has to read after writing -- which it could not do under the commit regime,
// which closes every read tool.
//
// Every call takes path as well as ref. path is what every write gate reads
// (nerd.md write protection, the campaign's lease guard, the Dreamer
// preflight, the written-path record the turn's verification runs on), so an
// edit without it would either be refused by all of them or bypass them.

// resultWholeLines is the element size up to which an edit's answer shows
// the whole edited element; above it, the changed lines with context.
const resultWholeLines = 80

// resultContextLines is the context shown around the changed lines of a
// larger element.
const resultContextLines = 4

var (
	revisionProperty = tools.Property{Type: "string", Description: "Optional precondition: the element's rev as get_element or get_elements printed it. The edit is refused if the element changed since."}
	pathProperty     = tools.Property{Type: "string", Description: "Workspace-relative file the element is in"}
	refProperty      = tools.Property{Type: "string", Description: "The element: its ref as the structural tools print it (internal/world.StructureIndex.Refresh) or its name in the file (StructureIndex.Refresh, header, syntax_error)"}
)

// EditElementTool is the anchored replace scoped to one element.
func EditElementTool() *tools.Tool {
	return &tools.Tool{
		Name: "edit_element",
		Description: "Replace text inside one element: old must occur exactly once in that element (or in the part named), so a short anchor is enough. " +
			"The file must parse afterwards, every other element keeps its bytes, the element is gofmt'd and imports are derived. " +
			"Answers with the element's new text and rev. Use this for most code changes; replace_element rewrites a whole element.",
		Category: tools.CategoryCode,
		Priority: 85,
		Effect:   tools.EffectWrite,
		Execute:  executeEditElement,
		Schema: tools.ToolSchema{
			Required: []string{"path", "ref", "old", "new"},
			Properties: map[string]tools.Property{
				"path":     pathProperty,
				"ref":      refProperty,
				"old":      {Type: "string", Description: "Text to replace, exactly as it appears in the element, unique within it"},
				"new":      {Type: "string", Description: "Replacement text (empty deletes old)"},
				"part":     {Type: "string", Description: "Optional nested part (from get_element's outline, e.g. 12.3) that old must be unique within"},
				"revision": revisionProperty,
			},
		},
	}
}

// ReplaceElementTool replaces a whole element, doc comment included.
func ReplaceElementTool() *tools.Tool {
	return &tools.Tool{
		Name: "replace_element",
		Description: "Replace one whole element -- its doc comment included -- with new source (one or more declarations). " +
			"Validated like edit_element; the answer carries the new text and rev. Also repairs a broken file: ref=syntax_error replaces the region that does not parse.",
		Category: tools.CategoryCode,
		Priority: 84,
		Effect:   tools.EffectWrite,
		Execute:  executeReplaceElement,
		Schema: tools.ToolSchema{
			Required: []string{"path", "ref", "source"},
			Properties: map[string]tools.Property{
				"path":     pathProperty,
				"ref":      refProperty,
				"source":   {Type: "string", Description: "The complete new source of the element, doc comment included"},
				"revision": revisionProperty,
			},
		},
	}
}

// InsertElementTool adds declarations next to an anchor element.
func InsertElementTool() *tools.Tool {
	return &tools.Tool{
		Name: "insert_element",
		Description: "Insert new declarations before or after an anchor element; anchor header puts them after the imports, anchor end at the end of the file. " +
			"Validated like edit_element (imports derived, gofmt'd); answers with the new elements' refs, text and revs.",
		Category: tools.CategoryCode,
		Priority: 83,
		Effect:   tools.EffectWrite,
		Execute:  executeInsertElement,
		Schema: tools.ToolSchema{
			Required: []string{"path", "anchor", "source"},
			Properties: map[string]tools.Property{
				"path":     pathProperty,
				"anchor":   {Type: "string", Description: "Ref or name of the element to insert next to, or header, or end"},
				"position": {Type: "string", Description: "before or after the anchor (default after)", Enum: []any{"before", "after"}},
				"source":   {Type: "string", Description: "The declarations to insert, doc comments included"},
			},
		},
	}
}

// DeleteElementTool removes an element whose name nothing else uses.
func DeleteElementTool() *tools.Tool {
	return &tools.Tool{
		Name: "delete_element",
		Description: "Delete one element, doc comment included. Refused while anything outside it still uses its name -- the uses are listed -- unless replace_with names what those uses should point at instead " +
			"(a package-qualified name, e.g. articulation.PiggybackEnvelope) and paths lists every file holding a use: then the uses are rewritten and the element deleted in one transaction.",
		Category: tools.CategoryCode,
		Priority: 82,
		Effect:   tools.EffectWrite,
		Execute:  executeDeleteElement,
		Schema: tools.ToolSchema{
			Required: []string{"path", "ref"},
			Properties: map[string]tools.Property{
				"path":         pathProperty,
				"ref":          refProperty,
				"revision":     revisionProperty,
				"replace_with": {Type: "string", Description: "Optional: the ref or package-qualified name the element's remaining uses should point at instead"},
				"paths":        {Type: "array", Description: "With replace_with: every file holding a use (the write set of the transaction)", Items: &tools.PropertyItems{Type: "string"}},
			},
		},
	}
}

// elementEdit is one validated single-file edit, ready to commit.
type elementEdit struct {
	lf     *loadedFile
	before *codemodel.File
	out    *codemodel.Outcome
	refs   map[string]string
}

// parsedTarget loads the file at path and resolves ref within it.
func parsedTarget(ctx context.Context, path, ref string) (*resolvedElement, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("path is required: it is what the write gates check")
	}
	re, err := resolveElement(ctx, ref, path)
	if err != nil {
		return nil, err
	}
	return re, nil
}

// checkRevision refuses an edit whose precondition names a revision the
// element no longer has, and hands back the element as it is now so the edit
// can be redone without a read.
func checkRevision(re *resolvedElement, rev string) error {
	rev = strings.TrimSpace(rev)
	if rev == "" || rev == re.elem.Revision {
		return nil
	}
	return fmt.Errorf("precondition failed: %s is at rev %s, not %s -- it changed since it was read. It is now:\n%s",
		RefOf(re.file.rel, re.elem, re.refs), re.elem.Revision, rev, re.model.NumberedLines(re.elem.StartLine, re.elem.EndLine))
}

// importResolver is the workspace's resolver for imports in rel, or nil when
// no index is registered (imports are then left as the edit wrote them).
func importResolver(ctx context.Context, rel string) codemodel.Resolver {
	provider := optionalStructureProvider()
	if provider == nil {
		return nil
	}
	res, err := provider.ImportResolver(ctx, rel)
	if err != nil {
		logging.ToolsDebug("import resolver for %s unavailable: %v", rel, err)
		return nil
	}
	return res
}

// apply validates a change against the resolved file.
func applyChange(ctx context.Context, re *resolvedElement, ch codemodel.Change, targeted []string) (*elementEdit, error) {
	out, err := codemodel.Apply(re.model, ch, targeted, importResolver(ctx, re.file.rel))
	if err != nil {
		return nil, err
	}
	if out.File.Source == re.model.Source {
		return nil, fmt.Errorf("the edit changes nothing in %s", re.file.rel)
	}
	return &elementEdit{lf: re.file, before: re.model, out: out, refs: re.refs}, nil
}

// commit writes the edit in the file's own line-ending convention, records
// what it changed for the kernel, and renders the answer.
func (ee *elementEdit) commit(ctx context.Context, verb string) (string, error) {
	ending := tactile.DetectLineEnding(ee.lf.data)
	data := []byte(tactile.NormalizeLineEnding(ee.out.File.Source, ending))
	mode := os.FileMode(0o644)
	if info, err := os.Stat(ee.lf.abs); err == nil {
		mode = info.Mode().Perm()
	}
	if err := commitFiles(ctx, []plannedWrite{{abs: ee.lf.abs, rel: ee.lf.rel, orig: ee.lf.data, data: data, mode: mode}}); err != nil {
		return "", err
	}
	tools.RecordEdit(ctx, editedElements(ee.lf.rel, ee.before, ee.out)...)
	refs := canonicalRefs(ctx, ee.lf.rel, ee.out.File)
	logging.Tools("%s completed: %s (%d elements touched, %d removed)", verb, ee.lf.rel, len(ee.out.Touched), len(ee.out.Removed))
	return renderOutcome(verb, ee.lf.rel, ee.before, ee.out, refs), nil
}

// editedElements are the kernel's record of an edit: every element it
// created or changed, and every targeted element it removed.
func editedElements(rel string, before *codemodel.File, out *codemodel.Outcome) []tools.EditedElement {
	var edits []tools.EditedElement
	for _, e := range out.Touched {
		edits = append(edits, tools.EditedElement{
			File: rel, Language: out.File.Language, Package: out.File.Package,
			Kind: string(e.Kind), Name: e.Name, Receiver: e.Receiver, Key: e.Key,
		})
	}
	for _, key := range out.Removed {
		if e := before.Element(key); e != nil {
			edits = append(edits, tools.EditedElement{
				File: rel, Language: before.Language, Package: before.Package,
				Kind: string(e.Kind), Name: e.Name, Receiver: e.Receiver, Key: e.Key, Removed: true,
			})
		}
	}
	return edits
}

// renderOutcome is an edit's answer: what changed, then each touched element
// as it now is, so no read is needed after the write.
func renderOutcome(verb, rel string, before *codemodel.File, out *codemodel.Outcome, refs map[string]string) string {
	nf := out.File
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %s now parses", verb, rel)
	if nf.Language == codemodel.LangGo {
		sb.WriteString(" and is gofmt'd where it changed")
	}
	fmt.Fprintf(&sb, "; %d lines (%+d); every other element keeps its ref and rev.\n", nf.LineCount(), nf.LineCount()-before.LineCount())
	if !nf.Parsed {
		sb.Reset()
		fmt.Fprintf(&sb, "%s: %s still does not parse (it did not before either); %d lines.\n", verb, rel, nf.LineCount())
		for _, se := range nf.Errors {
			fmt.Fprintf(&sb, "-- syntax error at line %d:%d: %s\n", se.Line, se.Column, se.Msg)
		}
	}
	if len(out.Imports.Added) > 0 {
		fmt.Fprintf(&sb, "-- imports added: %s\n", strings.Join(out.Imports.Added, ", "))
	}
	if len(out.Imports.Removed) > 0 {
		fmt.Fprintf(&sb, "-- imports dropped (no longer used): %s\n", strings.Join(out.Imports.Removed, ", "))
	}
	for _, note := range out.Imports.Notes {
		fmt.Fprintf(&sb, "-- import note: %s\n", note)
	}
	for _, key := range out.Removed {
		fmt.Fprintf(&sb, "-- removed %s\n", key)
	}
	for _, e := range out.Touched {
		if e.Kind == codemodel.KindHeader && len(out.Touched) > 1 && out.Imports.Changed() {
			continue
		}
		sb.WriteString("\n")
		sb.WriteString(renderTouched(before, nf, e, refs))
	}
	return sb.String()
}

// renderTouched shows one edited element: whole when it is small, otherwise
// the lines that changed with a little context.
func renderTouched(before, nf *codemodel.File, e *codemodel.Element, refs map[string]string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  %s  %s:%d-%d  rev %s\n", RefOf(nf.Path, e, refs), e.Kind, nf.Path, e.StartLine, e.EndLine, e.Revision)
	lines := e.EndLine - e.StartLine + 1
	if lines <= resultWholeLines {
		sb.WriteString(nf.NumberedLines(e.StartLine, e.EndLine))
		return sb.String()
	}
	lo, hi := changedLines(before, nf, e)
	lo = max(e.StartLine, lo-resultContextLines)
	hi = min(e.EndLine, hi+resultContextLines)
	fmt.Fprintf(&sb, "-- %d lines; the changed lines %d-%d with context:\n", lines, lo, hi)
	sb.WriteString(nf.NumberedLines(lo, hi))
	return sb.String()
}

// changedLines is the span of an element's lines that differ from the element
// of the same key before the edit (the whole element when it is new).
func changedLines(before, nf *codemodel.File, e *codemodel.Element) (int, int) {
	old := before.Element(e.Key)
	if old == nil {
		return e.StartLine, e.EndLine
	}
	a := strings.Split(before.Text(old), "\n")
	b := strings.Split(nf.Text(e), "\n")
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	lo := e.StartLine + p
	hi := e.StartLine + len(b) - 1 - s
	if hi < lo {
		hi = lo
	}
	return lo, hi
}

func executeEditElement(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	ref, _ := args["ref"].(string)
	oldText, _ := args["old"].(string)
	newText, hasNew := args["new"].(string)
	part, _ := args["part"].(string)
	rev, _ := args["revision"].(string)
	if oldText == "" {
		return "", fmt.Errorf("old is required: the text to replace, unique within the element")
	}
	if !hasNew {
		return "", fmt.Errorf("new is required (an empty string deletes old)")
	}
	re, err := parsedTarget(ctx, path, ref)
	if err != nil {
		return "", err
	}
	if err := checkRevision(re, rev); err != nil {
		return "", err
	}
	spanStart, spanEnd, where := re.elem.Start, re.elem.End, RefOf(re.file.rel, re.elem, re.refs)
	if strings.TrimSpace(part) != "" {
		p, err := re.model.FindPart(re.elem, part)
		if err != nil {
			return "", err
		}
		spanStart, spanEnd, where = p.Start, p.End, where+" part "+p.Path
	}
	start, end, err := anchor(re.model, spanStart, spanEnd, codemodel.Normalize(oldText), where)
	if err != nil {
		return "", err
	}
	ee, err := applyChange(ctx, re, codemodel.Change{Start: start, End: end, Text: newText}, []string{re.elem.Key})
	if err != nil {
		return "", err
	}
	return ee.commit(ctx, "edit_element")
}

// anchor finds old within [spanStart, spanEnd) of the source: exactly once,
// verbatim; or, when the verbatim text is absent, exactly one run of whole
// lines equal to old's lines with leading and trailing whitespace ignored (a
// tab written as spaces is the usual cause). Anything else is refused with
// what would make it unique.
func anchor(f *codemodel.File, spanStart, spanEnd int, old, where string) (int, int, error) {
	span := f.Source[spanStart:spanEnd]
	switch n := strings.Count(span, old); {
	case n == 1:
		i := strings.Index(span, old)
		return spanStart + i, spanStart + i + len(old), nil
	case n > 1:
		var lines []string
		for off := 0; ; {
			i := strings.Index(span[off:], old)
			if i < 0 {
				break
			}
			lines = append(lines, fmt.Sprint(f.LineOf(spanStart+off+i)))
			off += i + len(old)
		}
		return 0, 0, fmt.Errorf("old occurs %d times in %s (lines %s); extend it until it is unique, or name a part", n, where, strings.Join(lines, ", "))
	}
	want := trimmedLines(old)
	if len(want) > 0 {
		startLine, endLine := f.LineOf(spanStart), f.LineOf(max(spanEnd-1, spanStart))
		var hits []int
		for l := startLine; l+len(want)-1 <= endLine; l++ {
			ok := true
			for k := range want {
				if strings.TrimSpace(f.LineText(l+k, l+k)) != want[k] {
					ok = false
					break
				}
			}
			if ok {
				hits = append(hits, l)
			}
		}
		if len(hits) == 1 {
			lo := max(f.LineStart(hits[0]), spanStart)
			hi := f.LineStart(hits[0]+len(want)) - 1
			if hits[0]+len(want) > f.LineCount() {
				hi = len(f.Source)
			}
			return lo, min(hi, spanEnd), nil
		}
	}
	e := f.LineOf(spanStart)
	return 0, 0, fmt.Errorf("old does not occur in %s. Its text is:\n%s", where, f.NumberedLines(e, f.LineOf(max(spanEnd-1, spanStart))))
}

func trimmedLines(s string) []string {
	lines := strings.Split(strings.Trim(s, "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func executeReplaceElement(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	ref, _ := args["ref"].(string)
	source, _ := args["source"].(string)
	rev, _ := args["revision"].(string)
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("source is required; delete_element removes an element")
	}
	re, err := parsedTarget(ctx, path, ref)
	if err != nil {
		return "", err
	}
	if err := checkRevision(re, rev); err != nil {
		return "", err
	}
	ee, err := applyChange(ctx, re, codemodel.Change{Start: re.elem.Start, End: re.elem.End, Text: strings.Trim(source, "\n")}, []string{re.elem.Key})
	if err != nil {
		return "", err
	}
	if len(ee.out.Touched) == 0 {
		return "", fmt.Errorf("the source declares nothing; delete_element removes an element")
	}
	return ee.commit(ctx, "replace_element")
}

func executeInsertElement(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	anchorRef, _ := args["anchor"].(string)
	position, _ := args["position"].(string)
	source, _ := args["source"].(string)
	source = strings.Trim(codemodel.Normalize(source), "\n")
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("source is required")
	}
	position = strings.ToLower(strings.TrimSpace(position))
	if position == "" {
		position = "after"
	}
	if position != "before" && position != "after" {
		return "", fmt.Errorf("position must be before or after, got %q", position)
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required: it is what the write gates check")
	}

	var re *resolvedElement
	var ch codemodel.Change
	if strings.TrimSpace(anchorRef) == codemodel.EndKey {
		lf, err := loadFile(ctx, path)
		if err != nil {
			return "", err
		}
		model, ok := codemodel.Parse(lf.rel, string(lf.data))
		if !ok {
			return "", fmt.Errorf("%s is not a file CodeDOM parses (Go or Mangle)", lf.rel)
		}
		re = &resolvedElement{file: lf, model: model, refs: canonicalRefs(ctx, lf.rel, model)}
		tail := strings.TrimRight(model.Source, "\n")
		ch = codemodel.Change{Start: len(tail), End: len(model.Source), Text: "\n\n" + source + "\n"}
	} else {
		var err error
		re, err = parsedTarget(ctx, path, anchorRef)
		if err != nil {
			return "", err
		}
		e := re.elem
		sep := "\n\n"
		if e.Grouped {
			sep = "\n"
		}
		switch {
		case e.Kind == codemodel.KindHeader && position == "before":
			return "", fmt.Errorf("nothing goes before the header; insert after it, or before the first declaration")
		case position == "after":
			ch = codemodel.Change{Start: e.End, End: e.End, Text: sep + source}
		default:
			ch = codemodel.Change{Start: e.Start, End: e.Start, Text: source + sep}
		}
	}
	ee, err := applyChange(ctx, re, ch, nil)
	if err != nil {
		return "", err
	}
	if len(ee.out.Touched) == 0 {
		return "", fmt.Errorf("the source declares nothing new")
	}
	return ee.commit(ctx, "insert_element")
}

// deletionChange removes an element's whole lines, and one blank line with
// them when that would leave two in a row.
func deletionChange(f *codemodel.File, e *codemodel.Element) codemodel.Change {
	start := f.LineStart(e.StartLine)
	end := f.LineStart(e.EndLine + 1)
	if e.EndLine >= f.LineCount() {
		end = len(f.Source)
	}
	before, after := f.Source[:start], f.Source[end:]
	if strings.HasSuffix(before, "\n\n") && strings.HasPrefix(after, "\n") {
		end++
	} else if start > 0 && after == "" && strings.HasSuffix(before, "\n\n") {
		start--
	}
	return codemodel.Change{Start: start, End: end}
}

func executeDeleteElement(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	ref, _ := args["ref"].(string)
	rev, _ := args["revision"].(string)
	replaceWith, _ := args["replace_with"].(string)
	re, err := parsedTarget(ctx, path, ref)
	if err != nil {
		return "", err
	}
	if err := checkRevision(re, rev); err != nil {
		return "", err
	}
	if re.elem.Kind == codemodel.KindHeader {
		return "", fmt.Errorf("the header cannot be deleted; replace_element edits it")
	}
	ref = RefOf(re.file.rel, re.elem, re.refs)

	uses, err := usesOutside(ctx, re)
	if err != nil {
		return "", err
	}
	if len(uses) > 0 && strings.TrimSpace(replaceWith) == "" {
		return "", fmt.Errorf("refusing to delete %s: %d uses remain outside it --\n%s\nRewrite them first, or call again with replace_with (what they should point at) and paths (their files)", ref, len(uses), renderUses(uses))
	}
	if len(uses) > 0 {
		return repointAndDelete(ctx, re, uses, replaceWith, parseStringArray(args["paths"]))
	}
	ee, err := applyChange(ctx, re, deletionChange(re.model, re.elem), []string{re.elem.Key})
	if err != nil {
		return "", err
	}
	return ee.commit(ctx, "delete_element")
}

// usesOutside lists the uses of an element's name outside the element, from
// the index. A syntax_error region has no name to be used.
func usesOutside(ctx context.Context, re *resolvedElement) ([]StructureUse, error) {
	if re.elem.Kind == codemodel.KindSyntaxError || re.elem.Name == "_" || re.elem.Name == "init" {
		return nil, nil
	}
	provider := optionalStructureProvider()
	if provider == nil {
		return nil, fmt.Errorf("delete_element needs the structure index to check for remaining uses, and none is registered in this process")
	}
	_, uses, _, err := provider.Uses(ctx, RefOf(re.file.rel, re.elem, re.refs))
	if err != nil {
		return nil, err
	}
	return uses, nil
}

func renderUses(uses []StructureUse) string {
	sort.SliceStable(uses, func(i, j int) bool {
		if uses[i].File != uses[j].File {
			return uses[i].File < uses[j].File
		}
		return uses[i].Line < uses[j].Line
	})
	var sb strings.Builder
	for _, u := range uses {
		fmt.Fprintf(&sb, "  %s:%d  %s  in %s\n", u.File, u.Line, u.Match, u.Ref)
	}
	return strings.TrimSuffix(sb.String(), "\n")
}
