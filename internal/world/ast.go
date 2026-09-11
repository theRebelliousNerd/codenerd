package world

import (
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ASTParser handles code parsing.
type ASTParser struct {
	tsParser *TreeSitterParser
}

// NewASTParser creates a new AST parser with tree-sitter support.
func NewASTParser() *ASTParser {
	logging.WorldDebug("Creating new ASTParser")
	return &ASTParser{
		tsParser: NewTreeSitterParser(),
	}
}

// ParseAs parses the source file at fsPath and returns symbol facts labelled
// with factPath.
//
// The two paths are deliberately separate, as on Cartographer.MapFileAs: the
// parser needs a path it can open, while every fact must carry the file's
// canonical (workspace-relative, forward-slash) identity so it joins the
// file_topology row the scanners emitted for the same file. The single-path
// form this replaces stamped whatever it was handed into the facts, and the
// callers that had an absolute path to open produced symbol_graph and
// dependency_link facts under an identity no other fact shared.
func (p *ASTParser) ParseAs(fsPath, factPath string) ([]types.Fact, error) {
	start := time.Now()
	logging.WorldDebug("AST parsing file: %s", filepath.Base(fsPath))

	var facts []types.Fact
	var err error

	switch {
	case strings.HasSuffix(fsPath, ".go"):
		facts, err = p.parseGo(fsPath, factPath)
	case strings.HasSuffix(fsPath, ".py"):
		facts, err = p.parsePython(fsPath, factPath)
	case strings.HasSuffix(fsPath, ".rs"):
		facts, err = p.parseRust(fsPath, factPath)
	case strings.HasSuffix(fsPath, ".ts"), strings.HasSuffix(fsPath, ".js"), strings.HasSuffix(fsPath, ".tsx"), strings.HasSuffix(fsPath, ".jsx"):
		facts, err = p.parseTypeScript(fsPath, factPath)
	default:
		logging.WorldDebug("Unsupported file type for AST parsing: %s", filepath.Ext(fsPath))
		return nil, nil
	}

	if err != nil {
		logging.Get(logging.CategoryWorld).Error("AST parse failed for %s: %v", fsPath, err)
		return nil, err
	}

	logging.WorldDebug("AST parsed %s: %d facts extracted in %v", filepath.Base(fsPath), len(facts), time.Since(start))
	return facts, nil
}

// parseGo emits the same fast-depth facts (symbol_graph, dependency_link) the
// full and incremental scanners emit for a Go file, from the same tree-sitter
// walker. It used to delegate to the Cartographer, which produces the DEEP
// predicates (code_defines, code_calls, data flow) and no symbol_graph or
// dependency_link at all — so a Go file reached through this parser (chat
// /scan-path, /scan-dir, the world-model shard) got deep facts filed under the
// fast cache depth and never contributed an import edge or a symbol, while
// the same file through `nerd scan` got the opposite set.
func (p *ASTParser) parseGo(fsPath, factPath string) ([]types.Fact, error) {
	content, err := os.ReadFile(fsPath)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read Go file: %s - %v", fsPath, err)
		return nil, err
	}
	if p.tsParser == nil {
		return nil, fmt.Errorf("tree-sitter parser not initialized for Go: %s", filepath.Base(fsPath))
	}
	facts, err := p.tsParser.ParseGo(factPath, content)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parsing failed for Go: %s - %w", filepath.Base(fsPath), err)
	}
	return facts, nil
}

// parsePython implements tree-sitter-based parsing for Python.
func (p *ASTParser) parsePython(fsPath, factPath string) ([]types.Fact, error) {
	content, err := os.ReadFile(fsPath)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read Python file: %s - %v", fsPath, err)
		return nil, err
	}

	if p.tsParser == nil {
		return nil, fmt.Errorf("tree-sitter parser not initialized for Python: %s", filepath.Base(fsPath))
	}

	logging.WorldDebug("Attempting tree-sitter parsing for Python: %s", filepath.Base(fsPath))
	facts, err := p.tsParser.ParsePython(factPath, content)
	if err != nil || len(facts) == 0 {
		return nil, fmt.Errorf("tree-sitter parsing failed or returned empty for Python: %s - %w", filepath.Base(fsPath), err)
	}

	logging.WorldDebug("Tree-sitter succeeded for Python: %s (%d facts)", filepath.Base(fsPath), len(facts))
	return facts, nil
}

// parseRust implements tree-sitter-based parsing for Rust.
func (p *ASTParser) parseRust(fsPath, factPath string) ([]types.Fact, error) {
	content, err := os.ReadFile(fsPath)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read Rust file: %s - %v", fsPath, err)
		return nil, err
	}

	if p.tsParser == nil {
		return nil, fmt.Errorf("tree-sitter parser not initialized for Rust: %s", filepath.Base(fsPath))
	}

	logging.WorldDebug("Attempting tree-sitter parsing for Rust: %s", filepath.Base(fsPath))
	facts, err := p.tsParser.ParseRust(factPath, content)
	if err != nil || len(facts) == 0 {
		return nil, fmt.Errorf("tree-sitter parsing failed or returned empty for Rust: %s - %w", filepath.Base(fsPath), err)
	}

	logging.WorldDebug("Tree-sitter succeeded for Rust: %s (%d facts)", filepath.Base(fsPath), len(facts))
	return facts, nil
}

// parseTypeScript implements tree-sitter-based parsing for TS/JS.
func (p *ASTParser) parseTypeScript(fsPath, factPath string) ([]types.Fact, error) {
	content, err := os.ReadFile(fsPath)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read TS/JS file: %s - %v", fsPath, err)
		return nil, err
	}

	if p.tsParser == nil {
		return nil, fmt.Errorf("tree-sitter parser not initialized for TS/JS: %s", filepath.Base(fsPath))
	}

	var facts []types.Fact
	var parseErr error

	// Determine if TypeScript or JavaScript
	if strings.HasSuffix(fsPath, ".ts") || strings.HasSuffix(fsPath, ".tsx") {
		logging.WorldDebug("Attempting tree-sitter parsing for TypeScript: %s", filepath.Base(fsPath))
		facts, parseErr = p.tsParser.ParseTypeScript(factPath, content)
	} else {
		logging.WorldDebug("Attempting tree-sitter parsing for JavaScript: %s", filepath.Base(fsPath))
		facts, parseErr = p.tsParser.ParseJavaScript(factPath, content)
	}

	if parseErr != nil || len(facts) == 0 {
		return nil, fmt.Errorf("tree-sitter parsing failed or returned empty for TS/JS: %s - %w", filepath.Base(fsPath), parseErr)
	}

	logging.WorldDebug("Tree-sitter succeeded for TS/JS: %s (%d facts)", filepath.Base(fsPath), len(facts))
	return facts, nil
}

// Close releases resources held by the AST parser
func (p *ASTParser) Close() {
	logging.WorldDebug("Closing ASTParser")
	if p.tsParser != nil {
		p.tsParser.Close()
	}
}
