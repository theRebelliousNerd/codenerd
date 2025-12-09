package tactile

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// FileOpType defines the types of file operations.
type FileOpType string

const (
	FileOpRead   FileOpType = "read"
	FileOpWrite  FileOpType = "write"
	FileOpEdit   FileOpType = "edit"
	FileOpInsert FileOpType = "insert"
	FileOpDelete FileOpType = "delete"
	FileOpPatch  FileOpType = "patch"
)

// FileAuditEvent represents an audit event for file operations.
type FileAuditEvent struct {
	Type        FileOpType `json:"type"`
	Timestamp   time.Time  `json:"timestamp"`
	Path        string     `json:"path"`
	SessionID   string     `json:"session_id"`
	RequestID   string     `json:"request_id"`
	StartLine   int        `json:"start_line,omitempty"`
	EndLine     int        `json:"end_line,omitempty"`
	LinesAdded  int        `json:"lines_added,omitempty"`
	LinesRemove int        `json:"lines_removed,omitempty"`
	Success     bool       `json:"success"`
	Error       string     `json:"error,omitempty"`
	OldHash     string     `json:"old_hash,omitempty"`
	NewHash     string     `json:"new_hash,omitempty"`
}

// ToFacts converts a FileAuditEvent to Mangle facts.
func (e FileAuditEvent) ToFacts() []Fact {
	facts := make([]Fact, 0)
	timestamp := e.Timestamp.Unix()

	switch e.Type {
	case FileOpRead:
		facts = append(facts, Fact{
			Predicate: "file_read",
			Args:      []interface{}{e.Path, e.SessionID, timestamp},
		})

	case FileOpWrite:
		facts = append(facts, Fact{
			Predicate: "file_written",
			Args:      []interface{}{e.Path, e.NewHash, e.SessionID, timestamp},
		})
		facts = append(facts, Fact{
			Predicate: "modified",
			Args:      []interface{}{e.Path},
		})

	case FileOpEdit:
		facts = append(facts, Fact{
			Predicate: "lines_edited",
			Args:      []interface{}{e.Path, int64(e.StartLine), int64(e.EndLine), e.SessionID},
		})
		facts = append(facts, Fact{
			Predicate: "modified",
			Args:      []interface{}{e.Path},
		})

	case FileOpInsert:
		facts = append(facts, Fact{
			Predicate: "lines_inserted",
			Args:      []interface{}{e.Path, int64(e.StartLine), int64(e.LinesAdded), e.SessionID},
		})
		facts = append(facts, Fact{
			Predicate: "modified",
			Args:      []interface{}{e.Path},
		})

	case FileOpDelete:
		facts = append(facts, Fact{
			Predicate: "lines_deleted",
			Args:      []interface{}{e.Path, int64(e.StartLine), int64(e.EndLine), e.SessionID},
		})
		facts = append(facts, Fact{
			Predicate: "modified",
			Args:      []interface{}{e.Path},
		})
	}

	return facts
}

// FileResult represents the result of a file operation.
type FileResult struct {
	Success       bool     `json:"success"`
	Path          string   `json:"path"`
	LinesAffected int      `json:"lines_affected"`
	OldContent    []string `json:"old_content,omitempty"` // For undo support
	NewContent    []string `json:"new_content,omitempty"`
	OldHash       string   `json:"old_hash,omitempty"`
	NewHash       string   `json:"new_hash,omitempty"`
	LineCount     int      `json:"line_count"`
	Facts         []Fact   `json:"facts,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// FileEditor handles all file read/write/edit operations with audit logging.
type FileEditor struct {
	mu sync.RWMutex

	// Audit callback for logging operations
	auditCallback func(FileAuditEvent)

	// Fact callback for injecting facts to kernel
	factCallback func(Fact)

	// Session tracking
	sessionID string

	// Working directory for relative paths
	workingDir string
}

// NewFileEditor creates a new FileEditor.
func NewFileEditor() *FileEditor {
	return &FileEditor{
		workingDir: ".",
	}
}

// NewFileEditorWithSession creates a new FileEditor with session context.
func NewFileEditorWithSession(sessionID string) *FileEditor {
	return &FileEditor{
		sessionID:  sessionID,
		workingDir: ".",
	}
}

// SetAuditCallback sets the callback for file audit events.
func (e *FileEditor) SetAuditCallback(callback func(FileAuditEvent)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.auditCallback = callback
}

// SetFactCallback sets the callback for fact injection.
func (e *FileEditor) SetFactCallback(callback func(Fact)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.factCallback = callback
}

// SetWorkingDir sets the working directory for relative paths.
func (e *FileEditor) SetWorkingDir(dir string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workingDir = dir
}

// emitAudit emits an audit event and associated facts.
func (e *FileEditor) emitAudit(event FileAuditEvent) {
	e.mu.RLock()
	auditCb := e.auditCallback
	factCb := e.factCallback
	e.mu.RUnlock()

	if auditCb != nil {
		auditCb(event)
	}

	if factCb != nil {
		for _, fact := range event.ToFacts() {
			factCb(fact)
		}
	}
}

// resolvePath resolves a path relative to working directory.
func (e *FileEditor) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	e.mu.RLock()
	workDir := e.workingDir
	e.mu.RUnlock()
	return filepath.Join(workDir, path)
}

// computeHash computes SHA256 hash of content.
func computeHash(content []string) string {
	h := sha256.New()
	for _, line := range content {
		h.Write([]byte(line))
		h.Write([]byte("\n"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ReadFile reads an entire file and returns its lines.
func (e *FileEditor) ReadFile(path string) ([]string, error) {
	timer := logging.StartTimer(logging.CategoryTactile, "File read")
	defer timer.Stop()

	absPath := e.resolvePath(path)
	logging.Tactile("Reading file: %s", absPath)

	file, err := os.Open(absPath)
	if err != nil {
		logging.TactileError("File read failed: %s - %v", path, err)
		e.emitAudit(FileAuditEvent{
			Type:      FileOpRead,
			Timestamp: time.Now(),
			Path:      path,
			SessionID: e.sessionID,
			Success:   false,
			Error:     err.Error(),
		})
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		logging.TactileError("File scan error: %s - %v", path, err)
		e.emitAudit(FileAuditEvent{
			Type:      FileOpRead,
			Timestamp: time.Now(),
			Path:      path,
			SessionID: e.sessionID,
			Success:   false,
			Error:     err.Error(),
		})
		return nil, err
	}

	logging.TactileDebug("File read completed: %s (%d lines)", path, len(lines))
	e.emitAudit(FileAuditEvent{
		Type:      FileOpRead,
		Timestamp: time.Now(),
		Path:      path,
		SessionID: e.sessionID,
		Success:   true,
	})

	return lines, nil
}

// ReadLines reads specific lines from a file (1-indexed, inclusive).
func (e *FileEditor) ReadLines(path string, startLine, endLine int) ([]string, error) {
	logging.TactileDebug("Reading lines %d-%d from: %s", startLine, endLine, path)
	lines, err := e.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Validate line numbers (1-indexed)
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		logging.TactileDebug("No lines in range %d-%d for file %s", startLine, endLine, path)
		return []string{}, nil
	}

	// Convert to 0-indexed
	result := lines[startLine-1 : endLine]
	logging.TactileDebug("Read %d lines from %s (range %d-%d)", len(result), path, startLine, endLine)
	return result, nil
}

// WriteFile writes content to a file, creating directories if needed.
func (e *FileEditor) WriteFile(path string, lines []string) (*FileResult, error) {
	timer := logging.StartTimer(logging.CategoryTactile, "File write")
	defer timer.Stop()

	absPath := e.resolvePath(path)
	logging.Tactile("Writing file: %s (%d lines)", absPath, len(lines))

	// Read old content for undo and hash comparison
	var oldContent []string
	var oldHash string
	if existing, err := e.ReadFile(path); err == nil {
		oldContent = existing
		oldHash = computeHash(existing)
		logging.TactileDebug("Existing file found: %d lines, hash=%s", len(existing), oldHash[:16])
	} else {
		logging.TactileDebug("Creating new file: %s", path)
	}

	// Ensure directory exists
	dir := filepath.Dir(absPath)
	logging.TactileDebug("Ensuring directory exists: %s", dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.TactileError("Failed to create directory: %s - %v", dir, err)
		result := &FileResult{
			Success: false,
			Path:    path,
			Error:   err.Error(),
		}
		e.emitAudit(FileAuditEvent{
			Type:      FileOpWrite,
			Timestamp: time.Now(),
			Path:      path,
			SessionID: e.sessionID,
			Success:   false,
			Error:     err.Error(),
		})
		return result, err
	}

	// Write content
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n" // Ensure trailing newline
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		logging.TactileError("File write failed: %s - %v", path, err)
		result := &FileResult{
			Success: false,
			Path:    path,
			Error:   err.Error(),
		}
		e.emitAudit(FileAuditEvent{
			Type:      FileOpWrite,
			Timestamp: time.Now(),
			Path:      path,
			SessionID: e.sessionID,
			Success:   false,
			Error:     err.Error(),
		})
		return result, err
	}

	newHash := computeHash(lines)
	logging.TactileDebug("File written successfully: %s (hash=%s)", path, newHash[:16])

	result := &FileResult{
		Success:       true,
		Path:          path,
		LinesAffected: len(lines),
		OldContent:    oldContent,
		NewContent:    lines,
		OldHash:       oldHash,
		NewHash:       newHash,
		LineCount:     len(lines),
	}

	event := FileAuditEvent{
		Type:      FileOpWrite,
		Timestamp: time.Now(),
		Path:      path,
		SessionID: e.sessionID,
		Success:   true,
		OldHash:   oldHash,
		NewHash:   newHash,
	}
	e.emitAudit(event)
	result.Facts = event.ToFacts()

	logging.Tactile("File write completed: %s (%d lines)", path, len(lines))
	return result, nil
}

// EditLines replaces lines in a file (1-indexed, inclusive).
func (e *FileEditor) EditLines(path string, startLine, endLine int, newLines []string) (*FileResult, error) {
	timer := logging.StartTimer(logging.CategoryTactile, "File edit")
	defer timer.Stop()

	logging.Tactile("Editing file: %s lines %d-%d (%d new lines)", path, startLine, endLine, len(newLines))
	lines, err := e.ReadFile(path)
	if err != nil {
		logging.TactileError("Failed to read file for edit: %s - %v", path, err)
		return nil, err
	}

	oldHash := computeHash(lines)

	// Validate line numbers (1-indexed)
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Store old content for undo
	var oldContent []string
	if startLine <= len(lines) && endLine >= startLine {
		oldContent = make([]string, endLine-startLine+1)
		copy(oldContent, lines[startLine-1:endLine])
	}

	// Build new content
	var result []string
	result = append(result, lines[:startLine-1]...)
	result = append(result, newLines...)
	if endLine < len(lines) {
		result = append(result, lines[endLine:]...)
	}

	// Write back
	writeResult, err := e.WriteFile(path, result)
	if err != nil {
		return nil, err
	}

	linesAffected := len(oldContent)
	if len(newLines) > linesAffected {
		linesAffected = len(newLines)
	}

	editResult := &FileResult{
		Success:       true,
		Path:          path,
		LinesAffected: linesAffected,
		OldContent:    oldContent,
		NewContent:    newLines,
		OldHash:       oldHash,
		NewHash:       writeResult.NewHash,
		LineCount:     len(result),
	}

	event := FileAuditEvent{
		Type:        FileOpEdit,
		Timestamp:   time.Now(),
		Path:        path,
		SessionID:   e.sessionID,
		StartLine:   startLine,
		EndLine:     endLine,
		LinesAdded:  len(newLines),
		LinesRemove: len(oldContent),
		Success:     true,
		OldHash:     oldHash,
		NewHash:     writeResult.NewHash,
	}
	e.emitAudit(event)
	editResult.Facts = event.ToFacts()

	logging.TactileDebug("Edit completed: %s (replaced %d lines with %d lines)", path, len(oldContent), len(newLines))
	return editResult, nil
}

// InsertLines inserts lines after the specified line (1-indexed, 0 = beginning).
func (e *FileEditor) InsertLines(path string, afterLine int, newLines []string) (*FileResult, error) {
	timer := logging.StartTimer(logging.CategoryTactile, "File insert")
	defer timer.Stop()

	logging.Tactile("Inserting into file: %s after line %d (%d lines)", path, afterLine, len(newLines))
	lines, err := e.ReadFile(path)
	if err != nil {
		// File doesn't exist, create with new content
		if os.IsNotExist(err) {
			logging.TactileDebug("File does not exist, creating with new content: %s", path)
			return e.WriteFile(path, newLines)
		}
		logging.TactileError("Failed to read file for insert: %s - %v", path, err)
		return nil, err
	}

	oldHash := computeHash(lines)

	// Validate line number
	if afterLine < 0 {
		afterLine = 0
	}
	if afterLine > len(lines) {
		afterLine = len(lines)
	}

	// Build new content
	var result []string
	result = append(result, lines[:afterLine]...)
	result = append(result, newLines...)
	result = append(result, lines[afterLine:]...)

	// Write back
	writeResult, err := e.WriteFile(path, result)
	if err != nil {
		return nil, err
	}

	insertResult := &FileResult{
		Success:       true,
		Path:          path,
		LinesAffected: len(newLines),
		OldContent:    nil, // No content replaced
		NewContent:    newLines,
		OldHash:       oldHash,
		NewHash:       writeResult.NewHash,
		LineCount:     len(result),
	}

	event := FileAuditEvent{
		Type:       FileOpInsert,
		Timestamp:  time.Now(),
		Path:       path,
		SessionID:  e.sessionID,
		StartLine:  afterLine + 1,
		LinesAdded: len(newLines),
		Success:    true,
		OldHash:    oldHash,
		NewHash:    writeResult.NewHash,
	}
	e.emitAudit(event)
	insertResult.Facts = event.ToFacts()

	logging.TactileDebug("Insert completed: %s (added %d lines after line %d)", path, len(newLines), afterLine)
	return insertResult, nil
}

// DeleteLines removes lines from a file (1-indexed, inclusive).
func (e *FileEditor) DeleteLines(path string, startLine, endLine int) (*FileResult, error) {
	timer := logging.StartTimer(logging.CategoryTactile, "File delete")
	defer timer.Stop()

	logging.Tactile("Deleting lines: %s lines %d-%d", path, startLine, endLine)
	lines, err := e.ReadFile(path)
	if err != nil {
		logging.TactileError("Failed to read file for delete: %s - %v", path, err)
		return nil, err
	}

	oldHash := computeHash(lines)

	// Validate line numbers (1-indexed)
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		return &FileResult{
			Success:       true,
			Path:          path,
			LinesAffected: 0,
			LineCount:     len(lines),
		}, nil
	}

	// Store old content for undo
	oldContent := make([]string, endLine-startLine+1)
	copy(oldContent, lines[startLine-1:endLine])

	// Build new content
	var result []string
	result = append(result, lines[:startLine-1]...)
	if endLine < len(lines) {
		result = append(result, lines[endLine:]...)
	}

	// Write back
	writeResult, err := e.WriteFile(path, result)
	if err != nil {
		return nil, err
	}

	deleteResult := &FileResult{
		Success:       true,
		Path:          path,
		LinesAffected: len(oldContent),
		OldContent:    oldContent,
		NewContent:    nil,
		OldHash:       oldHash,
		NewHash:       writeResult.NewHash,
		LineCount:     len(result),
	}

	event := FileAuditEvent{
		Type:        FileOpDelete,
		Timestamp:   time.Now(),
		Path:        path,
		SessionID:   e.sessionID,
		StartLine:   startLine,
		EndLine:     endLine,
		LinesRemove: len(oldContent),
		Success:     true,
		OldHash:     oldHash,
		NewHash:     writeResult.NewHash,
	}
	e.emitAudit(event)
	deleteResult.Facts = event.ToFacts()

	logging.TactileDebug("Delete completed: %s (removed %d lines)", path, len(oldContent))
	return deleteResult, nil
}

// ReplaceElement replaces content between start and end lines (1-indexed, inclusive).
// This is a convenience wrapper for EditLines that takes a string instead of []string.
func (e *FileEditor) ReplaceElement(path string, startLine, endLine int, newContent string) (*FileResult, error) {
	logging.TactileDebug("ReplaceElement: %s lines %d-%d (%d chars)", path, startLine, endLine, len(newContent))
	// Split content into lines, preserving empty lines
	var newLines []string
	if newContent != "" {
		newLines = strings.Split(strings.TrimSuffix(newContent, "\n"), "\n")
	}
	return e.EditLines(path, startLine, endLine, newLines)
}

// GetFileInfo returns metadata about a file.
func (e *FileEditor) GetFileInfo(path string) (*FileInfo, error) {
	logging.TactileDebug("Getting file info: %s", path)
	absPath := e.resolvePath(path)

	stat, err := os.Stat(absPath)
	if err != nil {
		logging.TactileWarn("Failed to stat file: %s - %v", path, err)
		return nil, err
	}

	lines, err := e.ReadFile(path)
	if err != nil {
		return nil, err
	}

	info := &FileInfo{
		Path:      path,
		AbsPath:   absPath,
		Size:      stat.Size(),
		ModTime:   stat.ModTime(),
		LineCount: len(lines),
		Hash:      computeHash(lines),
	}
	logging.TactileDebug("File info: %s (size=%d, lines=%d)", path, info.Size, info.LineCount)
	return info, nil
}

// FileInfo contains metadata about a file.
type FileInfo struct {
	Path      string    `json:"path"`
	AbsPath   string    `json:"abs_path"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	LineCount int       `json:"line_count"`
	Hash      string    `json:"hash"`
}

// FileExists checks if a file exists.
func (e *FileEditor) FileExists(path string) bool {
	absPath := e.resolvePath(path)
	_, err := os.Stat(absPath)
	return err == nil
}

// CreateDirectory creates a directory and all parent directories.
func (e *FileEditor) CreateDirectory(path string) error {
	absPath := e.resolvePath(path)
	logging.Tactile("Creating directory: %s", absPath)
	err := os.MkdirAll(absPath, 0755)
	if err != nil {
		logging.TactileError("Failed to create directory: %s - %v", absPath, err)
	} else {
		logging.TactileDebug("Directory created successfully: %s", absPath)
	}
	return err
}
