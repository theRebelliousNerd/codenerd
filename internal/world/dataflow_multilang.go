package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"path/filepath"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
)

// MultiLangDataFlowExtractor extends data flow extraction to Python, TypeScript,
// JavaScript, and Rust using Tree-sitter for AST parsing.
type MultiLangDataFlowExtractor struct {
	pythonParser *sitter.Parser
	jsParser     *sitter.Parser
	tsParser     *sitter.Parser
	rustParser   *sitter.Parser
	goExtractor  *DataFlowExtractor // Delegate to Go's native AST parser
}

// NewMultiLangDataFlowExtractor creates a new multi-language data flow extractor.
func NewMultiLangDataFlowExtractor() *MultiLangDataFlowExtractor {
	logging.WorldDebug("Creating new MultiLangDataFlowExtractor")
	return &MultiLangDataFlowExtractor{
		pythonParser: sitter.NewParser(),
		jsParser:     sitter.NewParser(),
		tsParser:     sitter.NewParser(),
		rustParser:   sitter.NewParser(),
		goExtractor:  NewDataFlowExtractor(),
	}
}

// Close releases resources held by the parsers.
func (m *MultiLangDataFlowExtractor) Close() {
	logging.WorldDebug("Closing MultiLangDataFlowExtractor")
	m.pythonParser.Close()
	m.jsParser.Close()
	m.tsParser.Close()
	m.rustParser.Close()
}

// DetectLanguage determines the programming language from file extension.
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	default:
		return ""
	}
}

// ExtractDataFlow extracts data flow facts from a file based on its language.
func (m *MultiLangDataFlowExtractor) ExtractDataFlow(path string) ([]core.Fact, error) {
	lang := DetectLanguage(path)
	return m.ExtractDataFlowForLanguage(path, lang)
}

// ExtractDataFlowForLanguage extracts data flow facts using the appropriate parser.
func (m *MultiLangDataFlowExtractor) ExtractDataFlowForLanguage(path string, lang string) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("MultiLangDataFlowExtractor: analyzing %s file: %s", lang, filepath.Base(path))

	var facts []core.Fact
	var err error

	switch lang {
	case "go":
		facts, err = m.goExtractor.ExtractDataFlow(path)
	case "python":
		facts, err = m.extractPython(path)
	case "javascript":
		facts, err = m.extractJavaScript(path)
	case "typescript":
		facts, err = m.extractTypeScript(path)
	case "rust":
		facts, err = m.extractRust(path)
	default:
		logging.WorldDebug("MultiLangDataFlowExtractor: unsupported language for %s", filepath.Base(path))
		return nil, nil
	}

	logging.WorldDebug("MultiLangDataFlowExtractor: analyzed %s in %v", filepath.Base(path), time.Since(start))
	return facts, err
}

// =========================================================================
// Multi-Language Summary
// =========================================================================
