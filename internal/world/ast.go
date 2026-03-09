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

// Parse parses a source file and returns symbol facts.
func (p *ASTParser) Parse(path string) ([]types.Fact, error) {
	start := time.Now()
	logging.WorldDebug("AST parsing file: %s", filepath.Base(path))

	var facts []types.Fact
	var err error

	if strings.HasSuffix(path, ".go") {
		facts, err = p.parseGo(path)
	} else if strings.HasSuffix(path, ".py") {
		facts, err = p.parsePython(path)
	} else if strings.HasSuffix(path, ".rs") {
		facts, err = p.parseRust(path)
	} else if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".jsx") {
		facts, err = p.parseTypeScript(path)
	} else {
		logging.WorldDebug("Unsupported file type for AST parsing: %s", filepath.Ext(path))
		return nil, nil
	}

	if err != nil {
		logging.Get(logging.CategoryWorld).Error("AST parse failed for %s: %v", path, err)
		return nil, err
	}

	logging.WorldDebug("AST parsed %s: %d facts extracted in %v", filepath.Base(path), len(facts), time.Since(start))
	return facts, nil
}

func (p *ASTParser) parseGo(path string) ([]types.Fact, error) {
	logging.WorldDebug("Delegating Go parsing to Cartographer: %s", filepath.Base(path))
	c := NewCartographer()
	return c.MapFile(path)
}

// parsePython implements tree-sitter-based parsing for Python.
func (p *ASTParser) parsePython(path string) ([]types.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read Python file: %s - %v", path, err)
		return nil, err
	}

	if p.tsParser == nil {
		return nil, fmt.Errorf("tree-sitter parser not initialized for Python: %s", filepath.Base(path))
	}

	logging.WorldDebug("Attempting tree-sitter parsing for Python: %s", filepath.Base(path))
	facts, err := p.tsParser.ParsePython(path, content)
	if err != nil || len(facts) == 0 {
		return nil, fmt.Errorf("tree-sitter parsing failed or returned empty for Python: %s - %w", filepath.Base(path), err)
	}

	logging.WorldDebug("Tree-sitter succeeded for Python: %s (%d facts)", filepath.Base(path), len(facts))
	return facts, nil
}

// parseRust implements tree-sitter-based parsing for Rust.
func (p *ASTParser) parseRust(path string) ([]types.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read Rust file: %s - %v", path, err)
		return nil, err
	}

	if p.tsParser == nil {
		return nil, fmt.Errorf("tree-sitter parser not initialized for Rust: %s", filepath.Base(path))
	}

	logging.WorldDebug("Attempting tree-sitter parsing for Rust: %s", filepath.Base(path))
	facts, err := p.tsParser.ParseRust(path, content)
	if err != nil || len(facts) == 0 {
		return nil, fmt.Errorf("tree-sitter parsing failed or returned empty for Rust: %s - %w", filepath.Base(path), err)
	}

	logging.WorldDebug("Tree-sitter succeeded for Rust: %s (%d facts)", filepath.Base(path), len(facts))
	return facts, nil
}

// parseTypeScript implements tree-sitter-based parsing for TS/JS.
func (p *ASTParser) parseTypeScript(path string) ([]types.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read TS/JS file: %s - %v", path, err)
		return nil, err
	}

	if p.tsParser == nil {
		return nil, fmt.Errorf("tree-sitter parser not initialized for TS/JS: %s", filepath.Base(path))
	}

	var facts []types.Fact
	var parseErr error

	// Determine if TypeScript or JavaScript
	if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
		logging.WorldDebug("Attempting tree-sitter parsing for TypeScript: %s", filepath.Base(path))
		facts, parseErr = p.tsParser.ParseTypeScript(path, content)
	} else {
		logging.WorldDebug("Attempting tree-sitter parsing for JavaScript: %s", filepath.Base(path))
		facts, parseErr = p.tsParser.ParseJavaScript(path, content)
	}

	if parseErr != nil || len(facts) == 0 {
		return nil, fmt.Errorf("tree-sitter parsing failed or returned empty for TS/JS: %s - %w", filepath.Base(path), parseErr)
	}
	
	logging.WorldDebug("Tree-sitter succeeded for TS/JS: %s (%d facts)", filepath.Base(path), len(facts))
	return facts, nil
}


// Close releases resources held by the AST parser
func (p *ASTParser) Close() {
	logging.WorldDebug("Closing ASTParser")
	if p.tsParser != nil {
		p.tsParser.Close()
	}
}
