package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/build/constraint"
	"go/version"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/build"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/observation/precondition"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// handleReadFile reads a file from disk.
func (v *VirtualStore) handleReadFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleReadFile")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)
	logging.VirtualStoreDebug("Reading file: %s", path)

	info, err := os.Stat(path)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "file_read_error", Args: []any{path, err.Error()}},
			},
		}, nil
	}

	if info.IsDir() {
		return v.handleReadDirectory(ctx, path)
	}

	// execution.max_read_file_bytes: 0 reads the file whole. A positive
	// ceiling refuses the read. file_content is the bytes that were read,
	// and the observation codec projects the model view from those bytes.
	// A prefix stored as the file made the edit precondition a prefix too.
	ceiling := v.readFileByteCeiling()
	if ceiling > 0 && info.Size() > ceiling {
		msg := fmt.Sprintf("refusing to read %s: %d bytes exceeds execution.max_read_file_bytes (%d)", path, info.Size(), ceiling)
		return ActionResult{
			Success: false,
			Error:   msg,
			FactsToAdd: []Fact{
				{Predicate: "file_read_error", Args: []any{path, msg}},
			},
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "file_read_error", Args: []any{path, err.Error()}},
			},
		}, nil
	}

	content := string(data)
	modTime := info.ModTime().Unix()
	timestamp := time.Now().Unix()

	// file_content keeps the whole of what was read. The kernel's record of the
	// file is not the model's view of it, and shrinking the fact to the
	// projection would quietly change what has_file_content means in
	// coder_workflow.mg.
	facts := []Fact{
		{Predicate: "file_content", Args: []any{path, content}},
		{Predicate: "file_read", Args: []any{path, req.SessionID, timestamp}},
	}

	// The Output is shaped by the file-read codec, and the precondition it
	// mints is redeemable by the edit tools even though this is the action
	// path: the store is process-wide for exactly that reason. Reading through
	// the kernel and editing through a tool is the ordinary shape of a turn,
	// and a precondition that only worked when both halves came from the same
	// package would be unusable in it.
	start, _ := tools.ArgInt(req.Payload, "start_line")
	end, _ := tools.ArgInt(req.Payload, "end_line")
	// Path is the resolved absolute and Display is the pretty name, and they are
	// separate here for a reason this call site is the proof of: this action
	// resolves against v.workingDir, which is allowed to be a subdirectory of
	// the session workspace the edit tools resolve against. A workspace-relative
	// identity would be two strings for one file, and a precondition is refused
	// outright when the two sides name it differently.
	// The projection bounds come from observation.* via the installed
	// policy: observation is a leaf package and cannot read config, so the
	// caller that builds ReadLimits passes the resolved values in.
	obsLimits := config.ResolvedObservationLimits()
	result := observation.EncodeRead(precondition.Read{
		Path:      path,
		Display:   tools.WorkspaceDisplayPath(tools.WithWorkspaceRoot(ctx, v.workingDir), path),
		Content:   content,
		Start:     start,
		End:       end,
		Truncated: false,
	}, observation.ReadLimits{
		MaxRegionLines: obsLimits.MaxRegionLines,
		PadLines:       obsLimits.PadLines,
		MaxOutline:     obsLimits.MaxOutline,
		MaxRegionBytes: obsLimits.MaxRegionBytes,
	})

	logging.VirtualStore("File read: path=%s, size=%d", path, info.Size())
	return ActionResult{
		Success: true,
		Output:  result.Text(),
		Metadata: map[string]any{
			"path":      path,
			"size":      info.Size(),
			"modified":  modTime,
			"truncated": false,
			"handle":    result.Handle,
		},
		FactsToAdd: facts,
	}, nil
}

// handleReadDirectory reads a directory and returns a summary.
func (v *VirtualStore) handleReadDirectory(ctx context.Context, dirPath string) (ActionResult, error) {
	logging.VirtualStoreDebug("Reading directory: %s", dirPath)

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "dir_read_error", Args: []any{dirPath, err.Error()}},
			},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory: %s\n\n", dirPath))

	var dirs, files []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name()+"/")
		} else {
			files = append(files, entry.Name())
		}
	}

	if len(dirs) > 0 {
		sb.WriteString("Subdirectories:\n")
		for _, d := range dirs {
			sb.WriteString(fmt.Sprintf("  %s\n", d))
		}
		sb.WriteString("\n")
	}

	if len(files) > 0 {
		sb.WriteString("Files:\n")
		for _, f := range files {
			info, err := os.Stat(filepath.Join(dirPath, f))
			if err == nil {
				sb.WriteString(fmt.Sprintf("  %s (%d bytes)\n", f, info.Size()))
			} else {
				sb.WriteString(fmt.Sprintf("  %s\n", f))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\nTotal: %d directories, %d files\n", len(dirs), len(files)))

	return ActionResult{
		Success: true,
		Output:  sb.String(),
		Metadata: map[string]any{
			"path":        dirPath,
			"is_dir":      true,
			"dir_count":   len(dirs),
			"file_count":  len(files),
			"total_count": len(entries),
		},
		FactsToAdd: []Fact{
			{Predicate: "dir_read", Args: []any{dirPath, int64(len(entries))}},
		},
	}, nil
}

// handleWriteFile writes content to a file.
func (v *VirtualStore) handleWriteFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleWriteFile")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

	content, ok := req.Payload["content"].(string)
	if !ok {
		logging.Get(logging.CategoryVirtualStore).Error("write_file missing content in payload")
		return ActionResult{}, fmt.Errorf("write_file requires 'content' in payload")
	}

	// Extract code block from content (removes LLM reasoning traces and markdown fences).
	// asked is kept: the extractor slices a Go file from its package clause and
	// drops a leading //go:build line, so the exclusion check has to see the
	// bytes the caller asked for, not only what would land.
	originalLen := len(content)
	asked := content
	content = extractCodeBlockForFile(content, path)
	if len(content) != originalLen {
		logging.VirtualStoreDebug("Extracted code block for %s: %d -> %d bytes", path, originalLen, len(content))
	}

	logging.VirtualStoreDebug("Writing file: %s (%d bytes)", path, len(content))

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to create directory %s: %v", dir, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Overwriting an existing file keeps that file's line ending; a new file
	// keeps the previous behaviour. See internal/core/line_ending.go.
	ending, exists, err := existingLineEnding(path)
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	if exists {
		content = normalizeLineEnding(content, ending)
	}

	// A write that hides its own file from the build is refused before it
	// lands (dogfood run 5: a repair added //go:build ignore to make the
	// build pass). before nil is a new file; a file that cannot be read
	// for another reason skips the check rather than failing the write.
	// Both the asked bytes and the bytes that would land are judged: a
	// constraint the extractor would strip is still a constraint the
	// caller asked to add.
	if before, readErr := os.ReadFile(path); readErr == nil || os.IsNotExist(readErr) {
		if readErr != nil {
			before = nil
		}
		for _, candidate := range [][]byte{[]byte(asked), []byte(content)} {
			if refuse := v.rejectBuildExclusion(path, before, candidate); refuse != nil {
				logging.Get(logging.CategoryVirtualStore).Warn("Write refused: %v", refuse)
				return ActionResult{Success: false, Error: refuse.Error()}, nil
			}
		}
	}

	err = atomicfile.WriteFilePreservingMode(path, []byte(content), 0o644)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to write file %s: %v", path, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "file_write_error", Args: []any{path, err.Error()}},
			},
		}, nil
	}

	// Calculate hash
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])
	timestamp := time.Now().Unix()

	logging.VirtualStore("File written: path=%s, bytes=%d", path, len(content))
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Written %d bytes to %s", len(content), path),
		FactsToAdd: []Fact{
			{Predicate: "file_written", Args: []any{v.factPath(path), hashStr, req.SessionID, timestamp}},
			{Predicate: "modified", Args: []any{v.factPath(path)}},
		},
	}, nil
}

// handleEditFile performs a search-and-replace edit on a file.
func (v *VirtualStore) handleEditFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleEditFile")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

	oldContent, ok := req.Payload["old"].(string)
	if !ok {
		logging.Get(logging.CategoryVirtualStore).Error("edit_file missing 'old' in payload")
		return ActionResult{}, fmt.Errorf("edit_file requires 'old' in payload")
	}
	newContent, ok := req.Payload["new"].(string)
	if !ok {
		logging.Get(logging.CategoryVirtualStore).Error("edit_file missing 'new' in payload")
		return ActionResult{}, fmt.Errorf("edit_file requires 'new' in payload")
	}

	logging.VirtualStoreDebug("Editing file: %s (old_len=%d, new_len=%d)", path, len(oldContent), len(newContent))

	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to read file for edit %s: %v", path, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Match in LF space so model-emitted multi-line text can target either LF
	// or CRLF files. Restore the file's original convention after replacement.
	originalEnding := detectLineEnding(data)
	content := normalizeLineEnding(string(data), "\n")
	oldContent = normalizeLineEnding(oldContent, "\n")
	newContent = normalizeLineEnding(newContent, "\n")
	if !strings.Contains(content, oldContent) {
		logging.Get(logging.CategoryVirtualStore).Warn("Edit failed: pattern not found in %s", path)
		return ActionResult{
			Success: false,
			Error:   "old content not found in file",
			FactsToAdd: []Fact{
				{Predicate: "edit_failed", Args: []any{path, types.Atom("pattern_not_found")}},
			},
		}, nil
	}

	newFileContent := strings.Replace(content, oldContent, newContent, 1)

	// The spliced-in replacement carries whatever line ending the model emitted,
	// which is how a CRLF file ended up with 36 lone LFs in it. Re-normalize the
	// whole file to its own convention. The file exists by construction here —
	// it was just read — so this branch always applies.
	newFileContent = normalizeLineEnding(newFileContent, originalEnding)

	// Like the write path: an edit that hides its own file from the build
	// is refused before it lands. content is the file as read; only added
	// constraint lines are judged, so a file that already carried one is
	// not affected.
	if refuse := v.rejectBuildExclusion(path, []byte(content), []byte(newFileContent)); refuse != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Edit refused: %v", refuse)
		return ActionResult{
			Success: false,
			Error:   refuse.Error(),
			FactsToAdd: []Fact{
				{Predicate: "edit_failed", Args: []any{path, types.Atom("build_exclusion")}},
			},
		}, nil
	}

	err = atomicfile.WriteFilePreservingMode(path, []byte(newFileContent), 0o644)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to write edited file %s: %v", path, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	logging.VirtualStore("File edited: %s", path)
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Edited %s", path),
		FactsToAdd: []Fact{
			{Predicate: "file_edited", Args: []any{v.factPath(path)}},
			{Predicate: "modified", Args: []any{v.factPath(path)}},
		},
	}, nil
}

// handleDeleteFile deletes a file (requires explicit confirmation flag).
func (v *VirtualStore) handleDeleteFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

	logging.VirtualStoreDebug("Delete file requested: %s", path)

	confirmed, _ := req.Payload["confirmed"].(bool)
	if !confirmed {
		logging.Get(logging.CategoryVirtualStore).Warn("Delete blocked: no confirmation for %s", path)
		return ActionResult{
			Success: false,
			Error:   "delete_file requires 'confirmed: true' in payload",
			FactsToAdd: []Fact{
				{Predicate: "delete_blocked", Args: []any{path, types.Atom("no_confirmation")}},
			},
		}, nil
	}

	err := os.Remove(path)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to delete file %s: %v", path, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	logging.VirtualStore("File deleted: %s", path)
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Deleted %s", path),
		FactsToAdd: []Fact{
			{Predicate: "file_deleted", Args: []any{path}},
		},
	}, nil
}

// handleSearchCode searches for code patterns using local filesystem search.
// For semantic/AST-based search, use the internal/world package via shards.
//
// The result is shaped by the code-search observation codec rather than
// returned as the lines that matched. A wall of "path:line:text" is the most
// expensive way to convey the least structure: whoever reads it — the console,
// the routing_result excerpt, or a model further down — has to re-derive which
// symbols these are and what depends on what from text they already paid for.
// The lines themselves are retained under the handle in the output and are
// readable with the search_expand tool, which reads the retained bytes and
// never runs the search again.
func (v *VirtualStore) handleSearchCode(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleSearchCode")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	pattern := req.Target
	facts := make([]Fact, 0)
	observed := observation.Search{Query: pattern}
	// sources maps each displayed path back to the file the walk actually
	// visited, so projection can only open files this search already opened.
	sources := make(map[string]string)
	// The walk loads every visited file whole. execution.max_search_file_bytes
	// is the process bound on that read. A file over it is named in the
	// result; it is not a match that was dropped.
	ceiling := v.searchFileByteCeiling()
	skipped := 0

	// Local search using filepath.Walk
	err := filepath.Walk(v.workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if strings.Contains(path, ".git") || strings.Contains(path, ".nerd") {
			return nil
		}
		if info.Size() > ceiling {
			skipped++
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := string(data)
		lines := strings.Split(content, "\n")
		relPath, _ := filepath.Rel(v.workingDir, path)

		for i, line := range lines {
			if strings.Contains(line, pattern) {
				lineNum := i + 1
				// search_result stays as it is, deliberately. It is the raw
				// record, no rule reads it, and re-pointing it at
				// code_defines/code_calls would make this a second writer into
				// predicates the Cartographer replaces per file — its next deep
				// scan would silently delete whatever a search had asserted.
				// See internal/world/world_predicates.go for the ownership
				// matrix that records why that hurts.
				facts = append(facts, Fact{
					Predicate: "search_result",
					Args: []any{
						relPath,
						lineNum,
						strings.TrimSpace(line),
					},
				})
				display := filepath.ToSlash(relPath)
				sources[display] = path
				observed.Match = append(observed.Match, observation.Match{
					File: display,
					Line: lineNum,
					Text: strings.TrimSpace(line),
				})
			}
		}
		return nil
	})

	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	// The codec is the process-wide one, shared with the search_code tool: a
	// handle is minted here and redeemed by a different call site entirely, and
	// two stores would make every handle either path published unredeemable.
	result := observation.Shared().Encode(observed, func(file string) ([]byte, error) {
		abs, ok := sources[file]
		if !ok {
			return nil, fmt.Errorf("%s was not part of this search", file)
		}
		return os.ReadFile(abs)
	}, observation.Limits{})

	logging.VirtualStoreDebug("Local search returned %d results in %d symbol(s), %d edge(s), %d file(s) over the size ceiling",
		len(facts), len(result.Symbols), len(result.Edges), skipped)
	output := result.Text(toolscore.SearchExpandToolName)
	if skipped > 0 {
		output += fmt.Sprintf("\n%d file(s) over %d bytes were not searched (execution.max_search_file_bytes)", skipped, ceiling)
	}
	return ActionResult{
		Success:    true,
		Output:     output,
		FactsToAdd: facts,
		Metadata: map[string]any{
			"matches": result.Matches,
			"files":   result.Files,
			"handle":  result.Handle,
		},
	}, nil
}

// RejectAddedBuildExclusion refuses a .go write that adds a build constraint
// excluding the file from the current build. Dogfood run 5 (2026-09-29): a
// repair loop hid another agent's compile failure by adding `//go:build
// ignore` to files outside its write set, making the build pass by deleting
// code from it. Hiding a file from the build is never a fix for a failing
// one, so like RejectUnparseableGo this is a mechanical refusal at the write
// path, not a policy verdict: the bytes before and after are in hand here,
// and Mangle cannot parse constraints (Go parses, Mangle decides).
// constitution.mg should not carry it. A string match on the payload fires
// on a mention and cannot spare a file that already had the constraint; the
// before and after bytes are what make that distinction, and they are not
// facts the kernel holds.
//
// Only added lines are judged, and only when they flip the file from
// included to excluded under the current GOOS, GOARCH and effective build
// tags: a file that already carried the constraint is not affected.
// path gates on the .go extension; before nil is a new file. root is the
// workspace whose gate tags (TestTagsForWorkspace) join GOFLAGS.
func RejectAddedBuildExclusion(root, path string, before, after []byte) error {
	if !strings.EqualFold(filepath.Ext(path), ".go") {
		return nil
	}
	beforeGo, beforePlus := headerBuildConstraints(before)
	beforeSet := make(map[string]bool, len(beforeGo)+len(beforePlus))
	for _, line := range beforeGo {
		beforeSet["go:"+line] = true
	}
	for _, line := range beforePlus {
		beforeSet["plus:"+line] = true
	}
	afterGo, afterPlus := headerBuildConstraints(after)
	var added []string
	for _, line := range afterGo {
		if !beforeSet["go:"+line] {
			added = append(added, "//go:build "+line)
		}
	}
	for _, line := range afterPlus {
		if !beforeSet["plus:"+line] {
			added = append(added, "// +build "+line)
		}
	}
	if len(added) == 0 {
		return nil
	}
	tags := currentBuildTags(root)
	if buildConstraintsExclude(afterGo, afterPlus, tags) && !buildConstraintsExclude(beforeGo, beforePlus, tags) {
		return fmt.Errorf("refusing to write %s: the edit adds %s, which excludes this file from the current build (GOOS=%s GOARCH=%s tags=%s); hiding a file from the build is not a fix",
			path, strings.Join(added, ", "), runtime.GOOS, runtime.GOARCH, formatBuildTags(tags))
	}
	return nil
}

func (v *VirtualStore) rejectBuildExclusion(path string, before, after []byte) error {
	root := ""
	if v != nil {
		root = v.workspaceRoot
		if root == "" {
			root = v.workingDir
		}
	}
	return RejectAddedBuildExclusion(root, path, before, after)
}

// currentBuildTags is the tag set the current build evaluates constraints
// against: the workspace's gate tags (TestTagsForWorkspace: this repo's
// gates build with sqlite_vec, so a sqlite_vec-gated file is not hidden
// here) plus whatever GOFLAGS carries in this process.
func currentBuildTags(root string) map[string]bool {
	tags := map[string]bool{}
	addTags := func(value string) {
		for _, field := range strings.FieldsFunc(value, func(r rune) bool { return r == ' ' || r == '\t' || r == ',' }) {
			if field = strings.Trim(field, `"'`); field != "" {
				tags[field] = true
			}
		}
	}
	flags := build.TestTagsForWorkspace(root)
	for i := 0; i < len(flags); i++ {
		if flags[i] == "-tags" && i+1 < len(flags) {
			i++
			addTags(flags[i])
		} else if rest, ok := strings.CutPrefix(flags[i], "-tags="); ok {
			addTags(rest)
		}
	}
	goflags := strings.Fields(os.Getenv("GOFLAGS"))
	for i := 0; i < len(goflags); i++ {
		if goflags[i] == "-tags" && i+1 < len(goflags) {
			i++
			addTags(goflags[i])
		} else if rest, ok := strings.CutPrefix(goflags[i], "-tags="); ok {
			addTags(rest)
		}
	}
	return tags
}

func formatBuildTags(tags map[string]bool) string {
	if len(tags) == 0 {
		return "[]"
	}
	names := make([]string, 0, len(tags))
	for name := range tags {
		names = append(names, name)
	}
	sort.Strings(names)
	return "[" + strings.Join(names, " ") + "]"
}

// headerBuildConstraints returns the //go:build and // +build expressions a
// .go file carries in its header: line comments before the package clause,
// matched the way the toolchain honors them. A bare mention inside other
// text (a string literal, a prose comment, a constraint after the package
// clause) is not a constraint, and a file with no package clause builds
// nowhere, so nothing in it can hide it. The toolchain also requires a blank
// line after the constraint paragraph; without one the lines are inert, and
// refusing an edit for inert lines would be a false positive.
func headerBuildConstraints(content []byte) (goLines, plusLines []string) {
	lines := strings.Split(string(content), "\n")
	packageIdx := -1
	for i, line := range lines {
		fields := strings.Fields(strings.TrimSuffix(line, "\r"))
		if len(fields) > 0 && fields[0] == "package" {
			packageIdx = i
			break
		}
	}
	if packageIdx < 0 {
		return nil, nil
	}
	type candidate struct {
		idx        int
		goBuild    bool
		expression string
	}
	var found []candidate
	for i := 0; i < packageIdx; i++ {
		trimmed := strings.TrimSuffix(lines[i], "\r")
		trimmed = strings.TrimSpace(trimmed)
		rest, ok := strings.CutPrefix(trimmed, "//")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		if expr, ok := strings.CutPrefix(rest, "go:build"); ok {
			found = append(found, candidate{idx: i, goBuild: true, expression: strings.TrimSpace(expr)})
		} else if expr, ok := strings.CutPrefix(rest, "+build"); ok {
			found = append(found, candidate{idx: i, expression: strings.TrimSpace(expr)})
		}
	}
	if len(found) == 0 {
		return nil, nil
	}
	last := found[len(found)-1].idx
	blanked := false
	for i := last + 1; i < packageIdx; i++ {
		if strings.TrimSpace(strings.TrimSuffix(lines[i], "\r")) == "" {
			blanked = true
			break
		}
	}
	if !blanked {
		return nil, nil
	}
	for _, c := range found {
		if c.goBuild {
			goLines = append(goLines, c.expression)
		} else {
			plusLines = append(plusLines, c.expression)
		}
	}
	return goLines, plusLines
}

// buildConstraintsExclude reports whether a file carrying these header
// expressions is excluded from the current build: any unsatisfied line
// excludes (the toolchain ANDs lines), and a file with no lines is
// included. A line the toolchain cannot parse fails closed: a constraint
// addition that is not even well-formed can never be a correct edit.
func buildConstraintsExclude(goLines, plusLines []string, tags map[string]bool) bool {
	ok := func(tag string) bool { return buildTagSatisfied(tag, tags) }
	for _, expr := range goLines {
		// Parse takes the whole comment line, not the bare expression.
		parsed, err := constraint.Parse("//go:build " + expr)
		if err != nil {
			return true
		}
		if !parsed.Eval(ok) {
			return true
		}
	}
	for _, expr := range plusLines {
		if !plusBuildSatisfied(expr, ok) {
			return true
		}
	}
	return false
}

// plusBuildSatisfied evaluates one // +build line: space-separated
// alternatives of comma-separated conjunctions, each term optionally
// negated. A structurally empty alternative satisfies nothing.
func plusBuildSatisfied(expr string, ok func(string) bool) bool {
	alts := strings.Fields(expr)
	if len(alts) == 0 {
		return false
	}
	for _, alt := range alts {
		terms := strings.Split(alt, ",")
		satisfied := true
		for _, term := range terms {
			negated := false
			for strings.HasPrefix(term, "!") {
				negated = !negated
				term = term[1:]
			}
			if term == "" || ok(term) == negated {
				satisfied = false
				break
			}
		}
		if satisfied {
			return true
		}
	}
	return false
}

// buildTagSatisfied reports whether one constraint tag holds for the current
// build: the platform (GOOS, GOARCH, compiler), the toolchain version for
// go1.x tags, cgo unless explicitly disabled, and the effective build tags.
// ignore is never satisfied: by convention no build passes -tags=ignore.
func buildTagSatisfied(tag string, tags map[string]bool) bool {
	switch {
	case tag == "ignore":
		return false
	case tag == runtime.GOOS || tag == runtime.GOARCH || tag == runtime.Compiler:
		return true
	case tag == "cgo":
		return os.Getenv("CGO_ENABLED") != "0"
	case len(tag) > 2 && strings.HasPrefix(tag, "go") && tag[2] >= '0' && tag[2] <= '9':
		if version.IsValid(tag) && version.IsValid(runtime.Version()) {
			return version.Compare(runtime.Version(), tag) >= 0
		}
		return tags[tag]
	default:
		return tags[tag]
	}
}
