package mangle

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// ============================================================================
// LSP Server for Mangle (.mg) Files - Cortex 1.5.0
// Implements Language Server Protocol for IDE integration
// ============================================================================

// LSPServer provides language intelligence for Mangle files.
type LSPServer struct {
	mu          sync.RWMutex
	engine      *Engine
	documents   map[string]*Document    // Open documents by URI
	definitions map[string][]Definition // Definitions by symbol name
	references  map[string][]Reference  // References by symbol name
	diagnostics map[string][]Diagnostic // Diagnostics by file URI
	hover       map[string]string       // Hover documentation by symbol
}

// Document represents an open Mangle file.
type Document struct {
	URI     string
	Version int
	Content string
	Lines   []string
}

// Definition represents where a symbol is defined.
type Definition struct {
	Symbol   string
	FilePath string
	Line     int
	Column   int
	Kind     SymbolKind
	Arity    int
}

// Reference represents where a symbol is used.
type Reference struct {
	Symbol   string
	FilePath string
	Line     int
	Column   int
	Kind     ReferenceKind
}

// Diagnostic represents a problem in the code.
type Diagnostic struct {
	FilePath string
	Line     int
	Column   int
	EndCol   int
	Severity DiagnosticSeverity
	Message  string
	Code     string
	Source   string
}

// SymbolKind indicates what kind of symbol a definition is.
type SymbolKind int

const (
	SymbolPredicate SymbolKind = iota
	SymbolRule
	SymbolFact
	SymbolNameConstant
)

// ReferenceKind indicates how a symbol is being used.
type ReferenceKind int

const (
	RefInHead  ReferenceKind = iota // Used in rule head
	RefInBody                       // Used in rule body
	RefInFact                       // Used in a fact
	RefInQuery                      // Used in a query
)

// DiagnosticSeverity follows LSP severity levels.
type DiagnosticSeverity int

const (
	DiagError       DiagnosticSeverity = 1
	DiagWarning     DiagnosticSeverity = 2
	DiagInformation DiagnosticSeverity = 3
	DiagHint        DiagnosticSeverity = 4
)

// NewLSPServer creates a new LSP server.
func NewLSPServer(engine *Engine) *LSPServer {
	return &LSPServer{
		engine:      engine,
		documents:   make(map[string]*Document),
		definitions: make(map[string][]Definition),
		references:  make(map[string][]Reference),
		diagnostics: make(map[string][]Diagnostic),
		hover:       make(map[string]string),
	}
}

// ============================================================================
// Document Management
// ============================================================================

// OpenDocument opens or updates a document.
func (s *LSPServer) OpenDocument(uri string, content string, version int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines := strings.Split(content, "\n")
	s.documents[uri] = &Document{
		URI:     uri,
		Version: version,
		Content: content,
		Lines:   lines,
	}

	// Re-index on document open/change
	s.indexDocumentLocked(uri, content)
}

// CloseDocument removes a document from the server.
func (s *LSPServer) CloseDocument(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.documents, uri)
}

// ============================================================================
// Indexing
// ============================================================================

// indexDocumentLocked indexes a document's symbols (must hold lock).
func (s *LSPServer) indexDocumentLocked(uri string, content string) {
	filePath := uriToPath(uri)

	// Clear existing entries for this file
	s.clearFileEntriesLocked(filePath)

	lines := strings.Split(content, "\n")

	// Parse patterns for Mangle syntax
	predicatePattern := regexp.MustCompile(`^(\w+)\s*\(`)
	nameConstantPattern := regexp.MustCompile(`/[\w_]+`)
	rulePattern := regexp.MustCompile(`^(\w+)\s*\([^)]*\)\s*:-`)
	declPattern := regexp.MustCompile(`^Decl\s+(\w+)\s*\(`)

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for declaration
		if matches := declPattern.FindStringSubmatch(line); len(matches) > 1 {
			predName := matches[1]
			s.addDefinition(predName, filePath, lineNum+1, 0, SymbolPredicate, countArity(line))
			s.hover[predName] = fmt.Sprintf("**Predicate Declaration**\n\n`%s`", line)
			continue
		}

		// Check for rule (has :-)
		if matches := rulePattern.FindStringSubmatch(line); len(matches) > 1 {
			predName := matches[1]
			s.addDefinition(predName, filePath, lineNum+1, 0, SymbolRule, countArity(line))
			s.addReference(predName, filePath, lineNum+1, 0, RefInHead)

			// Extract body predicates
			bodyStart := strings.Index(line, ":-")
			if bodyStart > 0 {
				body := line[bodyStart+2:]
				bodyPreds := predicatePattern.FindAllStringSubmatch(body, -1)
				for _, bp := range bodyPreds {
					if len(bp) > 1 {
						s.addReference(bp[1], filePath, lineNum+1, strings.Index(body, bp[1])+bodyStart+2, RefInBody)
					}
				}
			}
			continue
		}

		// Check for fact (predicate without :-)
		if matches := predicatePattern.FindStringSubmatch(line); len(matches) > 1 {
			predName := matches[1]
			if predName != "Decl" && predName != "fn" && predName != "let" && predName != "do" {
				s.addReference(predName, filePath, lineNum+1, 0, RefInFact)
			}
		}

		// Index all name constants
		nameMatches := nameConstantPattern.FindAllStringIndex(line, -1)
		for _, match := range nameMatches {
			name := line[match[0]:match[1]]
			s.addReference(name, filePath, lineNum+1, match[0], RefInFact)
		}
	}

	// Run diagnostics
	s.runDiagnosticsLocked(uri, content)
}

// clearFileEntriesLocked removes all entries for a file.
func (s *LSPServer) clearFileEntriesLocked(filePath string) {
	// Clear definitions
	for symbol, defs := range s.definitions {
		var filtered []Definition
		for _, d := range defs {
			if d.FilePath != filePath {
				filtered = append(filtered, d)
			}
		}
		if len(filtered) > 0 {
			s.definitions[symbol] = filtered
		} else {
			delete(s.definitions, symbol)
		}
	}

	// Clear references
	for symbol, refs := range s.references {
		var filtered []Reference
		for _, r := range refs {
			if r.FilePath != filePath {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) > 0 {
			s.references[symbol] = filtered
		} else {
			delete(s.references, symbol)
		}
	}

	// Clear diagnostics
	delete(s.diagnostics, pathToURI(filePath))
}

func (s *LSPServer) addDefinition(symbol, filePath string, line, col int, kind SymbolKind, arity int) {
	def := Definition{
		Symbol:   symbol,
		FilePath: filePath,
		Line:     line,
		Column:   col,
		Kind:     kind,
		Arity:    arity,
	}
	s.definitions[symbol] = append(s.definitions[symbol], def)
}

func (s *LSPServer) addReference(symbol, filePath string, line, col int, kind ReferenceKind) {
	ref := Reference{
		Symbol:   symbol,
		FilePath: filePath,
		Line:     line,
		Column:   col,
		Kind:     kind,
	}
	s.references[symbol] = append(s.references[symbol], ref)
}

// ============================================================================
// Diagnostics
// ============================================================================

// runDiagnosticsLocked runs diagnostics on a document.
func (s *LSPServer) runDiagnosticsLocked(uri string, content string) {
	filePath := uriToPath(uri)
	var diagnostics []Diagnostic

	lines := strings.Split(content, "\n")

	// Check each line for common errors
	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for missing period at end of facts/rules
		if !strings.HasSuffix(trimmed, ".") && !strings.HasSuffix(trimmed, ",") &&
			!strings.HasSuffix(trimmed, ":-") && !strings.HasSuffix(trimmed, "{") &&
			!strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "//") {
			// Likely a continuation line or error
			if strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")") &&
				!strings.Contains(trimmed, ":-") {
				diagnostics = append(diagnostics, Diagnostic{
					FilePath: filePath,
					Line:     lineNum + 1,
					Column:   len(trimmed),
					Severity: DiagWarning,
					Message:  "Statement may be missing terminating period '.'",
					Code:     "missing-period",
					Source:   "mangle-lsp",
				})
			}
		}

		// Check for unbalanced parentheses
		opens := strings.Count(trimmed, "(")
		closes := strings.Count(trimmed, ")")
		if opens != closes {
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     lineNum + 1,
				Column:   0,
				Severity: DiagError,
				Message:  fmt.Sprintf("Unbalanced parentheses: %d open, %d close", opens, closes),
				Code:     "unbalanced-parens",
				Source:   "mangle-lsp",
			})
		}

		// Check for common Mangle syntax patterns that might be wrong
		if strings.Contains(trimmed, ":-") {
			// Rule with no body predicates
			bodyStart := strings.Index(trimmed, ":-")
			body := strings.TrimSpace(trimmed[bodyStart+2:])
			body = strings.TrimSuffix(body, ".")
			if body == "" {
				diagnostics = append(diagnostics, Diagnostic{
					FilePath: filePath,
					Line:     lineNum + 1,
					Column:   bodyStart + 2,
					Severity: DiagError,
					Message:  "Rule has empty body",
					Code:     "empty-body",
					Source:   "mangle-lsp",
				})
			}
		}

		// Check for undefined predicates (would need full indexing)
		// This is a placeholder for more sophisticated analysis
	}

	s.diagnostics[uri] = diagnostics
}

// GetDiagnostics returns diagnostics for a document.
func (s *LSPServer) GetDiagnostics(uri string) []Diagnostic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.diagnostics[uri]
}

// ============================================================================
// Go To Definition
// ============================================================================

// GoToDefinition finds the definition of a symbol at a position.
func (s *LSPServer) GoToDefinition(uri string, line, col int) []Definition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.documents[uri]
	if !ok || line < 1 || line > len(doc.Lines) {
		return nil
	}

	// Get the word at the position
	symbol := getWordAtPosition(doc.Lines[line-1], col)
	if symbol == "" {
		return nil
	}

	return s.definitions[symbol]
}

// ============================================================================
// Find References
// ============================================================================

// FindReferences finds all references to a symbol.
func (s *LSPServer) FindReferences(uri string, line, col int, includeDeclaration bool) []Reference {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.documents[uri]
	if !ok || line < 1 || line > len(doc.Lines) {
		return nil
	}

	symbol := getWordAtPosition(doc.Lines[line-1], col)
	if symbol == "" {
		return nil
	}

	refs := s.references[symbol]
	if includeDeclaration {
		// Add definition locations as references
		for _, def := range s.definitions[symbol] {
			refs = append(refs, Reference{
				Symbol:   def.Symbol,
				FilePath: def.FilePath,
				Line:     def.Line,
				Column:   def.Column,
				Kind:     RefInHead,
			})
		}
	}

	return refs
}

// ============================================================================
// Hover
// ============================================================================

// GetHover returns hover information for a symbol.
func (s *LSPServer) GetHover(uri string, line, col int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.documents[uri]
	if !ok || line < 1 || line > len(doc.Lines) {
		return ""
	}

	symbol := getWordAtPosition(doc.Lines[line-1], col)
	if symbol == "" {
		return ""
	}

	// Check for predefined hover
	if hover, ok := s.hover[symbol]; ok {
		return hover
	}

	// Generate hover from definitions
	defs := s.definitions[symbol]
	if len(defs) == 0 {
		// Check if it's a name constant
		if strings.HasPrefix(symbol, "/") {
			return fmt.Sprintf("**Name Constant**\n\n`%s`", symbol)
		}
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s**\n\n", symbol))

	for _, def := range defs {
		switch def.Kind {
		case SymbolPredicate:
			sb.WriteString(fmt.Sprintf("Predicate (arity %d)\n", def.Arity))
		case SymbolRule:
			sb.WriteString(fmt.Sprintf("Rule (arity %d)\n", def.Arity))
		case SymbolFact:
			sb.WriteString("Fact\n")
		}
		sb.WriteString(fmt.Sprintf("Defined at: %s:%d\n", def.FilePath, def.Line))
	}

	// Add reference count
	refs := s.references[symbol]
	sb.WriteString(fmt.Sprintf("\n%d references\n", len(refs)))

	return sb.String()
}

// ============================================================================
// Completion
// ============================================================================

// CompletionItem represents a completion suggestion.
type CompletionItem struct {
	Label         string
	Kind          CompletionKind
	Detail        string
	Documentation string
	InsertText    string
}

// CompletionKind follows LSP completion item kinds.
type CompletionKind int

const (
	CompletionFunction CompletionKind = 3
	CompletionField    CompletionKind = 5
	CompletionConstant CompletionKind = 21
	CompletionKeyword  CompletionKind = 14
)

// GetCompletions returns completion items at a position.
func (s *LSPServer) GetCompletions(uri string, line, col int) []CompletionItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.documents[uri]
	if !ok || line < 1 || line > len(doc.Lines) {
		return nil
	}

	lineText := doc.Lines[line-1]
	prefix := getWordPrefixAtPosition(lineText, col)

	var items []CompletionItem

	// Add predicates
	for symbol, defs := range s.definitions {
		if strings.HasPrefix(symbol, prefix) {
			for _, def := range defs {
				items = append(items, CompletionItem{
					Label:      symbol,
					Kind:       CompletionFunction,
					Detail:     fmt.Sprintf("(%d args)", def.Arity),
					InsertText: symbol,
				})
				break // Only add once per symbol
			}
		}
	}

	// Add keywords
	keywords := []string{"Decl", "fn", "do", "let"}
	for _, kw := range keywords {
		if strings.HasPrefix(kw, prefix) {
			items = append(items, CompletionItem{
				Label: kw,
				Kind:  CompletionKeyword,
			})
		}
	}

	// Add common name constants if typing /
	if strings.HasPrefix(prefix, "/") || prefix == "" {
		constants := []string{
			"/query", "/mutation", "/instruction",
			"/explain", "/refactor", "/debug", "/generate",
			"/go", "/python", "/ts", "/rust",
			"/error", "/warning", "/info",
			"/true", "/false",
		}
		for _, c := range constants {
			if strings.HasPrefix(c, prefix) {
				items = append(items, CompletionItem{
					Label: c,
					Kind:  CompletionConstant,
				})
			}
		}
	}

	// Add built-in functions if typing fn:
	if strings.HasPrefix(prefix, "fn:") || strings.HasSuffix(lineText[:min(col, len(lineText))], "fn:") {
		builtins := []string{
			"fn:count", "fn:sum", "fn:max", "fn:min",
			"fn:plus", "fn:minus", "fn:mult", "fn:div",
			"fn:group_by", "fn:list:get", "fn:pair",
		}
		for _, b := range builtins {
			items = append(items, CompletionItem{
				Label:  b,
				Kind:   CompletionFunction,
				Detail: "Built-in function",
			})
		}
	}

	return items
}

// ============================================================================
// Batch Query API (for World Model Projection)
// ============================================================================

// GetDefinitions returns all definitions for a symbol.
// Used by LSP Manager to project definitions into World Model facts.
func (s *LSPServer) GetDefinitions(symbol string) []Definition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.definitions[symbol]
}

// GetReferences returns all references to a symbol.
// Used by LSP Manager to project references into World Model facts.
func (s *LSPServer) GetReferences(symbol string) []Reference {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.references[symbol]
}

// GetAllDefinitions returns all definitions across all symbols.
// Used by LSP Manager for full World Model fact projection.
func (s *LSPServer) GetAllDefinitions() map[string][]Definition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string][]Definition, len(s.definitions))
	for symbol, defs := range s.definitions {
		result[symbol] = append([]Definition(nil), defs...)
	}
	return result
}

// GetAllReferences returns all references across all symbols.
// Used by LSP Manager for full World Model fact projection.
func (s *LSPServer) GetAllReferences() map[string][]Reference {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string][]Reference, len(s.references))
	for symbol, refs := range s.references {
		result[symbol] = append([]Reference(nil), refs...)
	}
	return result
}

// GetAllDiagnostics returns all diagnostics across all documents.
// Used by LSP Manager for full World Model fact projection.
func (s *LSPServer) GetAllDiagnostics() map[string][]Diagnostic {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string][]Diagnostic, len(s.diagnostics))
	for uri, diags := range s.diagnostics {
		result[uri] = append([]Diagnostic(nil), diags...)
	}
	return result
}

// ValidateCode validates Mangle code without opening it as a document.
// Returns diagnostics for the given code.
// Used by CoderShard and LegislatorShard to validate generated code.
func (s *LSPServer) ValidateCode(uri, content string) []Diagnostic {
	var diags []Diagnostic
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for missing period
		if !strings.HasSuffix(line, ".") && !strings.HasSuffix(line, ":-") && !strings.Contains(line, "|>") {
			diags = append(diags, Diagnostic{
				FilePath: uriToPath(uri),
				Line:     lineNum + 1,
				Column:   len(line),
				Severity: DiagWarning,
				Message:  "Statement should end with period",
				Code:     "missing-period",
				Source:   "mangle-lsp",
			})
		}

		// Check for unbalanced parentheses
		openCount := strings.Count(line, "(")
		closeCount := strings.Count(line, ")")
		if openCount != closeCount {
			diags = append(diags, Diagnostic{
				FilePath: uriToPath(uri),
				Line:     lineNum + 1,
				Column:   0,
				Severity: DiagError,
				Message:  "Unbalanced parentheses",
				Code:     "unbalanced-parens",
				Source:   "mangle-lsp",
			})
		}

		// Check for empty rule body
		if strings.Contains(line, ":-") {
			parts := strings.Split(line, ":-")
			if len(parts) > 1 && strings.TrimSpace(parts[1]) == "." {
				diags = append(diags, Diagnostic{
					FilePath: uriToPath(uri),
					Line:     lineNum + 1,
					Column:   strings.Index(line, ":-"),
					Severity: DiagError,
					Message:  "Empty rule body",
					Code:     "empty-body",
					Source:   "mangle-lsp",
				})
			}
		}
	}

	return diags
}

// ============================================================================
// Index Workspace
// ============================================================================

// IndexWorkspace indexes all .mg files in a directory.
func (s *LSPServer) IndexWorkspace(ctx context.Context, rootPath string) error {
	return filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			// Skip common non-source directories
			if d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, ".mg") || strings.HasSuffix(path, ".mangle") {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip files we can't read
			}

			uri := pathToURI(path)
			s.OpenDocument(uri, string(content), 1)
		}

		return nil
	})
}

// ============================================================================
// LSP Protocol (JSON-RPC)
// ============================================================================

// LSPRequest represents an LSP JSON-RPC request.
type LSPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// LSPResponse represents an LSP JSON-RPC response.
type LSPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *LSPError   `json:"error,omitempty"`
}

// LSPError represents an LSP error.
type LSPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServeStdio starts the LSP server on stdin/stdout.
func (s *LSPServer) ServeStdio(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read Content-Length header
		header, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		var contentLength int
		if strings.HasPrefix(header, "Content-Length: ") {
			lengthStr := strings.TrimPrefix(header, "Content-Length: ")
			lengthStr = strings.TrimSpace(lengthStr)
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				continue
			}
		} else {
			continue
		}

		// Skip blank line
		reader.ReadString('\n')

		// Read content
		content := make([]byte, contentLength)
		_, err = io.ReadFull(reader, content)
		if err != nil {
			continue
		}

		// Parse request
		var req LSPRequest
		if err := json.Unmarshal(content, &req); err != nil {
			continue
		}

		// Handle request
		response := s.handleRequest(req)
		if response != nil {
			responseBytes, _ := json.Marshal(response)
			fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n%s", len(responseBytes), responseBytes)
		}
	}
}

// handleRequest processes an LSP request.
func (s *LSPServer) handleRequest(req LSPRequest) *LSPResponse {
	switch req.Method {
	case "initialize":
		return &LSPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"capabilities": map[string]interface{}{
					"textDocumentSync":   1, // Full sync
					"definitionProvider": true,
					"referencesProvider": true,
					"hoverProvider":      true,
					"completionProvider": map[string]interface{}{
						"triggerCharacters": []string{"/", ":", "("},
					},
				},
			},
		}

	case "textDocument/didOpen":
		var params struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Text    string `json:"text"`
				Version int    `json:"version"`
			} `json:"textDocument"`
		}
		json.Unmarshal(req.Params, &params)
		s.OpenDocument(params.TextDocument.URI, params.TextDocument.Text, params.TextDocument.Version)
		return nil

	case "textDocument/didChange":
		var params struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		json.Unmarshal(req.Params, &params)
		if len(params.ContentChanges) > 0 {
			s.OpenDocument(params.TextDocument.URI, params.ContentChanges[0].Text, params.TextDocument.Version)
		}
		return nil

	case "textDocument/definition":
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		json.Unmarshal(req.Params, &params)
		defs := s.GoToDefinition(params.TextDocument.URI, params.Position.Line+1, params.Position.Character)
		if len(defs) == 0 {
			return &LSPResponse{JSONRPC: "2.0", ID: req.ID, Result: nil}
		}
		return &LSPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"uri": pathToURI(defs[0].FilePath),
				"range": map[string]interface{}{
					"start": map[string]int{"line": defs[0].Line - 1, "character": defs[0].Column},
					"end":   map[string]int{"line": defs[0].Line - 1, "character": defs[0].Column},
				},
			},
		}

	case "textDocument/hover":
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		json.Unmarshal(req.Params, &params)
		hover := s.GetHover(params.TextDocument.URI, params.Position.Line+1, params.Position.Character)
		if hover == "" {
			return &LSPResponse{JSONRPC: "2.0", ID: req.ID, Result: nil}
		}
		return &LSPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"contents": map[string]string{
					"kind":  "markdown",
					"value": hover,
				},
			},
		}

	case "shutdown":
		return &LSPResponse{JSONRPC: "2.0", ID: req.ID, Result: nil}

	case "exit":
		os.Exit(0)
		return nil

	default:
		return nil
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		path := strings.TrimPrefix(uri, "file://")
		// Handle Windows paths
		if len(path) > 2 && path[0] == '/' && path[2] == ':' {
			path = path[1:] // Remove leading slash for Windows
		}
		return filepath.FromSlash(path)
	}
	return uri
}

func pathToURI(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") && len(path) > 1 && path[1] == ':' {
		// Windows path
		path = "/" + path
	}
	return "file://" + path
}

func getWordAtPosition(line string, col int) string {
	if col < 0 || col > len(line) {
		return ""
	}

	// Find word boundaries
	start := col
	for start > 0 && isWordChar(line[start-1]) {
		start--
	}

	end := col
	for end < len(line) && isWordChar(line[end]) {
		end++
	}

	return line[start:end]
}

func getWordPrefixAtPosition(line string, col int) string {
	if col < 0 || col > len(line) {
		return ""
	}

	start := col
	for start > 0 && isWordChar(line[start-1]) {
		start--
	}

	return line[start:col]
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '/' || c == ':'
}

func countArity(line string) int {
	// Count arguments by counting commas + 1
	parenStart := strings.Index(line, "(")
	if parenStart < 0 {
		return 0
	}

	depth := 0
	commas := 0
	inQuote := false

	for i := parenStart; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			inQuote = !inQuote
		}
		if !inQuote {
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
				if depth == 0 {
					break
				}
			} else if c == ',' && depth == 1 {
				commas++
			}
		}
	}

	if commas == 0 {
		// Check if there's any content between parens
		parenEnd := strings.Index(line[parenStart:], ")")
		if parenEnd > 0 {
			content := strings.TrimSpace(line[parenStart+1 : parenStart+parenEnd])
			if content == "" {
				return 0
			}
		}
		return 1
	}

	return commas + 1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
