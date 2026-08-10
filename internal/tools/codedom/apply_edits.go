package codedom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// Package-level serialization for commit. The preflight (snapshot + staging) can
// run concurrently, but the commit phase — optimistic check + writes + rollback —
// is serialized exactly like the single-file tools' implicit file lock.
var applyEditsMu sync.Mutex

// applyEditsWriteFile is the write seam for tests. Production uses os.WriteFile.
// Tests replace it to inject a failure mid-commit and exercise rollback.
var applyEditsWriteFile = os.WriteFile

// applyEditsBeforeCommitHook is a test seam invoked inside the commit mutex
// immediately before the optimistic conflict check. Tests use it to mutate a
// file after the snapshot but before the commit verifies it, proving optimistic
// conflict detection deterministically without a race.
var applyEditsBeforeCommitHook func()

// maxAggregateInputBytes is the aggregate size limit for new_content/content
// across all edits. One transaction should not be a bulk file copy.
const maxAggregateInputBytes = 1 << 20 // 1 MiB

// ApplyEditsTool returns the transactional multi-file CodeDOM edit tool.
func ApplyEditsTool() *tools.Tool {
	return &tools.Tool{
		Name:        "apply_edits",
		Description: "Transactionally apply 2-16 line edits across distinct existing files. Each entry is one of edit_lines, insert_lines, or delete_lines with the same arguments as the single-file tools. Every edit is staged and validated before commit; optimistic conflict checks and best-effort rollback protect against partial writes.",
		Category:    tools.CategoryCode,
		Priority:    80,
		Execute:     executeApplyEdits,
		Schema: tools.ToolSchema{
			Required: []string{"edits"},
			Properties: map[string]tools.Property{
				"edits": {
					Type:        "array",
					Description: "Array of 2-16 edit objects, each with operation and path plus operation-specific fields, one distinct existing file per edit",
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
		data, err := os.ReadFile(abs)
		if err != nil {
			return "", fmt.Errorf("failed to read %q: %w", pe.rawPath, err)
		}
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
	if err := ctx.Err(); err != nil {
		return "", err
	}
	stagingRoot, err := os.MkdirTemp("", "codedom-apply-*")
	if err != nil {
		return "", fmt.Errorf("failed to create staging workspace: %w", err)
	}
	defer os.RemoveAll(stagingRoot)
	for _, sn := range snaps {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		stagedPath := filepath.Join(stagingRoot, filepath.FromSlash(sn.relPath))
		if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
			return "", fmt.Errorf("staging mkdir failed for %s: %w", sn.relPath, err)
		}
		if err := os.WriteFile(stagedPath, sn.orig, sn.mode.Perm()); err != nil {
			return "", fmt.Errorf("staging write failed for %s: %w", sn.relPath, err)
		}
	}
	stagingCtx := context.WithValue(ctx, tools.CtxKeyWorkspaceRoot, stagingRoot)
	for i := range snaps {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		rel := snaps[i].relPath
		var execErr error
		switch snaps[i].edit.operation {
		case "edit_lines":
			_, execErr = executeEditLines(stagingCtx, map[string]any{
				"path":        rel,
				"start_line":  snaps[i].edit.startLine,
				"end_line":    snaps[i].edit.endLine,
				"new_content": snaps[i].edit.newContent,
			})
		case "insert_lines":
			_, execErr = executeInsertLines(stagingCtx, map[string]any{
				"path":       rel,
				"after_line": snaps[i].edit.afterLine,
				"content":    snaps[i].edit.content,
			})
		case "delete_lines":
			_, execErr = executeDeleteLines(stagingCtx, map[string]any{
				"path":       rel,
				"start_line": snaps[i].edit.startLine,
				"end_line":   snaps[i].edit.endLine,
			})
		default:
			execErr = fmt.Errorf("unknown operation %q", snaps[i].edit.operation)
		}
		if execErr != nil {
			return "", fmt.Errorf("preflight edits[%d] %s on %s failed: %w", i, snaps[i].edit.operation, snaps[i].edit.rawPath, execErr)
		}
	}
	planned := make(map[string][]byte, len(snaps))
	for i, sn := range snaps {
		stagedPath := filepath.Join(stagingRoot, filepath.FromSlash(sn.relPath))
		data, err := os.ReadFile(stagedPath)
		if err != nil {
			return "", fmt.Errorf("failed to read staged %s: %w", sn.relPath, err)
		}
		if bytes.Equal(data, sn.orig) {
			return "", fmt.Errorf("edits[%d] %s on %s produces no change", i, sn.edit.operation, sn.relPath)
		}
		planned[sn.absPath] = data
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	applyEditsMu.Lock()
	defer applyEditsMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if applyEditsBeforeCommitHook != nil {
		applyEditsBeforeCommitHook()
	}
	for _, sn := range snaps {
		cur, err := os.ReadFile(sn.absPath)
		if err != nil {
			return "", fmt.Errorf("optimistic conflict: failed to re-read %s: %w", sn.relPath, err)
		}
		if !bytes.Equal(cur, sn.orig) {
			return "", fmt.Errorf("optimistic conflict: file %s changed since snapshot", sn.relPath)
		}
	}
	type committed struct {
		absPath string
		relPath string
		planned []byte
		orig    []byte
		mode    os.FileMode
	}
	var succeeded []committed
	var commitErr error
	for _, sn := range snaps {
		if err := ctx.Err(); err != nil {
			commitErr = err
			break
		}
		cur, err := os.ReadFile(sn.absPath)
		if err != nil {
			commitErr = fmt.Errorf("optimistic conflict: failed to re-read %s immediately before write: %w", sn.relPath, err)
			break
		}
		if !bytes.Equal(cur, sn.orig) {
			commitErr = fmt.Errorf("optimistic conflict: file %s changed immediately before write", sn.relPath)
			break
		}
		p := planned[sn.absPath]
		if err := applyEditsWriteFile(sn.absPath, p, sn.mode); err != nil {
			commitErr = fmt.Errorf("failed to write %s: %w", sn.relPath, err)
			break
		}
		succeeded = append(succeeded, committed{
			absPath: sn.absPath,
			relPath: sn.relPath,
			planned: p,
			orig:    sn.orig,
			mode:    sn.mode,
		})
	}
	if commitErr != nil {
		var rollbackConflicts []string
		for j := len(succeeded) - 1; j >= 0; j-- {
			c := succeeded[j]
			cur, err := os.ReadFile(c.absPath)
			if err != nil {
				rollbackConflicts = append(rollbackConflicts, c.relPath)
				continue
			}
			if !bytes.Equal(cur, c.planned) {
				rollbackConflicts = append(rollbackConflicts, c.relPath)
				continue
			}
			if err := applyEditsWriteFile(c.absPath, c.orig, c.mode); err != nil {
				rollbackConflicts = append(rollbackConflicts, c.relPath+": restore failed: "+err.Error())
				continue
			}
		}
		if len(rollbackConflicts) > 0 {
			return "", fmt.Errorf("%w; rollback conflicts on: %s", commitErr, strings.Join(rollbackConflicts, ", "))
		}
		return "", commitErr
	}
	changed := make([]string, len(snaps))
	ops := make([]string, len(snaps))
	for i, sn := range snaps {
		changed[i] = sn.relPath
		ops[i] = sn.edit.operation
	}
	res := map[string]any{
		"changed":    changed,
		"operations": ops,
	}
	b, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	logging.VirtualStore("apply_edits completed: %d files (%s)", len(changed), strings.Join(changed, ", "))
	return string(b), nil
}

func applyCoerceInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		if n > int64(math.MaxInt) || n < int64(math.MinInt) {
			return 0, false
		}
		return int(n), true
	case uint:
		if n > uint(math.MaxInt) {
			return 0, false
		}
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		if uint64(n) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(n), true
	case uint64:
		if n > uint64(math.MaxInt) {
			return 0, false
		}
		return int(n), true
	case float32:
		return applyCoerceFloat(float64(n))
	case float64:
		return applyCoerceFloat(n)
	}
	return 0, false
}

func applyCoerceFloat(n float64) (int, bool) {
	// The upper bound is exclusive because float64(math.MaxInt) rounds up to
	// 2^63 on 64-bit platforms. Checking <= that rounded value would admit an
	// overflowing conversion.
	maxExclusive := float64(uint64(math.MaxInt) + 1)
	if math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) ||
		n < float64(math.MinInt) || n >= maxExclusive {
		return 0, false
	}
	return int(n), true
}
