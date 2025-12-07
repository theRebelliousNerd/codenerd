package world

import (
	"codenerd/internal/core"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ASTParser handles code parsing.
type ASTParser struct {
	tsParser *TreeSitterParser
}

func NewASTParser() *ASTParser {
	return &ASTParser{
		tsParser: NewTreeSitterParser(),
	}
}

func (p *ASTParser) Parse(path string) ([]core.Fact, error) {
	if strings.HasSuffix(path, ".go") {
		return p.parseGo(path)
	} else if strings.HasSuffix(path, ".py") {
		return p.parsePython(path)
	} else if strings.HasSuffix(path, ".rs") {
		return p.parseRust(path)
	} else if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".jsx") {
		return p.parseTypeScript(path)
	}
	return nil, nil
}

func (p *ASTParser) parseGo(path string) ([]core.Fact, error) {
	// Delegate to Cartographer for holographic mapping
	c := NewCartographer()
	return c.MapFile(path)
}

// parsePython implements tree-sitter-based parsing for Python with regex fallback
func (p *ASTParser) parsePython(path string) ([]core.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try tree-sitter parsing first
	if p.tsParser != nil {
		facts, err := p.tsParser.ParsePython(path, content)
		if err == nil && len(facts) > 0 {
			return facts, nil
		}
		// If tree-sitter fails or returns no facts, fall back to regex
	}

	// Fallback: regex-based parsing
	var facts []core.Fact
	lines := strings.Split(string(content), "\n")

	// Regex for definitions
	classRegex := regexp.MustCompile(`^\s*class\s+(\w+)`)
	defRegex := regexp.MustCompile(`^\s*def\s+(\w+)`)
	importRegex := regexp.MustCompile(`^\s*(?:from|import)\s+(\w+)`)

	for _, line := range lines {
		// Classes
		if matches := classRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("class:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "class", "public", path, line},
			})
		}

		// Functions
		if matches := defRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("func:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "function", "public", path, line},
			})
		}

		// Imports
		if matches := importRegex.FindStringSubmatch(line); len(matches) > 1 {
			module := matches[1]
			facts = append(facts, core.Fact{
				Predicate: "dependency_link",
				Args:      []interface{}{path, fmt.Sprintf("mod:%s", module), module},
			})
		}
	}

	return facts, nil
}

// parseRust implements tree-sitter-based parsing for Rust with regex fallback
func (p *ASTParser) parseRust(path string) ([]core.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try tree-sitter parsing first
	if p.tsParser != nil {
		facts, err := p.tsParser.ParseRust(path, content)
		if err == nil && len(facts) > 0 {
			return facts, nil
		}
		// If tree-sitter fails or returns no facts, fall back to regex
	}

	// Fallback: regex-based parsing
	var facts []core.Fact
	lines := strings.Split(string(content), "\n")

	// Regex for definitions
	fnRegex := regexp.MustCompile(`^\s*(?:pub\s+)?fn\s+(\w+)`)
	structRegex := regexp.MustCompile(`^\s*(?:pub\s+)?struct\s+(\w+)`)
	enumRegex := regexp.MustCompile(`^\s*(?:pub\s+)?enum\s+(\w+)`)
	modRegex := regexp.MustCompile(`^\s*(?:pub\s+)?mod\s+(\w+)`)
	useRegex := regexp.MustCompile(`^\s*use\s+([\w:]+)`)

	for _, line := range lines {
		// Functions
		if matches := fnRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("fn:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "function", "public", path, line},
			})
		}

		// Structs
		if matches := structRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("struct:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "struct", "public", path, line},
			})
		}

		// Enums
		if matches := enumRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("enum:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "enum", "public", path, line},
			})
		}

		// Modules
		if matches := modRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("mod:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "module", "public", path, line},
			})
		}

		// Imports (use)
		if matches := useRegex.FindStringSubmatch(line); len(matches) > 1 {
			pkg := matches[1]
			facts = append(facts, core.Fact{
				Predicate: "dependency_link",
				Args:      []interface{}{path, fmt.Sprintf("crate:%s", pkg), pkg},
			})
		}
	}

	return facts, nil
}

// parseTypeScript implements tree-sitter-based parsing for TS/JS with regex fallback
func (p *ASTParser) parseTypeScript(path string) ([]core.Fact, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try tree-sitter parsing first
	if p.tsParser != nil {
		var facts []core.Fact
		var parseErr error

		// Determine if TypeScript or JavaScript
		if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
			facts, parseErr = p.tsParser.ParseTypeScript(path, content)
		} else {
			facts, parseErr = p.tsParser.ParseJavaScript(path, content)
		}

		if parseErr == nil && len(facts) > 0 {
			return facts, nil
		}
		// If tree-sitter fails or returns no facts, fall back to regex
	}

	// Fallback: regex-based parsing
	var facts []core.Fact
	lines := strings.Split(string(content), "\n")

	// Regex for definitions
	classRegex := regexp.MustCompile(`^\s*(?:export\s+)?class\s+(\w+)`)
	interfaceRegex := regexp.MustCompile(`^\s*(?:export\s+)?interface\s+(\w+)`)
	funcRegex := regexp.MustCompile(`^\s*(?:export\s+)?function\s+(\w+)`)
	constFuncRegex := regexp.MustCompile(`^\s*(?:export\s+)?const\s+(\w+)\s*=\s*(?:\(.*\)|.*)\s*=>`)
	importRegex := regexp.MustCompile(`^\s*import.*from\s+['"]([^'"]+)['"]`)

	for _, line := range lines {
		// Classes
		if matches := classRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("class:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "class", "public", path, line},
			})
		}

		// Interfaces
		if matches := interfaceRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("interface:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "interface", "public", path, line},
			})
		}

		// Functions
		if matches := funcRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("func:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "function", "public", path, line},
			})
		}

		// Const Functions (arrow functions)
		if matches := constFuncRegex.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			id := fmt.Sprintf("func:%s", name)
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{id, "function", "public", path, line},
			})
		}

		// Imports
		if matches := importRegex.FindStringSubmatch(line); len(matches) > 1 {
			module := matches[1]
			facts = append(facts, core.Fact{
				Predicate: "dependency_link",
				Args:      []interface{}{path, fmt.Sprintf("mod:%s", module), module},
			})
		}
	}

	return facts, nil
}

// Close releases resources held by the AST parser
func (p *ASTParser) Close() {
	if p.tsParser != nil {
		p.tsParser.Close()
	}
}
