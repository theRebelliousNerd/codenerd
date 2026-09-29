package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/projectdoc"
	"codenerd/internal/tactile"
	"codenerd/internal/world/codemodel"
)

// RefuseAddedBuildExclusion projects the bytes a write tool would land and
// refuses the call when those bytes add a build constraint that excludes the
// file. It is the one projection: the session's guardWrite and the registry
// write guard both call it, so a tool reached through either path is judged
// the same way.
//
// Where the projection is uncertain it refuses, except the cases proven not
// to write or proven not to add a header constraint:
//
//   - A missing or ill-typed argument the tool itself rejects is not a
//     write (edit_file with no old_text, a non-integral line, an
//     edit_element anchor that is not unique inside its element, insert
//     before the header). Those return nil.
//   - delete_file removes a file. delete_element refuses to delete the
//     header (element_edit.go) and removing a declaration does not insert a
//     constraint line. repoint rewrites identifiers and imports from the
//     parse, not build tags.
//   - A non-.go path cannot carry a Go build constraint.
//   - A relative path with no workspace, a file that cannot be read, an
//     element ref that does not resolve in the file, and a write-mutation
//     tool this projection does not understand are refused. The structure
//     index may still resolve a ref this projection cannot see (canonical
//     refs are not on the call), and a guess that does not match the tool
//     would let a header edit through.
func RefuseAddedBuildExclusion(toolName string, args map[string]any, workspace string) error {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "delete_file", "delete_element", "repoint":
		return nil
	case "write_file", "fs_write":
		content, ok := argString(args, "content")
		return projectWholeFile(toolName, workspace, args, content, ok)
	case "create_file":
		return projectCreateFile(toolName, workspace, args)
	case "edit_file", "str_replace", "replace_in_file":
		return projectEditFile(toolName, workspace, args)
	case "edit_lines":
		return projectEditLines(toolName, workspace, args)
	case "insert_lines":
		return projectInsertLines(toolName, workspace, args)
	case "delete_lines":
		return projectDeleteLines(toolName, workspace, args)
	case "apply_edits", "multi_edit":
		return projectApplyEdits(workspace, args)
	case "edit_element":
		return projectEditElement(toolName, workspace, args)
	case "replace_element":
		return projectReplaceElement(toolName, workspace, args)
	case "insert_element":
		return projectInsertElement(toolName, workspace, args)
	default:
		// apply_patch is classified as a write and has no projector. A name
		// this switch does not understand is refused: returning nil would
		// let a header edit through a tool the projection cannot see.
		if projectdoc.IsWriteMutationTool(toolName) {
			return fmt.Errorf("refusing %s: this write is not projected, so it cannot be checked for a build exclusion", toolName)
		}
		return nil
	}
}

func projectWholeFile(toolName, workspace string, args map[string]any, content string, have bool) error {
	if !have {
		return nil
	}
	loc, err := locateGo(toolName, workspace, args)
	if err != nil || loc == nil {
		return err
	}
	return RejectAddedBuildExclusion(workspace, loc.abs, loc.before, []byte(content))
}

// projectCreateFile judges the bytes create_file would write. The tool
// gofmts the source before the commit (create_file.go), so the raw payload
// is not the file: a constraint gofmt keeps is the one that lands.
func projectCreateFile(toolName, workspace string, args map[string]any) error {
	source, ok := argString(args, "source")
	if !ok || strings.TrimSpace(source) == "" {
		return nil
	}
	loc, err := locateGo(toolName, workspace, args)
	if err != nil || loc == nil {
		return err
	}
	if loc.exists {
		// create_file refuses a path that already exists.
		return nil
	}
	source = strings.Trim(codemodel.Normalize(source), "\n") + "\n"
	model, ok := codemodel.Parse(loc.rel, source)
	if !ok || model == nil || !model.Parsed {
		return nil
	}
	formatted, err := codemodel.FormatUnit(source, true)
	if err != nil {
		return nil
	}
	return RejectAddedBuildExclusion(workspace, loc.abs, nil, []byte(formatted+"\n"))
}

func projectEditFile(toolName, workspace string, args map[string]any) error {
	oldText, newText, ok := editPair(args)
	if !ok || oldText == "" {
		return nil
	}
	loc, err := locateGo(toolName, workspace, args)
	if err != nil || loc == nil || !loc.exists {
		return err
	}
	content := tactile.NormalizeLineEnding(string(loc.before), "\n")
	oldText = tactile.NormalizeLineEnding(oldText, "\n")
	newText = tactile.NormalizeLineEnding(newText, "\n")
	if !strings.Contains(content, oldText) {
		// file_ops.stripLineNumberPrefixes: read_file shows "N\tline", and
		// the tool strips that prefix when every line has one. The same
		// recovery has to happen here or a pasted read would be judged as
		// a miss and the real edit would land unchecked.
		if stripped, ok := stripLineNumberPrefixes(oldText); ok && strings.Contains(content, stripped) {
			oldText = stripped
		} else {
			return nil
		}
	}
	replaceAll, _ := args["replace_all"].(bool)
	if !replaceAll && strings.Count(content, oldText) != 1 {
		return nil
	}
	var after string
	if replaceAll {
		after = strings.ReplaceAll(content, oldText, newText)
	} else {
		after = strings.Replace(content, oldText, newText, 1)
	}
	return RejectAddedBuildExclusion(workspace, loc.abs, loc.before, []byte(after))
}

func projectEditLines(toolName, workspace string, args map[string]any) error {
	startLine, ok := ArgIntStrict(args, "start_line")
	if !ok {
		return nil
	}
	endLine, ok := ArgIntStrict(args, "end_line")
	if !ok {
		return nil
	}
	// executeEditLines treats a missing or non-string new_content as empty.
	newContent, _ := args["new_content"].(string)
	return projectLineChange(toolName, workspace, args, func(lines []string) ([]string, bool) {
		if startLine < 1 || startLine > len(lines) || endLine < startLine || endLine > len(lines) {
			return nil, false
		}
		newLines := strings.Split(tactile.NormalizeLineEnding(newContent, "\n"), "\n")
		out := append([]string{}, lines[:startLine-1]...)
		out = append(out, newLines...)
		out = append(out, lines[endLine:]...)
		return out, true
	})
}

func projectInsertLines(toolName, workspace string, args map[string]any) error {
	insertContent, ok := argString(args, "content")
	if !ok || insertContent == "" {
		return nil
	}
	// executeInsertLines defaults a missing after_line to 0 (insert at the
	// beginning) even though the schema requires the key. The registry
	// validates after this guard on the session path; a caller that skips
	// validation would land a header constraint. Project the default.
	afterLine := 0
	if _, present := args["after_line"]; present {
		afterLine, ok = ArgIntStrict(args, "after_line")
		if !ok {
			return nil
		}
	}
	return projectLineChange(toolName, workspace, args, func(lines []string) ([]string, bool) {
		if afterLine < 0 || afterLine > len(lines) {
			return nil, false
		}
		newLines := strings.Split(tactile.NormalizeLineEnding(insertContent, "\n"), "\n")
		out := append([]string{}, lines[:afterLine]...)
		out = append(out, newLines...)
		out = append(out, lines[afterLine:]...)
		return out, true
	})
}

func projectDeleteLines(toolName, workspace string, args map[string]any) error {
	startLine, ok := ArgIntStrict(args, "start_line")
	if !ok {
		return nil
	}
	endLine, ok := ArgIntStrict(args, "end_line")
	if !ok {
		return nil
	}
	return projectLineChange(toolName, workspace, args, func(lines []string) ([]string, bool) {
		if startLine < 1 || startLine > len(lines) || endLine < startLine || endLine > len(lines) {
			return nil, false
		}
		out := append([]string{}, lines[:startLine-1]...)
		out = append(out, lines[endLine:]...)
		return out, true
	})
}

func projectApplyEdits(workspace string, args map[string]any) error {
	raw, ok := args["edits"]
	if !ok || raw == nil {
		return nil
	}
	var edits []map[string]any
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil
			}
			edits = append(edits, m)
		}
	case []map[string]any:
		edits = v
	default:
		return nil
	}
	// One file per edit, each applied to the original (apply_edits rejects
	// a duplicate path before it writes). An op the tool rejects as unknown
	// is not a write; a sibling that does exclude still refuses the call.
	for _, m := range edits {
		op, _ := m["operation"].(string)
		var err error
		switch op {
		case "edit_lines":
			err = projectEditLines(op, workspace, m)
		case "insert_lines":
			err = projectInsertLines(op, workspace, m)
		case "delete_lines":
			err = projectDeleteLines(op, workspace, m)
		default:
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func projectEditElement(toolName, workspace string, args map[string]any) error {
	oldText, ok := args["old"].(string)
	newText, hasNew := args["new"].(string)
	if !ok || !hasNew || oldText == "" {
		return nil
	}
	ref, ok := args["ref"].(string)
	if !ok || strings.TrimSpace(ref) == "" {
		return nil
	}
	loc, model, elem, err := resolveOne(toolName, workspace, ref, args)
	if err != nil || loc == nil || elem == nil {
		return err
	}
	if rev, ok := args["revision"].(string); ok && strings.TrimSpace(rev) != "" && rev != elem.Revision {
		return nil
	}
	spanStart, spanEnd := elem.Start, elem.End
	if part, _ := args["part"].(string); strings.TrimSpace(part) != "" {
		p, err := model.FindPart(elem, part)
		if err != nil {
			return nil
		}
		spanStart, spanEnd = p.Start, p.End
	}
	// The tool matches inside the element, not the file. "package p" can
	// occur again in a comment and still be unique in the header; judging
	// the first file-wide hit would miss that edit (H5a left it unprojected).
	start, end, ok := anchorIn(model, spanStart, spanEnd, codemodel.Normalize(oldText))
	if !ok {
		return nil
	}
	out, err := codemodel.Apply(model, codemodel.Change{Start: start, End: end, Text: newText}, []string{elem.Key}, nil)
	if err != nil || out == nil || out.File == nil || out.File.Source == model.Source {
		return nil
	}
	return RejectAddedBuildExclusion(workspace, loc.abs, loc.before, []byte(out.File.Source))
}

func projectReplaceElement(toolName, workspace string, args map[string]any) error {
	source, ok := args["source"].(string)
	if !ok || strings.TrimSpace(source) == "" {
		return nil
	}
	ref, ok := args["ref"].(string)
	if !ok || strings.TrimSpace(ref) == "" {
		return nil
	}
	loc, model, elem, err := resolveOne(toolName, workspace, ref, args)
	if err != nil || loc == nil || elem == nil {
		return err
	}
	if rev, ok := args["revision"].(string); ok && strings.TrimSpace(rev) != "" && rev != elem.Revision {
		return nil
	}
	// The header span starts at byte 0 and includes the build constraints.
	// Replacing it is how a //go:build ignore lands without a file-wide
	// search, so the result of Apply is judged, not the source string.
	out, err := codemodel.Apply(model, codemodel.Change{
		Start: elem.Start,
		End:   elem.End,
		Text:  strings.Trim(source, "\n"),
	}, []string{elem.Key}, nil)
	if err != nil || out == nil || out.File == nil || out.File.Source == model.Source || len(out.Touched) == 0 {
		return nil
	}
	return RejectAddedBuildExclusion(workspace, loc.abs, loc.before, []byte(out.File.Source))
}

func projectInsertElement(toolName, workspace string, args map[string]any) error {
	source, ok := args["source"].(string)
	if !ok || strings.TrimSpace(source) == "" {
		return nil
	}
	anchor, _ := args["anchor"].(string)
	if strings.TrimSpace(anchor) == "" {
		return nil
	}
	position, _ := args["position"].(string)
	position = strings.ToLower(strings.TrimSpace(position))
	if position == "" {
		position = "after"
	}
	if position != "before" && position != "after" {
		return nil
	}
	source = strings.Trim(codemodel.Normalize(source), "\n")
	if strings.TrimSpace(anchor) == codemodel.EndKey {
		return projectInsertAtEnd(toolName, workspace, args, source)
	}
	loc, model, elem, err := resolveOne(toolName, workspace, anchor, args)
	if err != nil || loc == nil || elem == nil {
		return err
	}
	// executeInsertElement refuses this before it builds a change.
	if elem.Kind == codemodel.KindHeader && position == "before" {
		return nil
	}
	sep := "\n\n"
	if elem.Grouped {
		sep = "\n"
	}
	var ch codemodel.Change
	if position == "after" {
		ch = codemodel.Change{Start: elem.End, End: elem.End, Text: sep + source}
	} else {
		// before a span that starts at 0 (a syntax_error covering the top of
		// the file) can add a header constraint. Apply is the bytes that
		// would land; a comment after the package clause is inert and the
		// judgement allows it.
		ch = codemodel.Change{Start: elem.Start, End: elem.Start, Text: source + sep}
	}
	out, err := codemodel.Apply(model, ch, nil, nil)
	if err != nil || out == nil || out.File == nil || out.File.Source == model.Source || len(out.Touched) == 0 {
		return nil
	}
	return RejectAddedBuildExclusion(workspace, loc.abs, loc.before, []byte(out.File.Source))
}

func projectInsertAtEnd(toolName, workspace string, args map[string]any, source string) error {
	loc, err := locateGo(toolName, workspace, args)
	if err != nil || loc == nil || !loc.exists {
		return err
	}
	model, ok := codemodel.Parse(loc.rel, string(loc.before))
	if !ok || model == nil {
		return fmt.Errorf("refusing %s on %s: the file has no element model, so it cannot be checked for a build exclusion", toolName, loc.abs)
	}
	tail := strings.TrimRight(model.Source, "\n")
	out, err := codemodel.Apply(model, codemodel.Change{
		Start: len(tail),
		End:   len(model.Source),
		Text:  "\n\n" + source + "\n",
	}, nil, nil)
	if err != nil || out == nil || out.File == nil || out.File.Source == model.Source || len(out.Touched) == 0 {
		return nil
	}
	return RejectAddedBuildExclusion(workspace, loc.abs, loc.before, []byte(out.File.Source))
}

func projectLineChange(toolName, workspace string, args map[string]any, apply func(lines []string) ([]string, bool)) error {
	loc, err := locateGo(toolName, workspace, args)
	if err != nil || loc == nil || !loc.exists {
		return err
	}
	lines := strings.Split(tactile.NormalizeLineEnding(string(loc.before), "\n"), "\n")
	out, ok := apply(lines)
	if !ok {
		return nil
	}
	return RejectAddedBuildExclusion(workspace, loc.abs, loc.before, []byte(strings.Join(out, "\n")))
}

// located is one .go path the projection can name. before is nil when the
// file does not exist yet; exists distinguishes that from an empty file.
type located struct {
	abs, rel string
	before   []byte
	exists   bool
}

func locateGo(toolName, workspace string, args map[string]any) (*located, error) {
	raw, ok := argPath(args)
	if !ok {
		return nil, nil
	}
	if !isGoPath(raw) {
		return nil, nil
	}
	clean := filepath.Clean(raw)
	var abs string
	if filepath.IsAbs(clean) {
		abs = clean
	} else if strings.TrimSpace(workspace) == "" {
		return nil, fmt.Errorf("refusing %s on %s: the path is relative and no workspace is set, so it cannot be checked for a build exclusion", toolName, raw)
	} else {
		abs = filepath.Join(workspace, clean)
	}
	rel := displayRel(workspace, abs)
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return &located{abs: abs, rel: rel}, nil
		}
		return nil, fmt.Errorf("refusing %s on %s: the file could not be read (%v), so it cannot be checked for a build exclusion", toolName, abs, err)
	}
	return &located{abs: abs, rel: rel, before: data, exists: true}, nil
}

func displayRel(workspace, abs string) string {
	if strings.TrimSpace(workspace) == "" {
		return filepath.ToSlash(abs)
	}
	rel, err := filepath.Rel(filepath.Clean(workspace), abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// resolveOne finds the one element ref names in the file. Zero matches is
// refused: with a structure index the tool's canonical ref can name an
// element this file-local lookup does not spell, and that edit can be the
// header. More than one match is the tool's own ambiguity error, which
// writes nothing.
func resolveOne(toolName, workspace, ref string, args map[string]any) (*located, *codemodel.File, *codemodel.Element, error) {
	loc, err := locateGo(toolName, workspace, args)
	if err != nil || loc == nil || !loc.exists {
		return nil, nil, nil, err
	}
	model, ok := codemodel.Parse(loc.rel, string(loc.before))
	if !ok || model == nil {
		return nil, nil, nil, fmt.Errorf("refusing %s on %s: the file has no element model, so it cannot be checked for a build exclusion", toolName, loc.abs)
	}
	matches := lookupProjected(model, loc.rel, strings.TrimSpace(ref))
	switch len(matches) {
	case 0:
		return nil, nil, nil, fmt.Errorf("refusing %s on %s: ref %q does not resolve in the file, so it cannot be checked for a build exclusion", toolName, loc.abs, strings.TrimSpace(ref))
	case 1:
		return loc, model, matches[0], nil
	default:
		return nil, nil, nil, nil
	}
}

// lookupProjected matches codedom.lookupInFile with no canonical-ref map.
// The index's spelling of a ref is not available here; a miss is refused by
// the caller rather than guessed.
func lookupProjected(model *codemodel.File, rel, ref string) []*codemodel.Element {
	for i := range model.Elements {
		e := &model.Elements[i]
		if refOfProjected(rel, e) == ref {
			return []*codemodel.Element{e}
		}
	}
	key := ref
	if base, file, ok := strings.Cut(key, "@"); ok && !strings.Contains(file, "/") && strings.HasSuffix(file, ".go") {
		if file != filepath.Base(rel) {
			return nil
		}
		key = base
	}
	prefix := "./"
	if dir := filepath.ToSlash(filepath.Dir(rel)); dir != "." && dir != "" {
		prefix = dir + "."
	}
	key = strings.TrimPrefix(key, prefix)
	return model.Lookup(key)
}

// refOfProjected matches codedom.RefOf when the index has no canonical ref.
func refOfProjected(rel string, e *codemodel.Element) string {
	if !strings.HasSuffix(strings.ToLower(rel), ".go") || e.Kind == codemodel.KindHeader || e.Kind == codemodel.KindSyntaxError {
		return rel + ":" + e.Key
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." || dir == "" {
		return "./" + e.Key
	}
	return dir + "." + e.Key
}

// anchorIn matches codedom.anchor. tools cannot import codedom (codedom
// imports tools), and a looser match would judge a different span than the
// one the tool writes. False means the tool rejects the call.
func anchorIn(f *codemodel.File, spanStart, spanEnd int, old string) (int, int, bool) {
	if spanStart < 0 || spanEnd > len(f.Source) || spanStart > spanEnd {
		return 0, 0, false
	}
	span := f.Source[spanStart:spanEnd]
	switch n := strings.Count(span, old); {
	case n == 1:
		i := strings.Index(span, old)
		return spanStart + i, spanStart + i + len(old), true
	case n > 1:
		return 0, 0, false
	}
	want := trimmedLines(old)
	if len(want) > 0 {
		startLine, endLine := f.LineOf(spanStart), f.LineOf(max(spanEnd-1, spanStart))
		var hits []int
		for l := startLine; l+len(want)-1 <= endLine; l++ {
			match := true
			for k := range want {
				if strings.TrimSpace(f.LineText(l+k, l+k)) != want[k] {
					match = false
					break
				}
			}
			if match {
				hits = append(hits, l)
			}
		}
		if len(hits) == 1 {
			lo := max(f.LineStart(hits[0]), spanStart)
			hi := f.LineStart(hits[0]+len(want)) - 1
			if hits[0]+len(want) > f.LineCount() {
				hi = len(f.Source)
			}
			return lo, min(hi, spanEnd), true
		}
	}
	return 0, 0, false
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

func argPath(args map[string]any) (string, bool) {
	if args == nil {
		return "", false
	}
	for _, key := range projectdoc.PathArgs {
		s, ok := args[key].(string)
		if ok && strings.TrimSpace(s) != "" {
			return s, true
		}
	}
	return "", false
}

// editPair reads the modular tool's old_text/new_text, or the VirtualStore
// shape old/new when that is what the call carried. A missing half is not
// projected: the tool rejects it, and a half-applied guess is not the edit.
func editPair(args map[string]any) (oldText, newText string, ok bool) {
	if s, yes := argString(args, "old_text"); yes {
		oldText = s
	} else if s, yes := argString(args, "old"); yes {
		oldText = s
	} else {
		return "", "", false
	}
	if s, yes := argString(args, "new_text"); yes {
		return oldText, s, true
	}
	if s, yes := argString(args, "new"); yes {
		return oldText, s, true
	}
	return "", "", false
}

func argString(args map[string]any, key string) (string, bool) {
	if args == nil {
		return "", false
	}
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func isGoPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".go")
}

// stripLineNumberPrefixes matches file_ops.stripLineNumberPrefixes. It is
// copied because that helper is unexported in internal/tools/core, and this
// package cannot import that one (it imports this package). The projection
// has to accept the same old_text the tool would.
func stripLineNumberPrefixes(s string) (string, bool) {
	if s == "" {
		return s, false
	}
	lines := strings.Split(s, "\n")
	out := make([]string, len(lines))
	for i, line := range lines {
		tab := strings.IndexByte(line, '\t')
		if tab <= 0 {
			return s, false
		}
		for _, r := range line[:tab] {
			if r < '0' || r > '9' {
				return s, false
			}
		}
		out[i] = line[tab+1:]
	}
	return strings.Join(out, "\n"), true
}
