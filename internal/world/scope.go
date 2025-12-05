package world

import (
	"bytes"
	"codenerd/internal/core"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// FileScope manages the 1-hop dependency scope for Code DOM.
// When a file is "opened", it loads the file plus its direct imports
// and files that directly import it.
type FileScope struct {
	mu sync.RWMutex

	// ActiveFile is the primary file being worked on
	ActiveFile string

	// InScope contains all files in the current scope (active + 1-hop)
	InScope []string

	// Elements contains all code elements in scope
	Elements []CodeElement

	// OutboundDeps maps file -> import paths (files this file imports)
	OutboundDeps map[string][]string

	// InboundDeps maps file -> files that import it
	InboundDeps map[string][]string

	// FileHashes maps file -> content hash for change detection
	FileHashes map[string]string

	// ProjectRoot is the root directory for resolving imports
	ProjectRoot string

	// ModulePath is the Go module path (from go.mod)
	ModulePath string

	// Parser for extracting code elements
	parser *CodeElementParser

	// Fact callback for injecting facts to kernel
	factCallback func(core.Fact)
}

// NewFileScope creates a new FileScope.
func NewFileScope(projectRoot string) *FileScope {
	return &FileScope{
		InScope:      make([]string, 0),
		Elements:     make([]CodeElement, 0),
		OutboundDeps: make(map[string][]string),
		InboundDeps:  make(map[string][]string),
		FileHashes:   make(map[string]string),
		ProjectRoot:  projectRoot,
		parser:       NewCodeElementParser(),
	}
}

// SetFactCallback sets the callback for fact injection to the kernel.
func (s *FileScope) SetFactCallback(callback func(core.Fact)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.factCallback = callback
}

// Open opens a file and loads its 1-hop dependency scope.
func (s *FileScope) Open(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Detect module path from go.mod
	if err := s.detectModulePath(); err != nil {
		// Non-fatal: just won't resolve module imports
	}

	s.ActiveFile = absPath
	s.InScope = []string{absPath}
	s.Elements = nil

	// 1. Parse active file and find its imports
	outbound, err := s.findOutboundDeps(absPath)
	if err != nil {
		return err
	}
	s.OutboundDeps[absPath] = outbound

	// 2. Find files that import the active file
	inbound, err := s.findInboundDeps(absPath)
	if err != nil {
		// Non-fatal: just won't have inbound deps
		inbound = []string{}
	}
	s.InboundDeps[absPath] = inbound

	// 3. Add 1-hop files to scope
	seen := make(map[string]bool)
	seen[absPath] = true

	for _, dep := range outbound {
		resolved := s.resolveImportPath(dep)
		if resolved != "" && !seen[resolved] {
			s.InScope = append(s.InScope, resolved)
			seen[resolved] = true
		}
	}

	for _, dep := range inbound {
		if !seen[dep] {
			s.InScope = append(s.InScope, dep)
			seen[dep] = true
		}
	}

	// 4. Parse all files in scope and extract elements
	for _, file := range s.InScope {
		if err := s.loadFile(file); err != nil {
			// Log but continue with other files
			continue
		}
	}

	// 5. Emit facts
	s.emitScopeFacts()

	return nil
}

// Refresh re-parses all in-scope files and updates element refs.
// Call this after an edit to update line numbers.
func (s *FileScope) Refresh() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Elements = nil

	for _, file := range s.InScope {
		if err := s.loadFile(file); err != nil {
			continue
		}
	}

	s.emitScopeFacts()
	return nil
}

// GetElement returns an element by ref.
func (s *FileScope) GetElement(ref string) *CodeElement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return GetElement(s.Elements, ref)
}

// GetElementBody returns the body text of an element by ref.
func (s *FileScope) GetElementBody(ref string) string {
	elem := s.GetElement(ref)
	if elem == nil {
		return ""
	}
	return elem.Body
}

// QueryElements returns elements matching a filter function.
func (s *FileScope) QueryElements(filter func(CodeElement) bool) []CodeElement {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []CodeElement
	for _, e := range s.Elements {
		if filter(e) {
			result = append(result, e)
		}
	}
	return result
}

// GetElementsByFile returns all elements in a specific file.
func (s *FileScope) GetElementsByFile(path string) []CodeElement {
	absPath, _ := filepath.Abs(path)
	return s.QueryElements(func(e CodeElement) bool {
		return e.File == absPath
	})
}

// IsInScope checks if a file is in the current scope.
func (s *FileScope) IsInScope(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	absPath, _ := filepath.Abs(path)
	for _, f := range s.InScope {
		if f == absPath {
			return true
		}
	}
	return false
}

// Close clears the current scope.
func (s *FileScope) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ActiveFile = ""
	s.InScope = nil
	s.Elements = nil
	s.OutboundDeps = make(map[string][]string)
	s.InboundDeps = make(map[string][]string)
	s.FileHashes = make(map[string]string)
}

// loadFile parses a file and adds its elements to the scope.
// It detects encoding issues and emits appropriate facts.
func (s *FileScope) loadFile(path string) error {
	if !strings.HasSuffix(path, ".go") {
		return nil // Only Go files for now
	}

	// Read file content first for validation
	content, err := os.ReadFile(path)
	if err != nil {
		s.emitErrorFact("file_not_found", path, err.Error())
		return err
	}

	// Check file size thresholds (> 1MB or > 10K lines is "large")
	byteSize := int64(len(content))
	lines := strings.Split(string(content), "\n")
	lineCount := len(lines)
	isLarge := byteSize > 1024*1024 || lineCount > 10000

	if isLarge {
		s.emitFact(core.Fact{
			Predicate: "large_file_warning",
			Args:      []interface{}{path, int64(lineCount), byteSize},
		})
	}

	// Detect encoding issues
	encoding := detectEncoding(content)
	if encoding.HasBOM {
		s.emitFact(core.Fact{
			Predicate: "encoding_issue",
			Args:      []interface{}{path, "/bom_detected"},
		})
	}
	if encoding.MixedLineEnding {
		s.emitFact(core.Fact{
			Predicate: "encoding_issue",
			Args:      []interface{}{path, "/crlf_inconsistent"},
		})
	}
	if !encoding.IsValidUTF8 {
		s.emitFact(core.Fact{
			Predicate: "encoding_issue",
			Args:      []interface{}{path, "/non_utf8"},
		})
	}

	// Parse with panic recovery
	elements, parseErr := s.safeParseFile(path)
	if parseErr != nil {
		s.emitErrorFact("parse_error", path, parseErr.Error())
		// Don't fail entirely - continue with other files
		return parseErr
	}

	s.Elements = append(s.Elements, elements...)

	// Update hash
	s.FileHashes[path] = computeFileHash(content)

	return nil
}

// safeParseFile wraps parser.ParseFile with panic recovery.
func (s *FileScope) safeParseFile(path string) (elements []CodeElement, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during parse: %v", r)
		}
	}()
	return s.parser.ParseFile(path)
}

// emitFact emits a single fact via the callback.
func (s *FileScope) emitFact(fact core.Fact) {
	if s.factCallback != nil {
		s.factCallback(fact)
	}
}

// emitErrorFact emits an error fact with timestamp.
func (s *FileScope) emitErrorFact(predicate, path, errMsg string) {
	if s.factCallback != nil {
		s.factCallback(core.Fact{
			Predicate: predicate,
			Args:      []interface{}{path, errMsg, time.Now().Unix()},
		})
	}
}

// VerifyFileHash checks if a file has been modified since it was loaded.
// Returns true if the file is unchanged, false if it was modified externally.
func (s *FileScope) VerifyFileHash(path string) (bool, error) {
	s.mu.RLock()
	expectedHash, exists := s.FileHashes[path]
	s.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("file not in scope: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	actualHash := computeFileHash(content)
	if actualHash != expectedHash {
		s.emitFact(core.Fact{
			Predicate: "file_hash_mismatch",
			Args:      []interface{}{path, expectedHash, actualHash},
		})
		return false, nil
	}

	return true, nil
}

// ValidateElementRef checks if an element reference is still valid.
// Returns the element if valid, nil and an error otherwise.
func (s *FileScope) ValidateElementRef(ref string) (*CodeElement, error) {
	// Get element under read lock
	s.mu.RLock()
	elem := GetElement(s.Elements, ref)
	if elem == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("element not found: %s", ref)
	}
	// Copy the file path before releasing lock
	filePath := elem.File
	s.mu.RUnlock()

	// Verify the file hasn't changed (without holding lock)
	valid, err := s.VerifyFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to verify file: %w", err)
	}
	if !valid {
		s.emitFact(core.Fact{
			Predicate: "element_stale",
			Args:      []interface{}{ref, "file_modified"},
		})
		return nil, fmt.Errorf("element stale: file was modified externally")
	}

	// Re-acquire lock to return element (it could have changed, but caller should handle)
	s.mu.RLock()
	elem = GetElement(s.Elements, ref)
	s.mu.RUnlock()

	return elem, nil
}

// RefreshWithRetry attempts to refresh the scope with retry logic.
func (s *FileScope) RefreshWithRetry(maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := s.Refresh(); err != nil {
			lastErr = err
			// Brief pause before retry
			time.Sleep(time.Duration(50*(i+1)) * time.Millisecond)
			continue
		}
		return nil
	}
	if lastErr != nil {
		s.emitErrorFact("scope_refresh_failed", s.ActiveFile, lastErr.Error())
	}
	return lastErr
}

// findOutboundDeps finds import paths from a Go file.
func (s *FileScope) findOutboundDeps(path string) ([]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	var imports []string
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		imports = append(imports, importPath)
	}
	return imports, nil
}

// findInboundDeps finds files in the project that import the given file's package.
func (s *FileScope) findInboundDeps(path string) ([]string, error) {
	// Get the package name of the target file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
	if err != nil {
		return nil, err
	}
	pkgName := node.Name.Name

	// Get the import path for this package
	pkgImportPath := s.getImportPathForFile(path)
	if pkgImportPath == "" {
		return nil, nil // Can't determine import path
	}

	var inbound []string

	// Walk project to find files that import this package
	err = filepath.Walk(s.ProjectRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			// Skip hidden directories and vendor
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		if p == path {
			return nil // Skip self
		}

		// Check if this file imports the target package
		imports, _ := s.findOutboundDeps(p)
		for _, imp := range imports {
			if imp == pkgImportPath || strings.HasSuffix(imp, "/"+pkgName) {
				inbound = append(inbound, p)
				break
			}
		}
		return nil
	})

	return inbound, err
}

// resolveImportPath resolves an import path to an absolute file path.
func (s *FileScope) resolveImportPath(importPath string) string {
	// Standard library: skip
	if !strings.Contains(importPath, ".") && !strings.Contains(importPath, "/") {
		return ""
	}

	// Module-relative import
	if s.ModulePath != "" && strings.HasPrefix(importPath, s.ModulePath) {
		relative := strings.TrimPrefix(importPath, s.ModulePath)
		relative = strings.TrimPrefix(relative, "/")
		pkgDir := filepath.Join(s.ProjectRoot, relative)

		// Find first .go file in package
		entries, err := os.ReadDir(pkgDir)
		if err != nil {
			return ""
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				return filepath.Join(pkgDir, entry.Name())
			}
		}
	}

	return ""
}

// getImportPathForFile returns the import path for a file's package.
func (s *FileScope) getImportPathForFile(path string) string {
	if s.ModulePath == "" {
		return ""
	}

	absPath, _ := filepath.Abs(path)
	dir := filepath.Dir(absPath)

	relPath, err := filepath.Rel(s.ProjectRoot, dir)
	if err != nil {
		return ""
	}

	// Convert to forward slashes for import path
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	if relPath == "." {
		return s.ModulePath
	}
	return s.ModulePath + "/" + relPath
}

// detectModulePath reads the go.mod file to find the module path.
func (s *FileScope) detectModulePath() error {
	modPath := filepath.Join(s.ProjectRoot, "go.mod")
	content, err := os.ReadFile(modPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			s.ModulePath = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			return nil
		}
	}
	return fmt.Errorf("module directive not found in go.mod")
}

// emitScopeFacts emits Mangle facts for the current scope.
func (s *FileScope) emitScopeFacts() {
	if s.factCallback == nil {
		return
	}

	// Emit active_file fact
	s.factCallback(core.Fact{
		Predicate: "active_file",
		Args:      []interface{}{s.ActiveFile},
	})

	// Emit file_in_scope facts
	for _, file := range s.InScope {
		hash := s.FileHashes[file]
		lineCount := 0
		if lines, err := readFileLines(file); err == nil {
			lineCount = len(lines)
		}
		s.factCallback(core.Fact{
			Predicate: "file_in_scope",
			Args:      []interface{}{file, hash, "/go", int64(lineCount)},
		})
	}

	// Emit code element facts
	for _, elem := range s.Elements {
		for _, fact := range elem.ToFacts() {
			s.factCallback(fact)
		}
	}
}

// readFileLines reads a file and returns its lines.
func readFileLines(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(content), "\n"), nil
}

// computeFileHash computes a SHA256 hash of file content.
func computeFileHash(content []byte) string {
	if len(content) == 0 {
		return "empty"
	}
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash[:8]) // First 8 bytes = 16 hex chars
}

// EncodingInfo contains file encoding detection results.
type EncodingInfo struct {
	HasBOM          bool
	BOMType         string // "utf8", "utf16le", "utf16be"
	HasCRLF         bool
	HasLF           bool
	MixedLineEnding bool
	IsValidUTF8     bool
}

// detectEncoding analyzes file content for encoding issues.
func detectEncoding(content []byte) EncodingInfo {
	info := EncodingInfo{
		IsValidUTF8: utf8.Valid(content),
	}

	// Check for BOM (Byte Order Mark)
	if len(content) >= 3 && bytes.Equal(content[:3], []byte{0xEF, 0xBB, 0xBF}) {
		info.HasBOM = true
		info.BOMType = "utf8"
	} else if len(content) >= 2 {
		if bytes.Equal(content[:2], []byte{0xFF, 0xFE}) {
			info.HasBOM = true
			info.BOMType = "utf16le"
		} else if bytes.Equal(content[:2], []byte{0xFE, 0xFF}) {
			info.HasBOM = true
			info.BOMType = "utf16be"
		}
	}

	// Check line endings
	info.HasCRLF = bytes.Contains(content, []byte{'\r', '\n'})
	// Check for standalone LF (not preceded by CR)
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			if i == 0 || content[i-1] != '\r' {
				info.HasLF = true
				break
			}
		}
	}
	info.MixedLineEnding = info.HasCRLF && info.HasLF

	return info
}

// FileLoadResult contains the result of loading a file with metadata.
type FileLoadResult struct {
	Path         string
	Elements     []CodeElement
	Hash         string
	LineCount    int
	Encoding     EncodingInfo
	ParseError   error
	LoadTime     time.Time
	ByteSize     int64
	IsLargeFile  bool // > 10K lines or > 1MB
}

// ScopeFacts returns all current scope facts as a slice.
func (s *FileScope) ScopeFacts() []core.Fact {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var facts []core.Fact

	// Active file
	if s.ActiveFile != "" {
		facts = append(facts, core.Fact{
			Predicate: "active_file",
			Args:      []interface{}{s.ActiveFile},
		})
	}

	// Files in scope
	for _, file := range s.InScope {
		hash := s.FileHashes[file]
		lineCount := 0
		if lines, err := readFileLines(file); err == nil {
			lineCount = len(lines)
		}
		facts = append(facts, core.Fact{
			Predicate: "file_in_scope",
			Args:      []interface{}{file, hash, "/go", int64(lineCount)},
		})
	}

	// Elements
	for _, elem := range s.Elements {
		facts = append(facts, elem.ToFacts()...)
	}

	return facts
}

// ParseGoPackage parses all Go files in a package directory.
func (s *FileScope) ParseGoPackage(dir string) ([]CodeElement, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var allElements []CodeElement

	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			absPath, _ := filepath.Abs(path)
			elements, err := s.parseAstFile(fset, file, absPath)
			if err != nil {
				continue
			}
			allElements = append(allElements, elements...)
		}
	}

	return allElements, nil
}

// parseAstFile parses an ast.File and returns code elements.
func (s *FileScope) parseAstFile(fset *token.FileSet, file *ast.File, path string) ([]CodeElement, error) {
	// Read file content for body extraction
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")

	var elements []CodeElement
	pkgName := file.Name.Name
	defaultActions := []ActionType{ActionView, ActionReplace, ActionInsertBefore, ActionInsertAfter, ActionDelete}

	// Track struct receivers
	structRefs := make(map[string]string)
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
						name := typeSpec.Name.Name
						ref := fmt.Sprintf("struct:%s.%s", pkgName, name)
						structRefs[name] = ref
					}
				}
			}
		}
	}

	// Parse declarations
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			elem := s.parser.parseFuncDecl(fset, d, path, pkgName, lines, structRefs, defaultActions)
			elements = append(elements, elem)
		case *ast.GenDecl:
			elems := s.parser.parseGenDecl(fset, d, path, pkgName, lines, defaultActions)
			elements = append(elements, elems...)
		}
	}

	return elements, nil
}

// =============================================================================
// CORE.CODESCOPE INTERFACE ADAPTER METHODS
// =============================================================================
// These methods adapt FileScope to satisfy the core.CodeScope interface
// by converting world.CodeElement to core.CodeElement.

// toCoreElement converts a world.CodeElement to core.CodeElement.
func toCoreElement(e *CodeElement) *core.CodeElement {
	if e == nil {
		return nil
	}
	actions := make([]string, len(e.Actions))
	for i, a := range e.Actions {
		actions[i] = string(a)
	}
	return &core.CodeElement{
		Ref:        e.Ref,
		Type:       string(e.Type),
		File:       e.File,
		StartLine:  e.StartLine,
		EndLine:    e.EndLine,
		Signature:  e.Signature,
		Body:       e.Body,
		Parent:     e.Parent,
		Visibility: string(e.Visibility),
		Actions:    actions,
	}
}

// toCoreElements converts a slice of world.CodeElement to core.CodeElement.
func toCoreElements(elements []CodeElement) []core.CodeElement {
	result := make([]core.CodeElement, len(elements))
	for i, e := range elements {
		ce := toCoreElement(&e)
		if ce != nil {
			result[i] = *ce
		}
	}
	return result
}

// GetCoreElement implements core.CodeScope.GetElement.
func (s *FileScope) GetCoreElement(ref string) *core.CodeElement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	elem := GetElement(s.Elements, ref)
	return toCoreElement(elem)
}

// GetCoreElementsByFile implements core.CodeScope.GetElementsByFile.
func (s *FileScope) GetCoreElementsByFile(path string) []core.CodeElement {
	absPath, _ := filepath.Abs(path)
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []CodeElement
	for _, e := range s.Elements {
		if e.File == absPath {
			result = append(result, e)
		}
	}
	return toCoreElements(result)
}

// GetActiveFile returns the currently active file.
func (s *FileScope) GetActiveFile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ActiveFile
}

// GetInScopeFiles returns all files in the current scope.
func (s *FileScope) GetInScopeFiles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.InScope))
	copy(result, s.InScope)
	return result
}
