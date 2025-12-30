package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

// =============================================================================
// CODE DOM HANDLERS
// =============================================================================
// These handlers enable structural code manipulation through the FileScope
// and CodeDOM abstractions. They work at the level of functions, methods,
// and types rather than raw text.

// handleOpenFile opens a file and loads its 1-hop dependency scope.
func (v *VirtualStore) handleOpenFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	path := v.resolvePath(req.Target)
	if err := scope.Open(path); err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "scope_open_failed", Args: []interface{}{path, err.Error()}},
			},
		}, nil
	}

	// Replace previous Code DOM state with the new scope.
	v.clearCodeDOMFacts()

	facts := scope.ScopeFacts()

	inScopeFiles := scope.GetInScopeFiles()
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Opened %s with %d files in scope", path, len(inScopeFiles)),
		Metadata: map[string]interface{}{
			"active_file":    path,
			"in_scope_count": len(inScopeFiles),
			"in_scope":       inScopeFiles,
		},
		FactsToAdd: facts,
	}, nil
}

// handleGetElements returns all elements in the current scope.
func (v *VirtualStore) handleGetElements(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	// Get elements, optionally filtered by file
	var elements []CodeElement
	if req.Target != "" {
		path := v.resolvePath(req.Target)
		elements = scope.GetCoreElementsByFile(path)
	} else {
		// Return all elements in scope - need to iterate files
		for _, file := range scope.GetInScopeFiles() {
			elements = append(elements, scope.GetCoreElementsByFile(file)...)
		}
	}

	// Filter by type if specified
	if elemType, ok := req.Payload["type"].(string); ok && elemType != "" {
		var filtered []CodeElement
		for _, e := range elements {
			if e.Type == elemType {
				filtered = append(filtered, e)
			}
		}
		elements = filtered
	}

	output, _ := json.Marshal(elements)
	return ActionResult{
		Success: true,
		Output:  string(output),
		Metadata: map[string]interface{}{
			"count": len(elements),
		},
	}, nil
}

// handleGetElement returns a single element by ref.
func (v *VirtualStore) handleGetElement(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	ref := req.Target
	elem := scope.GetCoreElement(ref)
	if elem == nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("element not found: %s", ref),
		}, nil
	}

	// Include body if requested
	includeBody, _ := req.Payload["include_body"].(bool)
	if includeBody && elem.Body == "" {
		elem.Body = scope.GetElementBody(ref)
	}

	output, _ := json.Marshal(elem)
	return ActionResult{
		Success: true,
		Output:  string(output),
		Metadata: map[string]interface{}{
			"ref":        elem.Ref,
			"type":       elem.Type,
			"file":       elem.File,
			"start_line": elem.StartLine,
			"end_line":   elem.EndLine,
		},
	}, nil
}

// handleEditElement replaces an element's body by ref.
func (v *VirtualStore) handleEditElement(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	editor := v.fileEditor
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}
	if editor == nil {
		return ActionResult{Success: false, Error: "file editor not configured"}, nil
	}

	ref := req.Target
	newContent, ok := req.Payload["content"].(string)
	if !ok {
		return ActionResult{Success: false, Error: "edit_element requires 'content' in payload"}, nil
	}

	elem := scope.GetCoreElement(ref)
	if elem == nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("element not found: %s", ref),
		}, nil
	}

	// Verify file hasn't been modified externally before editing
	unchanged, hashErr := scope.VerifyFileHash(elem.File)
	if hashErr != nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to verify file hash: %v", hashErr),
			FactsToAdd: []Fact{
				{Predicate: "element_edit_blocked", Args: []interface{}{ref, "hash_verification_failed"}},
			},
		}, nil
	}
	if !unchanged {
		// File was modified externally - refresh scope first
		if err := scope.RefreshWithRetry(3); err != nil {
			return ActionResult{
				Success: false,
				Error:   "file was modified externally and refresh failed",
				FactsToAdd: []Fact{
					{Predicate: "element_edit_blocked", Args: []interface{}{ref, "concurrent_modification"}},
					{Predicate: "file_modified_externally", Args: []interface{}{elem.File}},
				},
			}, nil
		}
		// Re-fetch element after refresh (line numbers may have changed)
		elem = scope.GetCoreElement(ref)
		if elem == nil {
			return ActionResult{
				Success: false,
				Error:   fmt.Sprintf("element %s no longer exists after refresh", ref),
				FactsToAdd: []Fact{
					{Predicate: "element_stale", Args: []interface{}{ref, "not_found_after_refresh"}},
				},
			}, nil
		}
	}

	// Replace the element
	result, err := editor.ReplaceElement(elem.File, elem.StartLine, elem.EndLine, newContent)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	factsToAdd := make([]Fact, 0, len(result.Facts)+8)
	factsToAdd = append(factsToAdd, result.Facts...)
	factsToAdd = append(factsToAdd,
		Fact{Predicate: "element_modified", Args: []interface{}{ref, req.SessionID, time.Now().Unix()}},
		Fact{Predicate: "modified", Args: []interface{}{elem.File}},
	)

	// Refresh scope to update line numbers with retry
	if err := scope.RefreshWithRetry(3); err != nil {
		factsToAdd = append(factsToAdd, Fact{
			Predicate: "scope_refresh_failed",
			Args:      []interface{}{elem.File, err.Error()},
		})
	} else {
		// Replace previous Code DOM state with the refreshed scope.
		v.clearCodeDOMFacts()
		factsToAdd = append(factsToAdd, scope.ScopeFacts()...)
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Replaced element %s (%d lines affected)", ref, result.LinesAffected),
		Metadata: map[string]interface{}{
			"ref":            ref,
			"lines_affected": result.LinesAffected,
			"new_line_count": result.LineCount,
		},
		FactsToAdd: factsToAdd,
	}, nil
}

// handleRefreshScope re-parses all in-scope files.
func (v *VirtualStore) handleRefreshScope(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	if err := scope.Refresh(); err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Replace previous Code DOM state with the refreshed scope.
	v.clearCodeDOMFacts()

	facts := scope.ScopeFacts()

	return ActionResult{
		Success:    true,
		Output:     "Scope refreshed",
		FactsToAdd: facts,
	}, nil
}

// handleCloseScope closes the current scope.
func (v *VirtualStore) handleCloseScope(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	scope.Close()
	v.clearCodeDOMFacts()

	return ActionResult{
		Success: true,
		Output:  "Scope closed",
		FactsToAdd: []Fact{
			{Predicate: "scope_closed", Args: []interface{}{}},
		},
	}, nil
}

// handleEditLines performs line-based file editing.
func (v *VirtualStore) handleEditLines(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	editor := v.fileEditor
	scope := v.codeScope
	v.mu.RUnlock()

	if editor == nil {
		return ActionResult{Success: false, Error: "file editor not configured"}, nil
	}

	path := v.resolvePath(req.Target)

	startLine, _ := req.Payload["start_line"].(float64)
	endLine, _ := req.Payload["end_line"].(float64)
	newContent, _ := req.Payload["content"].(string)

	if startLine == 0 || endLine == 0 {
		return ActionResult{Success: false, Error: "edit_lines requires 'start_line' and 'end_line' in payload"}, nil
	}

	// Split content into lines
	var newLines []string
	if newContent != "" {
		newLines = strings.Split(strings.TrimSuffix(newContent, "\n"), "\n")
	}

	result, err := editor.EditLines(path, int(startLine), int(endLine), newLines)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	factsToAdd := make([]Fact, 0, len(result.Facts)+8)
	factsToAdd = append(factsToAdd, result.Facts...)

	// Refresh scope if active with retry
	if scope != nil && scope.IsInScope(path) {
		if err := scope.RefreshWithRetry(3); err != nil {
			factsToAdd = append(factsToAdd, Fact{
				Predicate: "scope_refresh_failed",
				Args:      []interface{}{path, err.Error()},
			})
		} else {
			// Replace previous Code DOM state with the refreshed scope.
			v.clearCodeDOMFacts()
			factsToAdd = append(factsToAdd, scope.ScopeFacts()...)
		}
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Edited lines %d-%d in %s", int(startLine), int(endLine), path),
		Metadata: map[string]interface{}{
			"path":           path,
			"start_line":     int(startLine),
			"end_line":       int(endLine),
			"lines_affected": result.LinesAffected,
		},
		FactsToAdd: factsToAdd,
	}, nil
}

// handleInsertLines inserts lines at a position.
func (v *VirtualStore) handleInsertLines(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	editor := v.fileEditor
	scope := v.codeScope
	v.mu.RUnlock()

	if editor == nil {
		return ActionResult{Success: false, Error: "file editor not configured"}, nil
	}

	path := v.resolvePath(req.Target)

	afterLine, _ := req.Payload["after_line"].(float64)
	content, _ := req.Payload["content"].(string)

	if content == "" {
		return ActionResult{Success: false, Error: "insert_lines requires 'content' in payload"}, nil
	}

	newLines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")

	result, err := editor.InsertLines(path, int(afterLine), newLines)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	factsToAdd := make([]Fact, 0, len(result.Facts)+8)
	factsToAdd = append(factsToAdd, result.Facts...)

	// Refresh scope if active with retry
	if scope != nil && scope.IsInScope(path) {
		if err := scope.RefreshWithRetry(3); err != nil {
			factsToAdd = append(factsToAdd, Fact{
				Predicate: "scope_refresh_failed",
				Args:      []interface{}{path, err.Error()},
			})
		} else {
			// Replace previous Code DOM state with the refreshed scope.
			v.clearCodeDOMFacts()
			factsToAdd = append(factsToAdd, scope.ScopeFacts()...)
		}
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Inserted %d lines after line %d in %s", result.LinesAffected, int(afterLine), path),
		Metadata: map[string]interface{}{
			"path":        path,
			"after_line":  int(afterLine),
			"lines_added": result.LinesAffected,
		},
		FactsToAdd: factsToAdd,
	}, nil
}

// handleDeleteLines removes lines from a file.
func (v *VirtualStore) handleDeleteLines(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	editor := v.fileEditor
	scope := v.codeScope
	v.mu.RUnlock()

	if editor == nil {
		return ActionResult{Success: false, Error: "file editor not configured"}, nil
	}

	path := v.resolvePath(req.Target)

	startLine, _ := req.Payload["start_line"].(float64)
	endLine, _ := req.Payload["end_line"].(float64)

	if startLine == 0 || endLine == 0 {
		return ActionResult{Success: false, Error: "delete_lines requires 'start_line' and 'end_line' in payload"}, nil
	}

	result, err := editor.DeleteLines(path, int(startLine), int(endLine))
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	factsToAdd := make([]Fact, 0, len(result.Facts)+8)
	factsToAdd = append(factsToAdd, result.Facts...)

	// Refresh scope if active with retry
	if scope != nil && scope.IsInScope(path) {
		if err := scope.RefreshWithRetry(3); err != nil {
			factsToAdd = append(factsToAdd, Fact{
				Predicate: "scope_refresh_failed",
				Args:      []interface{}{path, err.Error()},
			})
		} else {
			// Replace previous Code DOM state with the refreshed scope.
			v.clearCodeDOMFacts()
			factsToAdd = append(factsToAdd, scope.ScopeFacts()...)
		}
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Deleted lines %d-%d from %s", int(startLine), int(endLine), path),
		Metadata: map[string]interface{}{
			"path":          path,
			"start_line":    int(startLine),
			"end_line":      int(endLine),
			"lines_deleted": result.LinesAffected,
		},
		FactsToAdd: factsToAdd,
	}, nil
}

// =============================================================================
// AUTOPOIESIS HANDLERS - GENERATED TOOL EXECUTION
// =============================================================================

// handleExecTool executes a generated tool from the Ouroboros registry.
func (v *VirtualStore) handleExecTool(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleExecTool")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	toolExec := v.toolExecutor
	registry := v.toolRegistry
	v.mu.RUnlock()

	if toolExec == nil {
		logging.Get(logging.CategoryVirtualStore).Error("Tool executor not configured")
		return ActionResult{
			Success: false,
			Error:   "tool executor not configured",
			FactsToAdd: []Fact{
				{Predicate: "tool_exec_failed", Args: []interface{}{req.Target, "no_executor"}},
			},
		}, nil
	}

	toolName := req.Target
	input, _ := req.Payload["input"].(string)

	logging.VirtualStore("Executing tool: %s (input_len=%d)", toolName, len(input))

	// Check if tool exists in registry first
	var registeredTool *Tool
	if registry != nil {
		registeredTool, _ = registry.GetTool(toolName)
	}

	// Check if tool exists in executor
	toolInfo, exists := toolExec.GetTool(toolName)
	if !exists {
		logging.Get(logging.CategoryVirtualStore).Warn("Tool not found: %s", toolName)
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("tool not found: %s", toolName),
			FactsToAdd: []Fact{
				{Predicate: "tool_not_found", Args: []interface{}{toolName}},
			},
		}, nil
	}

	// Execute the tool
	output, err := toolExec.ExecuteTool(ctx, toolName, input)

	// Update execution count in registry
	if registeredTool != nil && registry != nil {
		registeredTool.ExecuteCount++
	}

	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Tool execution failed: %s - %v", toolName, err)
		return ActionResult{
			Success: false,
			Output:  output, // Might have partial output
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "tool_exec_failed", Args: []interface{}{toolName, err.Error()}},
			},
		}, nil
	}

	metadata := map[string]interface{}{
		"tool_name":     toolName,
		"tool_hash":     toolInfo.Hash,
		"execute_count": toolInfo.ExecuteCount + 1,
	}

	if registeredTool != nil {
		metadata["shard_affinity"] = registeredTool.ShardAffinity
		metadata["command"] = registeredTool.Command
	}

	logging.VirtualStore("Tool %s executed successfully: output_len=%d", toolName, len(output))
	return ActionResult{
		Success:  true,
		Output:   output,
		Metadata: metadata,
		FactsToAdd: []Fact{
			{Predicate: "tool_executed", Args: []interface{}{toolName, output}},
			{Predicate: "tool_exec_success", Args: []interface{}{toolName}},
		},
	}, nil
}

// =============================================================================
// FILE EDITOR ADAPTER
// =============================================================================

// TactileFileEditorAdapter wraps tactile.FileEditor to implement core.FileEditor.
type TactileFileEditorAdapter struct {
	editor *tactile.FileEditor
}

// NewTactileFileEditorAdapter creates a new adapter.
func NewTactileFileEditorAdapter(editor *tactile.FileEditor) *TactileFileEditorAdapter {
	return &TactileFileEditorAdapter{editor: editor}
}

func (a *TactileFileEditorAdapter) ReadFile(path string) ([]string, error) {
	return a.editor.ReadFile(path)
}

func (a *TactileFileEditorAdapter) ReadLines(path string, startLine, endLine int) ([]string, error) {
	return a.editor.ReadLines(path, startLine, endLine)
}

func (a *TactileFileEditorAdapter) WriteFile(path string, lines []string) (*FileEditResult, error) {
	result, err := a.editor.WriteFile(path, lines)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) EditLines(path string, startLine, endLine int, newLines []string) (*FileEditResult, error) {
	result, err := a.editor.EditLines(path, startLine, endLine, newLines)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) InsertLines(path string, afterLine int, newLines []string) (*FileEditResult, error) {
	result, err := a.editor.InsertLines(path, afterLine, newLines)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) DeleteLines(path string, startLine, endLine int) (*FileEditResult, error) {
	result, err := a.editor.DeleteLines(path, startLine, endLine)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) ReplaceElement(path string, startLine, endLine int, newContent string) (*FileEditResult, error) {
	result, err := a.editor.ReplaceElement(path, startLine, endLine, newContent)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	if a.editor == nil {
		return "", "", fmt.Errorf("file editor not configured")
	}
	return a.editor.Exec(ctx, cmd, env)
}

func (a *TactileFileEditorAdapter) convertResult(r *tactile.FileResult) *FileEditResult {
	if r == nil {
		return nil
	}
	// Convert tactile.Fact to core.Fact
	facts := make([]Fact, len(r.Facts))
	for i, f := range r.Facts {
		facts[i] = Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return &FileEditResult{
		Success:       r.Success,
		Path:          r.Path,
		LinesAffected: r.LinesAffected,
		OldContent:    r.OldContent,
		NewContent:    r.NewContent,
		OldHash:       r.OldHash,
		NewHash:       r.NewHash,
		LineCount:     r.LineCount,
		Facts:         facts,
		Error:         r.Error,
	}
}
