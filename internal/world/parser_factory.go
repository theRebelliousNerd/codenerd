package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// ParserFactory manages language-specific CodeParsers and routes
// parse requests to the appropriate parser based on file extension.
//
// This factory enables the polyglot CodeDOM by:
//   1. Registering parsers for specific file extensions
//   2. Routing parse requests to the correct parser
//   3. Aggregating language facts from all parsers
//   4. Providing a unified interface for the rest of the system
type ParserFactory struct {
	mu          sync.RWMutex
	parsers     map[string]CodeParser // extension -> parser (e.g., ".go" -> GoCodeParser)
	projectRoot string                // For generating repo-anchored refs
}

// NewParserFactory creates a new ParserFactory with the given project root.
// The project root is used to generate repo-anchored Ref URIs.
func NewParserFactory(projectRoot string) *ParserFactory {
	logging.WorldDebug("Creating ParserFactory with project root: %s", projectRoot)
	return &ParserFactory{
		parsers:     make(map[string]CodeParser),
		projectRoot: projectRoot,
	}
}

// Register adds a parser for its supported extensions.
// If a parser is already registered for an extension, it is replaced.
func (f *ParserFactory) Register(parser CodeParser) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, ext := range parser.SupportedExtensions() {
		ext = normalizeExtension(ext)
		logging.WorldDebug("ParserFactory: registering %s parser for extension %s",
			parser.Language(), ext)
		f.parsers[ext] = parser
	}
}

// GetParser returns the parser for a given file path.
// Returns nil if no parser is registered for the file's extension.
func (f *ParserFactory) GetParser(path string) CodeParser {
	f.mu.RLock()
	defer f.mu.RUnlock()

	ext := normalizeExtension(filepath.Ext(path))
	return f.parsers[ext]
}

// HasParser returns true if a parser exists for the given file path.
func (f *ParserFactory) HasParser(path string) bool {
	return f.GetParser(path) != nil
}

// Parse extracts CodeElements from a file using the appropriate parser.
// Returns an error if no parser is registered for the file's extension.
func (f *ParserFactory) Parse(path string, content []byte) ([]CodeElement, error) {
	parser := f.GetParser(path)
	if parser == nil {
		ext := filepath.Ext(path)
		return nil, fmt.Errorf("no parser registered for extension: %s", ext)
	}
	return parser.Parse(path, content)
}

// ParseWithFacts parses a file and returns both elements and language facts.
// This is the preferred method for full CodeDOM integration.
func (f *ParserFactory) ParseWithFacts(path string, content []byte) (*ParseResult, error) {
	parser := f.GetParser(path)
	if parser == nil {
		ext := filepath.Ext(path)
		return nil, fmt.Errorf("no parser registered for extension: %s", ext)
	}

	elements, err := parser.Parse(path, content)
	if err != nil {
		return nil, err
	}

	// Generate language-specific facts
	langFacts := parser.EmitLanguageFacts(elements)

	// Detect code patterns
	patterns := DetectCodePatterns(string(content), elements)

	return &ParseResult{
		Elements:      elements,
		LanguageFacts: langFacts,
		Patterns:      patterns,
	}, nil
}

// EmitAllFacts returns all Mangle facts for a parsed file:
// - Standard code_element facts from CodeElement.ToFacts()
// - Language-specific Stratum 0 facts from EmitLanguageFacts()
// - Code pattern facts from DetectCodePatterns()
func (f *ParserFactory) EmitAllFacts(result *ParseResult, file string) []core.Fact {
	var facts []core.Fact

	// Add standard CodeElement facts
	for _, elem := range result.Elements {
		facts = append(facts, elem.ToFacts()...)
	}

	// Add language-specific Stratum 0 facts
	facts = append(facts, result.LanguageFacts...)

	// Add code pattern facts
	facts = append(facts, result.Patterns.ToPatternFacts(file, result.Elements)...)

	return facts
}

// SupportedExtensions returns all registered file extensions.
func (f *ParserFactory) SupportedExtensions() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	exts := make([]string, 0, len(f.parsers))
	for ext := range f.parsers {
		exts = append(exts, ext)
	}
	return exts
}

// RegisteredLanguages returns all registered language identifiers.
func (f *ParserFactory) RegisteredLanguages() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	seen := make(map[string]bool)
	var langs []string
	for _, parser := range f.parsers {
		lang := parser.Language()
		if !seen[lang] {
			seen[lang] = true
			langs = append(langs, lang)
		}
	}
	return langs
}

// ProjectRoot returns the project root used for repo-anchored refs.
func (f *ParserFactory) ProjectRoot() string {
	return f.projectRoot
}

// RelativePath returns the path relative to the project root.
// This is used for generating repo-anchored Ref URIs.
func (f *ParserFactory) RelativePath(absPath string) string {
	rel, err := filepath.Rel(f.projectRoot, absPath)
	if err != nil {
		return absPath
	}
	// Normalize to forward slashes for cross-platform consistency
	return filepath.ToSlash(rel)
}

// normalizeExtension ensures extensions are lowercase with leading dot.
func normalizeExtension(ext string) string {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

// DefaultParserFactory creates a ParserFactory with all built-in parsers registered.
// This is the recommended way to create a fully-functional factory.
func DefaultParserFactory(projectRoot string) *ParserFactory {
	factory := NewParserFactory(projectRoot)

	// Register Go parser (always available)
	factory.Register(NewGoCodeParser(projectRoot))

	// Register Mangle parser
	factory.Register(NewMangleCodeParser(projectRoot))

	// Register Python parser (Tree-sitter based)
	factory.Register(NewPythonCodeParser(projectRoot))

	// Register TypeScript/JavaScript parser (Tree-sitter based)
	factory.Register(NewTypeScriptCodeParser(projectRoot))

	// Register Rust parser (Tree-sitter based)
	factory.Register(NewRustCodeParser(projectRoot))

	logging.WorldDebug("DefaultParserFactory: registered %d parsers for %v",
		len(factory.parsers), factory.SupportedExtensions())

	return factory
}
