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

// MultiLangDataFlowSummary provides aggregated statistics across languages.
type MultiLangDataFlowSummary struct {
	TotalFacts       int
	ByLanguage       map[string]int
	AssignmentsFacts int
	NullableFacts    int
	OptionFacts      int
	ResultFacts      int
	ErrorFacts       int
	GuardBlockFacts  int
	GuardReturnFacts int
	SafeAccessFacts  int
	UsesFacts        int
	CallArgFacts     int
}

// SummarizeMultiLangDataFlow analyzes extracted facts from multiple languages.
func SummarizeMultiLangDataFlow(facts []core.Fact) MultiLangDataFlowSummary {
	summary := MultiLangDataFlowSummary{
		TotalFacts: len(facts),
		ByLanguage: make(map[string]int),
	}

	for _, fact := range facts {
		// Track by file extension
		if len(fact.Args) >= 3 {
			if path, ok := fact.Args[2].(string); ok {
				lang := DetectLanguage(path)
				if lang != "" {
					summary.ByLanguage[lang]++
				}
			}
		}

		switch fact.Predicate {
		case "assigns":
			summary.AssignmentsFacts++
			if len(fact.Args) >= 2 {
				if typeClass, ok := fact.Args[1].(core.MangleAtom); ok {
					switch string(typeClass) {
					case "/nullable":
						summary.NullableFacts++
					case "/option":
						summary.OptionFacts++
					case "/result":
						summary.ResultFacts++
					case "/error":
						summary.ErrorFacts++
					}
				}
			}
		case "guards_block":
			summary.GuardBlockFacts++
		case "guards_return":
			summary.GuardReturnFacts++
		case "safe_access":
			summary.SafeAccessFacts++
		case "uses":
			summary.UsesFacts++
		case "call_arg":
			summary.CallArgFacts++
		}
	}

	return summary
}
