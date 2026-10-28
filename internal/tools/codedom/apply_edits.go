package codedom

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/observation/precondition"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
)

// maxAggregateInputBytes is the aggregate size limit for new_content/content
// across all edits. One transaction should not be a bulk file copy.
const maxAggregateInputBytes = 1 << 20 // 1 MiB

// ApplyEditsTool returns the transactional multi-file CodeDOM edit tool.
func ApplyEditsTool() *tools.Tool {
	return &tools.Tool{
		Name:        "apply_edits",
		Description: "Transactionally apply 2-16 line edits across distinct existing files. Each entry is one of edit_lines, insert_lines, or delete_lines with the same arguments as the single-file tools, including precondition: a supplied read handle is checked against the file before anything is planned, exactly as the single-file tool checks it. Every edit is planned and validated before commit; optimistic conflict checks and best-effort rollback protect against partial writes.",
		Category:    tools.CategoryCode,
		Priority:    80,
		Execute:     executeApplyEdits,
		Schema: tools.ToolSchema{
			Required: []string{"edits"},
			Properties: map[string]tools.Property{
				"edits": {
					Type:        "array",
					Description: "Array of 2-16 edit objects, each with operation and path plus operation-specific fields, one distinct existing file per edit. Each object may carry " + precondition.Arg + ": " + precondition.ArgDescription,
					Items:       &tools.PropertyItems{Type: "object"},
				},
			},
		},
	}
}

type parsedEdit struct {
	operation  string
	rawPath    string
	absPath    string
	relPath    string
	startLine  int
	endLine    int
	afterLine  int
	newContent string
	content    string
	// precondition is the caller's read handle for this entry, "" when none
	// was supplied. It is checked against the original path and the exact
	// snapshot bytes the batch plans from, before any entry is planned;
	// minting a fresh handle would certify the batch's own read instead of
	// the caller's.
	precondition string
}

// Precondition states of one batch entry, reported per entry so a caller can
// tell an unchecked edit from a checked one.
const (
	preconditionNotSupplied         = "not_supplied"
	preconditionSatisfied           = "satisfied"
	preconditionSatisfiedWithCaveat = "satisfied_with_caveat"
)

// entryPrecondition is one entry's precondition outcome in the result.
type entryPrecondition struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Caveat string `json:"caveat,omitempty"`
}

func executeApplyEdits(ctx context.Context, args map[string]any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	rawEditsVal, ok := args["edits"]
	if !ok {
		return "", fmt.Errorf("edits is required")
	}
	var rawSlice []any
	switch v := rawEditsVal.(type) {
	case []any:
		rawSlice = v
	case []map[string]any:
		rawSlice = make([]any, len(v))
		for i, m := range v {
			rawSlice[i] = m
		}
	default:
		return "", fmt.Errorf("edits must be an array")
	}
	n := len(rawSlice)
	if n < 2 {
		return "", fmt.Errorf("edits must contain at least 2 entries (got %d)", n)
	}
	if n > 16 {
		return "", fmt.Errorf("edits must contain at most 16 entries (got %d)", n)
	}

	var parsed []parsedEdit
	aggregate := 0
	for idx, raw := range rawSlice {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		m, ok := raw.(map[string]any)
		if !ok {
			return "", fmt.Errorf("edits[%d]: malformed shape — expected object", idx)
		}
		opVal, ok := m["operation"]
		if !ok {
			return "", fmt.Errorf("edits[%d]: malformed shape — operation is required", idx)
		}
		op, ok := opVal.(string)
		if !ok || op == "" {
			return "", fmt.Errorf("edits[%d]: malformed shape — operation must be a string", idx)
		}
		if op != "edit_lines" && op != "insert_lines" && op != "delete_lines" {
			return "", fmt.Errorf("edits[%d]: malformed shape — unknown operation %q", idx, op)
		}
		rawPathVal, ok := m["path"]
		if !ok {
			return "", fmt.Errorf("edits[%d]: malformed shape — path is required", idx)
		}
		rawPath, ok := rawPathVal.(string)
		if !ok || strings.TrimSpace(rawPath) == "" {
			return "", fmt.Errorf("edits[%d]: malformed shape — path must be a non-empty string", idx)
		}
		pe := parsedEdit{operation: op, rawPath: rawPath}
		if pv, present := m[precondition.Arg]; present && pv != nil {
			ps, ok := pv.(string)
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — %s must be the string handle read_file reported", idx, precondition.Arg)
			}
			pe.precondition = strings.TrimSpace(ps)
		}
		switch op {
		case "edit_lines":
			sl, ok := applyCoerceInt(m["start_line"])
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — start_line is required for edit_lines", idx)
			}
			el, ok := applyCoerceInt(m["end_line"])
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — end_line is required for edit_lines", idx)
			}
			ncVal, ok := m["new_content"]
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — new_content is required for edit_lines", idx)
			}
			nc, ok := ncVal.(string)
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — new_content must be a string", idx)
			}
			pe.startLine = sl
			pe.endLine = el
			pe.newContent = nc
			aggregate += len(nc)
		case "insert_lines":
			al, ok := applyCoerceInt(m["after_line"])
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — after_line is required for insert_lines", idx)
			}
			cVal, ok := m["content"]
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — content is required for insert_lines", idx)
			}
			cStr, ok := cVal.(string)
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — content must be a string", idx)
			}
			if cStr == "" {
				return "", fmt.Errorf("edits[%d]: malformed shape — content is required for insert_lines", idx)
			}
			pe.afterLine = al
			pe.content = cStr
			aggregate += len(cStr)
		case "delete_lines":
			sl, ok := applyCoerceInt(m["start_line"])
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — start_line is required for delete_lines", idx)
			}
			el, ok := applyCoerceInt(m["end_line"])
			if !ok {
				return "", fmt.Errorf("edits[%d]: malformed shape — end_line is required for delete_lines", idx)
			}
			pe.startLine = sl
			pe.endLine = el
		}
		parsed = append(parsed, pe)
	}
	if aggregate > maxAggregateInputBytes {
		return "", fmt.Errorf("aggregate input too large: %d bytes exceeds %d byte limit", aggregate, maxAggregateInputBytes)
	}
	wsRoot, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	type snapshot struct {
		absPath string
		relPath string
		orig    []byte
		mode    os.FileMode
		edit    parsedEdit
	}
	var snaps []snapshot
	conditions := make([]entryPrecondition, 0, len(parsed))
	absSeen := make(map[string]int)
	for i, pe := range parsed {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		abs, err := tools.ResolveWorkspacePath(ctx, wsRoot, pe.rawPath)
		if err != nil {
			return "", fmt.Errorf("edits[%d] path %q: %w", i, pe.rawPath, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("edits[%d] path %q: file does not exist", i, pe.rawPath)
			}
			return "", fmt.Errorf("edits[%d] path %q: %w", i, pe.rawPath, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("edits[%d] path %q: is a directory, not a file", i, pe.rawPath)
		}
		rel, err := filepath.Rel(wsRoot, abs)
		if err != nil {
			return "", fmt.Errorf("edits[%d] path %q: %w", i, pe.rawPath, err)
		}
		if prev, ok := absSeen[abs]; ok {
			return "", fmt.Errorf("duplicate canonical path %q in edits[%d] and edits[%d]", rel, prev, i)
		}
		absSeen[abs] = i
		data, err := projectdoc.ReadFileForTool(abs)
		if err != nil {
			return "", fmt.Errorf("failed to read %q: %w", pe.rawPath, err)
		}
		// The caller's observation is checked on the original identity and the
		// exact bytes this batch will transform, before anything is planned. A
		// refused entry refuses the whole batch: no file has changed yet.
		cond := entryPrecondition{Path: filepath.ToSlash(rel), State: preconditionNotSupplied}
		if pe.precondition != "" {
			caveat, err := precondition.Enforce(map[string]any{precondition.Arg: pe.precondition}, abs, data)
			if err != nil {
				return "", fmt.Errorf("edits[%d] %s on %s: precondition refused, no file was changed: %w", i, pe.operation, pe.rawPath, err)
			}
			cond.State = preconditionSatisfied
			if caveat != "" {
				cond.State = preconditionSatisfiedWithCaveat
				cond.Caveat = caveat
			}
		}
		conditions = append(conditions, cond)
		parsed[i].absPath = abs
		parsed[i].relPath = filepath.ToSlash(rel)
		snaps = append(snaps, snapshot{
			absPath: abs,
			relPath: filepath.ToSlash(rel),
			orig:    data,
			mode:    info.Mode(),
			edit:    parsed[i],
		})
	}
	// Every entry is planned in memory against its snapshot with the same
	// planner its single-file verb uses, so all validation precedes any write
	// and nothing is reported as changed until the commit lands.
	writes := make([]plannedWrite, 0, len(snaps))
	for i, sn := range snaps {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		var plan lineEditPlan
		var planErr error
		switch sn.edit.operation {
		case "edit_lines":
			plan, planErr = planEditLines(sn.absPath, sn.orig, sn.edit.startLine, sn.edit.endLine, sn.edit.newContent)
		case "insert_lines":
			plan, planErr = planInsertLines(sn.absPath, sn.orig, sn.edit.afterLine, sn.edit.content)
		case "delete_lines":
			plan, planErr = planDeleteLines(sn.absPath, sn.orig, sn.edit.startLine, sn.edit.endLine)
		default:
			planErr = fmt.Errorf("unknown operation %q", sn.edit.operation)
		}
		if planErr != nil {
			return "", fmt.Errorf("preflight edits[%d] %s on %s failed: %w", i, sn.edit.operation, sn.edit.rawPath, planErr)
		}
		if plan.output == string(sn.orig) {
			return "", fmt.Errorf("edits[%d] %s on %s produces no change", i, sn.edit.operation, sn.relPath)
		}
		writes = append(writes, plannedWrite{abs: sn.absPath, rel: sn.relPath, orig: sn.orig, data: []byte(plan.output), mode: sn.mode})
	}
	if err := commitFiles(ctx, writes); err != nil {
		return "", err
	}
	for _, w := range writes {
		tools.RecordChangedSource(ctx, w.abs, string(w.orig), string(w.data))
	}
	changed := make([]string, len(snaps))
	ops := make([]string, len(snaps))
	for i, sn := range snaps {
		changed[i] = sn.relPath
		ops[i] = sn.edit.operation
	}
	res := map[string]any{
		"changed":       changed,
		"operations":    ops,
		"preconditions": conditions,
	}
	b, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	logging.Tools("apply_edits completed: %d files (%s)", len(changed), strings.Join(changed, ", "))
	return string(b), nil
}

// applyCoerceInt reads edit line addresses through the canonical strict helper.
// Batch edits fail their whole commit on one malformed bound, so fractional,
// non-finite, and out-of-range values are refused rather than coerced onto a
// neighboring line. (The overflow guards this wrapper once carried locally now
// live in tools.coerceFloat, where every strict call site shares them.)
func applyCoerceInt(v any) (int, bool) {
	return tools.CoerceIntStrict(v)
}
