package session

import (
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
)

// refuseAddedBuildExclusion projects the bytes a write tool would land and
// refuses the call when those bytes add a build constraint that excludes the
// file. The projection mirrors the tool: where the tool would itself reject
// the call (old text missing, line out of range, a non-unique edit_element
// replacement), nothing is judged, because a guess that does not match the
// tool would refuse a write that cannot happen or miss one that can.
// replace_element and insert_element are not projected. A header constraint
// is outside every element span except the header itself, insert before the
// header is already refused by the tool, and replace_element's span is the
// parsed element -- judging the source as a whole file would refuse a
// replacement that never lands in the header.
func refuseAddedBuildExclusion(toolName string, args map[string]any, workspace string) error {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "delete_file", "delete_element", "repoint", "replace_element", "insert_element":
		return nil
	case "write_file", "fs_write":
		content, ok := argString(args, "content")
		return projectWholeFile(workspace, args, content, ok)
	case "create_file":
		source, ok := argString(args, "source")
		return projectWholeFile(workspace, args, source, ok)
	case "edit_file", "str_replace", "replace_in_file":
		return projectEditFile(workspace, args)
	case "edit_lines":
		return projectEditLines(workspace, args)
	case "insert_lines":
		return projectInsertLines(workspace, args)
	case "delete_lines":
		return projectDeleteLines(workspace, args)
	case "apply_edits", "multi_edit":
		return projectApplyEdits(workspace, args)
	case "edit_element":
		return projectEditElement(workspace, args)
	default:
		return nil
	}
}

func projectWholeFile(workspace string, args map[string]any, content string, have bool) error {
	if !have {
		return nil
	}
	abs, ok := projectedAbs(workspace, args)
	if !ok {
		return nil
	}
	before, _, ok := readProjected(abs)
	if !ok {
		return nil
	}
	return core.RejectAddedBuildExclusion(workspace, abs, before, []byte(content))
}

func projectEditFile(workspace string, args map[string]any) error {
	oldText, newText, ok := editPair(args)
	if !ok || oldText == "" {
		return nil
	}
	abs, ok := projectedAbs(workspace, args)
	if !ok || !isGoPath(abs) {
		return nil
	}
	before, exists, ok := readProjected(abs)
	if !ok || !exists {
		return nil
	}
	content := tactile.NormalizeLineEnding(string(before), "\n")
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
	return core.RejectAddedBuildExclusion(workspace, abs, before, []byte(after))
}

func projectEditLines(workspace string, args map[string]any) error {
	startLine, ok := tools.ArgIntStrict(args, "start_line")
	if !ok {
		return nil
	}
	endLine, ok := tools.ArgIntStrict(args, "end_line")
	if !ok {
		return nil
	}
	newContent := ""
	if v, present := args["new_content"]; present {
		s, ok := v.(string)
		if !ok {
			return nil
		}
		newContent = s
	}
	return projectLineChange(workspace, args, func(lines []string) ([]string, bool) {
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

func projectInsertLines(workspace string, args map[string]any) error {
	if _, present := args["after_line"]; !present {
		return nil
	}
	afterLine, ok := tools.ArgIntStrict(args, "after_line")
	if !ok {
		return nil
	}
	insertContent, ok := argString(args, "content")
	if !ok || insertContent == "" {
		return nil
	}
	return projectLineChange(workspace, args, func(lines []string) ([]string, bool) {
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

func projectDeleteLines(workspace string, args map[string]any) error {
	startLine, ok := tools.ArgIntStrict(args, "start_line")
	if !ok {
		return nil
	}
	endLine, ok := tools.ArgIntStrict(args, "end_line")
	if !ok {
		return nil
	}
	return projectLineChange(workspace, args, func(lines []string) ([]string, bool) {
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
	// a duplicate path before it writes). An op whose shape the tool would
	// reject is skipped; a sibling that does exclude still refuses the call.
	for _, m := range edits {
		op, _ := m["operation"].(string)
		var err error
		switch op {
		case "edit_lines":
			err = projectEditLines(workspace, m)
		case "insert_lines":
			err = projectInsertLines(workspace, m)
		case "delete_lines":
			err = projectDeleteLines(workspace, m)
		default:
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func projectEditElement(workspace string, args map[string]any) error {
	oldText, ok := args["old"].(string)
	newText, hasNew := args["new"].(string)
	if !ok || !hasNew || oldText == "" {
		return nil
	}
	abs, ok := projectedAbs(workspace, args)
	if !ok || !isGoPath(abs) {
		return nil
	}
	before, exists, ok := readProjected(abs)
	if !ok || !exists {
		return nil
	}
	content := tactile.NormalizeLineEnding(string(before), "\n")
	oldText = tactile.NormalizeLineEnding(oldText, "\n")
	newText = tactile.NormalizeLineEnding(newText, "\n")
	// The tool matches inside one element, and falls back to a
	// whitespace-insensitive line run. A replacement that is not unique in
	// the whole file is not projected: the tool may still apply it inside
	// the element, and a first-match guess would judge the wrong site.
	if strings.Count(content, oldText) != 1 {
		return nil
	}
	after := strings.Replace(content, oldText, newText, 1)
	return core.RejectAddedBuildExclusion(workspace, abs, before, []byte(after))
}

func projectLineChange(workspace string, args map[string]any, apply func(lines []string) ([]string, bool)) error {
	abs, ok := projectedAbs(workspace, args)
	if !ok || !isGoPath(abs) {
		return nil
	}
	before, exists, ok := readProjected(abs)
	if !ok || !exists {
		return nil
	}
	lines := strings.Split(tactile.NormalizeLineEnding(string(before), "\n"), "\n")
	out, ok := apply(lines)
	if !ok {
		return nil
	}
	return core.RejectAddedBuildExclusion(workspace, abs, before, []byte(strings.Join(out, "\n")))
}

func projectedAbs(workspace string, args map[string]any) (string, bool) {
	raw, ok := argPath(args)
	if !ok {
		return "", false
	}
	clean := filepath.Clean(raw)
	if filepath.IsAbs(clean) {
		return clean, true
	}
	if strings.TrimSpace(workspace) == "" {
		return "", false
	}
	return filepath.Join(workspace, clean), true
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

func readProjected(abs string) (before []byte, exists, ok bool) {
	data, err := os.ReadFile(abs)
	if err == nil {
		return data, true, true
	}
	if os.IsNotExist(err) {
		return nil, false, true
	}
	return nil, false, false
}

func isGoPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".go")
}

// stripLineNumberPrefixes matches file_ops.stripLineNumberPrefixes. It is
// copied because that helper is unexported in another package, and the
// projection has to accept the same old_text the tool would.
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
