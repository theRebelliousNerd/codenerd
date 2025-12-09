package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
func (p *ASTParser) Parse(path string) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("AST parsing file: %s", filepath.Base(path))

	var facts []core.Fact
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

func (p *ASTParser) parseGo(path string) ([]core.Fact, error) {
	logging.WorldDebug("Delegating Go parsing to Cartographer: %s", filepath.Base(path))
	c := NewCartographer()
	return c.MapFile(path)
}

// parsePython implements tree-sitter-based parsing for Python with regex fallback
func (p *ASTParser) parsePython(path string) ([]core.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read Python file: %s - %v", path, err)
		return nil, err
	}

	// Try tree-sitter parsing first
	if p.tsParser != nil {
		logging.WorldDebug("Attempting tree-sitter parsing for Python: %s", filepath.Base(path))
		facts, err := p.tsParser.ParsePython(path, content)
		if err == nil && len(facts) > 0 {
			logging.WorldDebug("Tree-sitter succeeded for Python: %s (%d facts)", filepath.Base(path), len(facts))
			return facts, nil
		}
		logging.WorldDebug("Tree-sitter failed or empty for Python, falling back to regex: %s", filepath.Base(path))
	}

	// Fallback: regex-based parsing
	logging.WorldDebug("Using regex fallback for Python parsing: %s", filepath.Base(path))
	var facts []core.Fact
	lines := strings.Split(string(content), "\n")

	// Regex for definitions
	classRegex := regexp.MustCompile(`^\s*class\s+(\w+)`)
	defRegex := regexp.MustCompile(`^\s*def\s+(\w+)`)
	importRegex := regexp.MustCompile(`^\s*(?:from|import)\s+(\w+)`)

	var classCount, funcCount, importCount int

	for _, line := range lines {
		// Classes
		if matches := classRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("class:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "class", "public", path, line},
			})
			classCount++
		}

		// Functions
		if matches := defRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("func:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "function", "public", path, line},
			})
			funcCount++
		}

		// Imports
		if matches := importRegex.FindStringSubmatch(line); len(matches) > 1 {
			module := matches[1]
			facts = append(facts, core.Fact{
				Predicate: "dependency_link",
				Args:      []interface{}{path, fmt.Sprintf("mod:%s", module), module},
			})
			importCount++
		}
	}

	logging.WorldDebug("Python regex parsing complete: %s (classes=%d, funcs=%d, imports=%d)",
		filepath.Base(path), classCount, funcCount, importCount)
	return facts, nil
}

// parseRust implements tree-sitter-based parsing for Rust with regex fallback
func (p *ASTParser) parseRust(path string) ([]core.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read Rust file: %s - %v", path, err)
		return nil, err
	}

	// Try tree-sitter parsing first
	if p.tsParser != nil {
		logging.WorldDebug("Attempting tree-sitter parsing for Rust: %s", filepath.Base(path))
		facts, err := p.tsParser.ParseRust(path, content)
		if err == nil && len(facts) > 0 {
			logging.WorldDebug("Tree-sitter succeeded for Rust: %s (%d facts)", filepath.Base(path), len(facts))
			return facts, nil
		}
		logging.WorldDebug("Tree-sitter failed or empty for Rust, falling back to regex: %s", filepath.Base(path))
	}

	// Fallback: regex-based parsing
	logging.WorldDebug("Using regex fallback for Rust parsing: %s", filepath.Base(path))
	var facts []core.Fact
	lines := strings.Split(string(content), "\n")

	// Regex for definitions
	fnRegex := regexp.MustCompile(`^\s*(?:pub\s+)?fn\s+(\w+)`)
	structRegex := regexp.MustCompile(`^\s*(?:pub\s+)?struct\s+(\w+)`)
	enumRegex := regexp.MustCompile(`^\s*(?:pub\s+)?enum\s+(\w+)`)
	modRegex := regexp.MustCompile(`^\s*(?:pub\s+)?mod\s+(\w+)`)
	useRegex := regexp.MustCompile(`^\s*use\s+([\w:]+)`)

	var fnCount, structCount, enumCount, modCount, useCount int

	for _, line := range lines {
		// Functions
		if matches := fnRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("fn:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "function", "public", path, line},
			})
			fnCount++
		}

		// Structs
		if matches := structRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("struct:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "struct", "public", path, line},
			})
			structCount++
		}

		// Enums
		if matches := enumRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("enum:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "enum", "public", path, line},
			})
			enumCount++
		}

		// Modules
		if matches := modRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("mod:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "module", "public", path, line},
			})
			modCount++
		}

		// Imports (use)
		if matches := useRegex.FindStringSubmatch(line); len(matches) > 1 {
			pkg := matches[1]
			facts = append(facts, core.Fact{
				Predicate: "dependency_link",
				Args:      []interface{}{path, fmt.Sprintf("crate:%s", pkg), pkg},
			})
			useCount++
		}
	}

	logging.WorldDebug("Rust regex parsing complete: %s (fns=%d, structs=%d, enums=%d, mods=%d, uses=%d)",
		filepath.Base(path), fnCount, structCount, enumCount, modCount, useCount)
	return facts, nil
}

// parseTypeScript implements tree-sitter-based parsing for TS/JS with regex fallback
func (p *ASTParser) parseTypeScript(path string) ([]core.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read TS/JS file: %s - %v", path, err)
		return nil, err
	}

	// Try tree-sitter parsing first
	if p.tsParser != nil {
		var facts []core.Fact
		var parseErr error

		// Determine if TypeScript or JavaScript
		if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
			logging.WorldDebug("Attempting tree-sitter parsing for TypeScript: %s", filepath.Base(path))
			facts, parseErr = p.tsParser.ParseTypeScript(path, content)
		} else {
			logging.WorldDebug("Attempting tree-sitter parsing for JavaScript: %s", filepath.Base(path))
			facts, parseErr = p.tsParser.ParseJavaScript(path, content)
		}

		if parseErr == nil && len(facts) > 0 {
			logging.WorldDebug("Tree-sitter succeeded for TS/JS: %s (%d facts)", filepath.Base(path), len(facts))
			return facts, nil
		}
		logging.WorldDebug("Tree-sitter failed or empty for TS/JS, falling back to regex: %s", filepath.Base(path))
	}

	// Fallback: regex-based parsing
	logging.WorldDebug("Using regex fallback for TS/JS parsing: %s", filepath.Base(path))
	var facts []core.Fact
	lines := strings.Split(string(content), "\n")

	// Regex for definitions
	classRegex := regexp.MustCompile(`^\s*(?:export\s+)?class\s+(\w+)`)
	interfaceRegex := regexp.MustCompile(`^\s*(?:export\s+)?interface\s+(\w+)`)
	funcRegex := regexp.MustCompile(`^\s*(?:export\s+)?function\s+(\w+)`)
	constFuncRegex := regexp.MustCompile(`^\s*(?:export\s+)?const\s+(\w+)\s*=\s*(?:\(.*\)|.*)\s*=>`)
	importRegex := regexp.MustCompile(`^\s*import.*from\s+['"]([^'"]+)['"]`)

	var classCount, ifaceCount, funcCount, importCount int

	for _, line := range lines {
		// Classes
		if matches := classRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("class:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "class", "public", path, line},
			})
			classCount++
		}

		// Interfaces
		if matches := interfaceRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("interface:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "interface", "public", path, line},
			})
			ifaceCount++
		}

		// Functions
		if matches := funcRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("func:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "function", "public", path, line},
			})
			funcCount++
		}

		// Const Functions (arrow functions)
		if matches := constFuncRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("func:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "function", "public", path, line},
			})
			funcCount++
		}

		// Imports
		if matches := importRegex.FindStringSubmatch(line); len(matches) > 1 {
			module := matches[1]
			facts = append(facts, core.Fact{
				Predicate: "dependency_link",
				Args:      []interface{}{path, fmt.Sprintf("mod:%s", module), module},
			})
			importCount++
		}
	}

	logging.WorldDebug("TS/JS regex parsing complete: %s (classes=%d, interfaces=%d, funcs=%d, imports=%d)",
		filepath.Base(path), classCount, ifaceCount, funcCount, importCount)
	return facts, nil
}

// Close releases resources held by the AST parser
func (p *ASTParser) Close() {
	logging.WorldDebug("Closing ASTParser")
	if p.tsParser != nil {
		p.tsParser.Close()
	}
}
