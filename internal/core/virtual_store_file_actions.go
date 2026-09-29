package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/observation/precondition"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
	"codenerd/internal/workspace"
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

	// Membership, not a substring of ".git": that check also dropped .github.
	mem, memErr := workspace.For(v.workingDir)
	if memErr != nil {
		return ActionResult{Success: false, Error: memErr.Error()}, nil
	}
	if memErr = mem.Refresh(); memErr != nil {
		return ActionResult{Success: false, Error: memErr.Error()}, nil
	}
	err := filepath.Walk(v.workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		member, admErr := mem.Admit(path, info.IsDir())
		if admErr != nil {
			return admErr
		}
		if !member || info.IsDir() {
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

// rejectBuildExclusion is the VirtualStore's binding of
// tools.RejectAddedBuildExclusion: the workspace whose gate tags the
// current build evaluates constraints against. The judgement lives in
// tools so file_ops and the syntax guard call the same function; core
// already imports tools, and those packages cannot import core.
func (v *VirtualStore) rejectBuildExclusion(path string, before, after []byte) error {
	root := ""
	if v != nil {
		root = v.workspaceRoot
		if root == "" {
			root = v.workingDir
		}
	}
	return tools.RejectAddedBuildExclusion(root, path, before, after)
}
