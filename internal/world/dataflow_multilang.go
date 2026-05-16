package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
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
// Python Data Flow Extraction
// =========================================================================

// extractPython extracts data flow facts from Python code using Tree-sitter.
// Detects: None checks, exception handling, variable assignments, uses.
func (m *MultiLangDataFlowExtractor) extractPython(path string) ([]core.Fact, error) {
	start := time.Now()

	content, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("MultiLang: failed to read Python file: %s - %v", path, err)
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	m.pythonParser.SetLanguage(python.GetLanguage())
	tree, err := m.pythonParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("MultiLang: Python parse failed: %s - %v", path, err)
		return nil, err
	}
	defer tree.Close()

	ctx := &pythonExtractionCtx{
		path:    path,
		content: string(content),
		facts:   []core.Fact{},
	}

	ctx.walk(tree.RootNode())

	logging.WorldDebug("MultiLangDataFlowExtractor: Python %s - %d facts in %v",
		filepath.Base(path), len(ctx.facts), time.Since(start))

	return ctx.facts, nil
}

type pythonExtractionCtx struct {
	path        string
	content     string
	facts       []core.Fact
	currentFunc string
	funcStart   int
	funcEnd     int
}

func (ctx *pythonExtractionCtx) getText(n *sitter.Node) string {
	return n.Content([]byte(ctx.content))
}

func (ctx *pythonExtractionCtx) emit(fact core.Fact) {
	ctx.facts = append(ctx.facts, fact)
}

func (ctx *pythonExtractionCtx) walk(n *sitter.Node) {
	if n == nil {
		return
	}

	nodeType := n.Type()

	switch nodeType {
	case "function_definition":
		ctx.extractPythonFunction(n)
		return // Handle children within extractPythonFunction

	case "assignment":
		ctx.extractPythonAssignment(n)

	case "if_statement":
		ctx.extractPythonIfGuard(n)

	case "try_statement":
		ctx.extractPythonTryExcept(n)

	case "attribute":
		ctx.extractPythonAttribute(n)

	case "call":
		ctx.extractPythonCall(n)
	}

	// Recurse into children
	for i := 0; i < int(n.ChildCount()); i++ {
		ctx.walk(n.Child(i))
	}
}

func (ctx *pythonExtractionCtx) extractPythonFunction(n *sitter.Node) {
	nameNode := n.ChildByFieldName("name")
	if nameNode == nil {
		return
	}

	oldFunc := ctx.currentFunc
	oldStart := ctx.funcStart
	oldEnd := ctx.funcEnd

	ctx.currentFunc = ctx.getText(nameNode)
	ctx.funcStart = int(n.StartPoint().Row) + 1
	ctx.funcEnd = int(n.EndPoint().Row) + 1

	// Emit function scope
	ctx.emit(core.Fact{
		Predicate: "function_scope",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			int64(ctx.funcStart),
			int64(ctx.funcEnd),
		},
	})

	// Walk children within function context
	body := n.ChildByFieldName("body")
	if body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			ctx.walk(body.Child(i))
		}
	}

	ctx.currentFunc = oldFunc
	ctx.funcStart = oldStart
	ctx.funcEnd = oldEnd
}

func (ctx *pythonExtractionCtx) extractPythonAssignment(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	// Get left side (variable)
	left := n.ChildByFieldName("left")
	if left == nil {
		return
	}

	varName := ""
	if left.Type() == "identifier" {
		varName = ctx.getText(left)
	}

	if varName == "" || varName == "_" {
		return
	}

	// Classify the assignment type from right side
	right := n.ChildByFieldName("right")
	typeClass := ctx.classifyPythonAssignment(right)

	if typeClass != "" {
		ctx.emit(core.Fact{
			Predicate: "assigns",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/" + typeClass),
				ctx.path,
				int64(line),
			},
		})
	}
}

func (ctx *pythonExtractionCtx) classifyPythonAssignment(n *sitter.Node) string {
	if n == nil {
		return ""
	}

	switch n.Type() {
	case "none":
		return "nullable"

	case "call":
		// Function calls may return None
		funcNode := n.ChildByFieldName("function")
		if funcNode != nil {
			funcName := ctx.getText(funcNode)
			// Common patterns that return Optional/None
			if strings.HasPrefix(funcName, "get") ||
				strings.HasPrefix(funcName, "find") ||
				strings.HasPrefix(funcName, "load") ||
				strings.HasPrefix(funcName, "read") ||
				strings.HasPrefix(funcName, "open") ||
				strings.HasPrefix(funcName, "parse") {
				return "nullable"
			}
		}
		return ""

	case "attribute":
		// Method calls that might return None
		attrNode := n.ChildByFieldName("attribute")
		if attrNode != nil {
			attrName := ctx.getText(attrNode)
			if attrName == "get" || attrName == "find" || attrName == "pop" {
				return "nullable"
			}
		}
		return ""
	}

	return ""
}

func (ctx *pythonExtractionCtx) extractPythonIfGuard(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1
	condNode := n.ChildByFieldName("condition")
	if condNode == nil {
		return
	}

	// Check for None comparison: `if x is None` or `if x is not None`
	isNoneCheck, varName, isNone := ctx.checkPythonNoneComparison(condNode)
	if !isNoneCheck {
		return
	}

	consequence := n.ChildByFieldName("consequence")
	if consequence == nil {
		return
	}

	blockStart := int(consequence.StartPoint().Row) + 1
	blockEnd := int(consequence.EndPoint().Row) + 1

	// Check for early return pattern
	hasReturn := ctx.hasEarlyReturn(consequence)

	if isNone && hasReturn {
		// if x is None: return ... (guard return pattern)
		ctx.emit(core.Fact{
			Predicate: "guards_return",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/none_check"),
				ctx.path,
				int64(line),
			},
		})
		// Emit dominance
		ctx.emit(core.Fact{
			Predicate: "guard_dominates",
			Args: []interface{}{
				ctx.path,
				core.MangleAtom("/" + ctx.currentFunc),
				int64(line),
				int64(ctx.funcEnd),
			},
		})
	} else if !isNone {
		// if x is not None: ... (block guard pattern)
		ctx.emit(core.Fact{
			Predicate: "guards_block",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/none_check"),
				ctx.path,
				int64(blockStart),
				int64(blockEnd),
			},
		})
	}
}

func (ctx *pythonExtractionCtx) checkPythonNoneComparison(n *sitter.Node) (isCheck bool, varName string, isNone bool) {
	if n == nil {
		return false, "", false
	}

	// Check for `x is None` or `x is not None`
	if n.Type() == "comparison_operator" {
		// Look for pattern: identifier "is" ["not"] "none"
		text := ctx.getText(n)

		if strings.Contains(text, " is not None") {
			// Extract variable name (first identifier)
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					return true, ctx.getText(child), false
				}
			}
		} else if strings.Contains(text, " is None") {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					return true, ctx.getText(child), true
				}
			}
		}
	}

	// Check for `x == None` or `x != None`
	if n.Type() == "comparison_operator" {
		text := ctx.getText(n)
		if strings.Contains(text, "== None") {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					return true, ctx.getText(child), true
				}
			}
		} else if strings.Contains(text, "!= None") {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					return true, ctx.getText(child), false
				}
			}
		}
	}

	return false, "", false
}

func (ctx *pythonExtractionCtx) hasEarlyReturn(block *sitter.Node) bool {
	if block == nil {
		return false
	}

	// Look for return or raise statements in the block
	for i := 0; i < int(block.ChildCount()); i++ {
		child := block.Child(i)
		if child.Type() == "return_statement" || child.Type() == "raise_statement" {
			return true
		}
	}
	return false
}

func (ctx *pythonExtractionCtx) extractPythonTryExcept(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	// try/except is Python's error handling
	body := n.ChildByFieldName("body")
	if body == nil {
		return
	}

	blockStart := int(body.StartPoint().Row) + 1
	blockEnd := int(body.EndPoint().Row) + 1

	// Emit error checked block for the try body
	ctx.emit(core.Fact{
		Predicate: "error_checked_block",
		Args: []interface{}{
			core.MangleAtom("/exception"),
			ctx.path,
			int64(blockStart),
			int64(blockEnd),
		},
	})
}

func (ctx *pythonExtractionCtx) extractPythonAttribute(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	// x.attr or x.method()
	obj := n.ChildByFieldName("object")
	if obj == nil {
		return
	}

	varName := ""
	if obj.Type() == "identifier" {
		varName = ctx.getText(obj)
	}

	if varName == "" || varName == "_" || varName == "self" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	ctx.emit(core.Fact{
		Predicate: "uses",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			core.MangleAtom("/" + varName),
			int64(line),
		},
	})
}

func (ctx *pythonExtractionCtx) extractPythonCall(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1
	funcNode := n.ChildByFieldName("function")
	if funcNode == nil {
		return
	}

	funcName := ""
	if funcNode.Type() == "identifier" {
		funcName = ctx.getText(funcNode)
	} else if funcNode.Type() == "attribute" {
		attr := funcNode.ChildByFieldName("attribute")
		if attr != nil {
			funcName = ctx.getText(attr)
		}
	}

	if funcName == "" {
		return
	}

	callsiteID := fmt.Sprintf("%s:%s:%d", ctx.currentFunc, funcName, line)

	// Extract arguments
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return
	}

	argPos := 0
	for i := 0; i < int(args.ChildCount()); i++ {
		arg := args.Child(i)
		if arg.Type() == "identifier" {
			varName := ctx.getText(arg)
			if varName != "" && varName != "_" {
				ctx.emit(core.Fact{
					Predicate: "call_arg",
					Args: []interface{}{
						core.MangleAtom("/" + callsiteID),
						int64(argPos),
						core.MangleAtom("/" + varName),
						ctx.path,
						int64(line),
					},
				})
			}
			argPos++
		}
	}
}

// =========================================================================
// JavaScript/TypeScript Data Flow Extraction
// =========================================================================

// extractJavaScript extracts data flow facts from JavaScript code.
func (m *MultiLangDataFlowExtractor) extractJavaScript(path string) ([]core.Fact, error) {
	start := time.Now()

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	m.jsParser.SetLanguage(javascript.GetLanguage())
	tree, err := m.jsParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	ctx := &jsExtractionCtx{
		path:    path,
		content: string(content),
		facts:   []core.Fact{},
	}

	ctx.walk(tree.RootNode())

	logging.WorldDebug("MultiLangDataFlowExtractor: JavaScript %s - %d facts in %v",
		filepath.Base(path), len(ctx.facts), time.Since(start))

	return ctx.facts, nil
}

// extractTypeScript extracts data flow facts from TypeScript code.
func (m *MultiLangDataFlowExtractor) extractTypeScript(path string) ([]core.Fact, error) {
	start := time.Now()

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	m.tsParser.SetLanguage(typescript.GetLanguage())
	tree, err := m.tsParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	ctx := &jsExtractionCtx{
		path:    path,
		content: string(content),
		facts:   []core.Fact{},
	}

	ctx.walk(tree.RootNode())

	logging.WorldDebug("MultiLangDataFlowExtractor: TypeScript %s - %d facts in %v",
		filepath.Base(path), len(ctx.facts), time.Since(start))

	return ctx.facts, nil
}

type jsExtractionCtx struct {
	path        string
	content     string
	facts       []core.Fact
	currentFunc string
	funcStart   int
	funcEnd     int
}

func (ctx *jsExtractionCtx) getText(n *sitter.Node) string {
	return n.Content([]byte(ctx.content))
}

func (ctx *jsExtractionCtx) emit(fact core.Fact) {
	ctx.facts = append(ctx.facts, fact)
}

func (ctx *jsExtractionCtx) walk(n *sitter.Node) {
	if n == nil {
		return
	}

	nodeType := n.Type()

	switch nodeType {
	case "function_declaration", "arrow_function", "method_definition":
		ctx.extractJSFunction(n)
		return

	case "variable_declaration", "lexical_declaration":
		ctx.extractJSVariableDecl(n)

	case "if_statement":
		ctx.extractJSIfGuard(n)

	case "try_statement":
		ctx.extractJSTryCatch(n)

	case "member_expression":
		ctx.extractJSMemberAccess(n)

	case "optional_chain_expression":
		ctx.extractJSOptionalChain(n)

	case "call_expression":
		ctx.extractJSCall(n)
	}

	for i := 0; i < int(n.ChildCount()); i++ {
		ctx.walk(n.Child(i))
	}
}

func (ctx *jsExtractionCtx) extractJSFunction(n *sitter.Node) {
	oldFunc := ctx.currentFunc
	oldStart := ctx.funcStart
	oldEnd := ctx.funcEnd

	// Get function name
	nameNode := n.ChildByFieldName("name")
	if nameNode != nil {
		ctx.currentFunc = ctx.getText(nameNode)
	} else {
		ctx.currentFunc = "anonymous"
	}

	ctx.funcStart = int(n.StartPoint().Row) + 1
	ctx.funcEnd = int(n.EndPoint().Row) + 1

	ctx.emit(core.Fact{
		Predicate: "function_scope",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			int64(ctx.funcStart),
			int64(ctx.funcEnd),
		},
	})

	// Walk body
	body := n.ChildByFieldName("body")
	if body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			ctx.walk(body.Child(i))
		}
	}

	ctx.currentFunc = oldFunc
	ctx.funcStart = oldStart
	ctx.funcEnd = oldEnd
}

func (ctx *jsExtractionCtx) extractJSVariableDecl(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	// Find variable declarators
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Type() == "variable_declarator" {
			nameNode := child.ChildByFieldName("name")
			valueNode := child.ChildByFieldName("value")

			if nameNode == nil {
				continue
			}

			varName := ctx.getText(nameNode)
			if varName == "" || varName == "_" {
				continue
			}

			typeClass := ctx.classifyJSAssignment(valueNode)
			if typeClass != "" {
				ctx.emit(core.Fact{
					Predicate: "assigns",
					Args: []interface{}{
						core.MangleAtom("/" + varName),
						core.MangleAtom("/" + typeClass),
						ctx.path,
						int64(line),
					},
				})
			}
		}
	}
}

func (ctx *jsExtractionCtx) classifyJSAssignment(n *sitter.Node) string {
	if n == nil {
		return ""
	}

	switch n.Type() {
	case "null", "undefined":
		return "nullable"

	case "call_expression":
		funcNode := n.ChildByFieldName("function")
		if funcNode != nil {
			funcName := ""
			if funcNode.Type() == "identifier" {
				funcName = ctx.getText(funcNode)
			} else if funcNode.Type() == "member_expression" {
				prop := funcNode.ChildByFieldName("property")
				if prop != nil {
					funcName = ctx.getText(prop)
				}
			}

			// Common patterns that return nullable
			if strings.HasPrefix(funcName, "get") ||
				strings.HasPrefix(funcName, "find") ||
				strings.HasPrefix(funcName, "fetch") ||
				funcName == "querySelector" ||
				funcName == "getElementById" {
				return "nullable"
			}
		}
	}

	return ""
}

func (ctx *jsExtractionCtx) extractJSIfGuard(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1
	condNode := n.ChildByFieldName("condition")
	if condNode == nil {
		return
	}

	isNullCheck, varName, isNull := ctx.checkJSNullComparison(condNode)
	if !isNullCheck {
		return
	}

	consequence := n.ChildByFieldName("consequence")
	if consequence == nil {
		return
	}

	blockStart := int(consequence.StartPoint().Row) + 1
	blockEnd := int(consequence.EndPoint().Row) + 1

	hasReturn := ctx.hasEarlyReturn(consequence)

	if isNull && hasReturn {
		// if (x === null) return ...
		ctx.emit(core.Fact{
			Predicate: "guards_return",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/null_check"),
				ctx.path,
				int64(line),
			},
		})
		ctx.emit(core.Fact{
			Predicate: "guard_dominates",
			Args: []interface{}{
				ctx.path,
				core.MangleAtom("/" + ctx.currentFunc),
				int64(line),
				int64(ctx.funcEnd),
			},
		})
	} else if !isNull {
		// if (x !== null) { ... }
		ctx.emit(core.Fact{
			Predicate: "guards_block",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/null_check"),
				ctx.path,
				int64(blockStart),
				int64(blockEnd),
			},
		})
	}
}

func (ctx *jsExtractionCtx) checkJSNullComparison(n *sitter.Node) (isCheck bool, varName string, isNull bool) {
	if n == nil {
		return false, "", false
	}

	// Handle parenthesized expressions
	if n.Type() == "parenthesized_expression" {
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() != "(" && child.Type() != ")" {
				return ctx.checkJSNullComparison(child)
			}
		}
	}

	// Check for binary expression: x === null, x !== null, x == null, x != null
	// Also: x === undefined, x !== undefined
	if n.Type() == "binary_expression" {
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		op := n.ChildByFieldName("operator")

		if left == nil || right == nil || op == nil {
			return false, "", false
		}

		opText := ctx.getText(op)
		isEqual := opText == "===" || opText == "=="
		isNotEqual := opText == "!==" || opText == "!="

		if !isEqual && !isNotEqual {
			return false, "", false
		}

		// Check if comparing to null or undefined
		rightText := ctx.getText(right)
		isNullOrUndefined := rightText == "null" || rightText == "undefined"

		if !isNullOrUndefined {
			// Maybe left is null
			leftText := ctx.getText(left)
			if leftText == "null" || leftText == "undefined" {
				if right.Type() == "identifier" {
					return true, ctx.getText(right), isEqual
				}
			}
			return false, "", false
		}

		if left.Type() == "identifier" {
			return true, ctx.getText(left), isEqual
		}
	}

	// Check for unary negation: !x (truthy check, implies non-null when entering else)
	if n.Type() == "unary_expression" {
		opNode := n.ChildByFieldName("operator")
		argNode := n.ChildByFieldName("argument")
		if opNode != nil && argNode != nil {
			if ctx.getText(opNode) == "!" && argNode.Type() == "identifier" {
				// !x means "if x is falsy" - entering here means x is null/undefined/0/""
				return true, ctx.getText(argNode), true
			}
		}
	}

	// Truthy check: if (x) - identifier alone
	if n.Type() == "identifier" {
		// This is a truthy check - entering means non-null
		return true, ctx.getText(n), false
	}

	return false, "", false
}

func (ctx *jsExtractionCtx) hasEarlyReturn(block *sitter.Node) bool {
	if block == nil {
		return false
	}

	for i := 0; i < int(block.ChildCount()); i++ {
		child := block.Child(i)
		if child.Type() == "return_statement" || child.Type() == "throw_statement" {
			return true
		}
	}
	return false
}

func (ctx *jsExtractionCtx) extractJSTryCatch(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	body := n.ChildByFieldName("body")
	if body == nil {
		return
	}

	blockStart := int(body.StartPoint().Row) + 1
	blockEnd := int(body.EndPoint().Row) + 1

	ctx.emit(core.Fact{
		Predicate: "error_checked_block",
		Args: []interface{}{
			core.MangleAtom("/exception"),
			ctx.path,
			int64(blockStart),
			int64(blockEnd),
		},
	})
}

func (ctx *jsExtractionCtx) extractJSMemberAccess(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	obj := n.ChildByFieldName("object")
	if obj == nil {
		return
	}

	varName := ""
	if obj.Type() == "identifier" {
		varName = ctx.getText(obj)
	}

	if varName == "" || varName == "_" || varName == "this" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	ctx.emit(core.Fact{
		Predicate: "uses",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			core.MangleAtom("/" + varName),
			int64(line),
		},
	})
}

func (ctx *jsExtractionCtx) extractJSOptionalChain(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	// Optional chaining (x?.foo) is safe - it's implicitly guarded
	// We can emit a fact that this access is safe
	line := int(n.StartPoint().Row) + 1

	// Find the base variable
	var findBase func(*sitter.Node) string
	findBase = func(node *sitter.Node) string {
		if node == nil {
			return ""
		}
		if node.Type() == "identifier" {
			return ctx.getText(node)
		}
		// Recurse into left side
		obj := node.ChildByFieldName("object")
		if obj != nil {
			return findBase(obj)
		}
		if node.ChildCount() > 0 {
			return findBase(node.Child(0))
		}
		return ""
	}

	varName := findBase(n)
	if varName != "" && varName != "this" {
		// Emit as a guarded use (optional chaining is safe by design)
		ctx.emit(core.Fact{
			Predicate: "safe_access",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/optional_chain"),
				ctx.path,
				int64(line),
			},
		})
	}
}

func (ctx *jsExtractionCtx) extractJSCall(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1
	funcNode := n.ChildByFieldName("function")
	if funcNode == nil {
		return
	}

	funcName := ""
	if funcNode.Type() == "identifier" {
		funcName = ctx.getText(funcNode)
	} else if funcNode.Type() == "member_expression" {
		prop := funcNode.ChildByFieldName("property")
		if prop != nil {
			funcName = ctx.getText(prop)
		}
	}

	if funcName == "" {
		return
	}

	callsiteID := fmt.Sprintf("%s:%s:%d", ctx.currentFunc, funcName, line)

	args := n.ChildByFieldName("arguments")
	if args == nil {
		return
	}

	argPos := 0
	for i := 0; i < int(args.ChildCount()); i++ {
		arg := args.Child(i)
		if arg.Type() == "identifier" {
			varName := ctx.getText(arg)
			if varName != "" && varName != "_" {
				ctx.emit(core.Fact{
					Predicate: "call_arg",
					Args: []interface{}{
						core.MangleAtom("/" + callsiteID),
						int64(argPos),
						core.MangleAtom("/" + varName),
						ctx.path,
						int64(line),
					},
				})
			}
			argPos++
		}
	}
}

// =========================================================================
// Rust Data Flow Extraction
// =========================================================================

// extractRust extracts data flow facts from Rust code.
// Detects: Option/Result types, match patterns, ? operator, uses.
func (m *MultiLangDataFlowExtractor) extractRust(path string) ([]core.Fact, error) {
	start := time.Now()

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	m.rustParser.SetLanguage(rust.GetLanguage())
	tree, err := m.rustParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	ctx := &rustExtractionCtx{
		path:    path,
		content: string(content),
		facts:   []core.Fact{},
	}

	ctx.walk(tree.RootNode())

	logging.WorldDebug("MultiLangDataFlowExtractor: Rust %s - %d facts in %v",
		filepath.Base(path), len(ctx.facts), time.Since(start))

	return ctx.facts, nil
}

type rustExtractionCtx struct {
	path        string
	content     string
	facts       []core.Fact
	currentFunc string
	funcStart   int
	funcEnd     int
}

func (ctx *rustExtractionCtx) getText(n *sitter.Node) string {
	return n.Content([]byte(ctx.content))
}

func (ctx *rustExtractionCtx) emit(fact core.Fact) {
	ctx.facts = append(ctx.facts, fact)
}

func (ctx *rustExtractionCtx) walk(n *sitter.Node) {
	if n == nil {
		return
	}

	nodeType := n.Type()

	switch nodeType {
	case "function_item":
		ctx.extractRustFunction(n)
		return

	case "let_declaration":
		ctx.extractRustLetBinding(n)

	case "if_expression":
		ctx.extractRustIfGuard(n)

	case "if_let_expression":
		ctx.extractRustIfLet(n)

	case "match_expression":
		ctx.extractRustMatch(n)

	case "try_expression":
		ctx.extractRustTryOperator(n)

	case "field_expression":
		ctx.extractRustFieldAccess(n)

	case "call_expression", "method_call_expression":
		ctx.extractRustCall(n)
	}

	for i := 0; i < int(n.ChildCount()); i++ {
		ctx.walk(n.Child(i))
	}
}

func (ctx *rustExtractionCtx) extractRustFunction(n *sitter.Node) {
	oldFunc := ctx.currentFunc
	oldStart := ctx.funcStart
	oldEnd := ctx.funcEnd

	nameNode := n.ChildByFieldName("name")
	if nameNode != nil {
		ctx.currentFunc = ctx.getText(nameNode)
	}

	ctx.funcStart = int(n.StartPoint().Row) + 1
	ctx.funcEnd = int(n.EndPoint().Row) + 1

	ctx.emit(core.Fact{
		Predicate: "function_scope",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			int64(ctx.funcStart),
			int64(ctx.funcEnd),
		},
	})

	// Walk body
	body := n.ChildByFieldName("body")
	if body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			ctx.walk(body.Child(i))
		}
	}

	ctx.currentFunc = oldFunc
	ctx.funcStart = oldStart
	ctx.funcEnd = oldEnd
}

func (ctx *rustExtractionCtx) extractRustLetBinding(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	patternNode := n.ChildByFieldName("pattern")
	valueNode := n.ChildByFieldName("value")

	if patternNode == nil {
		return
	}

	varName := ""
	if patternNode.Type() == "identifier" {
		varName = ctx.getText(patternNode)
	}

	if varName == "" || varName == "_" {
		return
	}

	typeClass := ctx.classifyRustAssignment(n, valueNode)
	if typeClass != "" {
		ctx.emit(core.Fact{
			Predicate: "assigns",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/" + typeClass),
				ctx.path,
				int64(line),
			},
		})
	}
}

func (ctx *rustExtractionCtx) classifyRustAssignment(letDecl, value *sitter.Node) string {
	// Check type annotation for Option or Result
	typeNode := letDecl.ChildByFieldName("type")
	if typeNode != nil {
		typeText := ctx.getText(typeNode)
		if strings.HasPrefix(typeText, "Option") {
			return "option"
		}
		if strings.HasPrefix(typeText, "Result") {
			return "result"
		}
	}

	// Check value for Option/Result construction
	if value != nil {
		valueText := ctx.getText(value)
		if strings.HasPrefix(valueText, "Some(") || valueText == "None" {
			return "option"
		}
		if strings.HasPrefix(valueText, "Ok(") || strings.HasPrefix(valueText, "Err(") {
			return "result"
		}
	}

	return ""
}

func (ctx *rustExtractionCtx) extractRustIfGuard(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1
	condNode := n.ChildByFieldName("condition")
	if condNode == nil {
		return
	}

	// Check for .is_some(), .is_none(), .is_ok(), .is_err() patterns
	condText := ctx.getText(condNode)

	var varName string
	var isNone bool

	if strings.Contains(condText, ".is_some()") {
		// Extract variable name before .is_some()
		varName = strings.TrimSuffix(strings.TrimSpace(condText), ".is_some()")
		isNone = false
	} else if strings.Contains(condText, ".is_none()") {
		varName = strings.TrimSuffix(strings.TrimSpace(condText), ".is_none()")
		isNone = true
	} else if strings.Contains(condText, ".is_ok()") {
		varName = strings.TrimSuffix(strings.TrimSpace(condText), ".is_ok()")
		isNone = false
	} else if strings.Contains(condText, ".is_err()") {
		varName = strings.TrimSuffix(strings.TrimSpace(condText), ".is_err()")
		isNone = true
	} else {
		return
	}

	consequence := n.ChildByFieldName("consequence")
	if consequence == nil {
		return
	}

	blockStart := int(consequence.StartPoint().Row) + 1
	blockEnd := int(consequence.EndPoint().Row) + 1

	hasReturn := ctx.hasEarlyReturn(consequence)

	if isNone && hasReturn {
		ctx.emit(core.Fact{
			Predicate: "guards_return",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/option_check"),
				ctx.path,
				int64(line),
			},
		})
		ctx.emit(core.Fact{
			Predicate: "guard_dominates",
			Args: []interface{}{
				ctx.path,
				core.MangleAtom("/" + ctx.currentFunc),
				int64(line),
				int64(ctx.funcEnd),
			},
		})
	} else if !isNone {
		ctx.emit(core.Fact{
			Predicate: "guards_block",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/option_check"),
				ctx.path,
				int64(blockStart),
				int64(blockEnd),
			},
		})
	}
}

func (ctx *rustExtractionCtx) extractRustIfLet(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	// if let Some(x) = y { ... } or if let Ok(x) = y { ... }
	// This is a guard pattern that extracts the value
	patternNode := n.ChildByFieldName("pattern")
	valueNode := n.ChildByFieldName("value")

	if patternNode == nil || valueNode == nil {
		return
	}

	patternText := ctx.getText(patternNode)
	valueText := ctx.getText(valueNode)

	checkType := "/option_check"
	if strings.HasPrefix(patternText, "Ok(") || strings.HasPrefix(patternText, "Err(") {
		checkType = "/result_check"
	}

	consequence := n.ChildByFieldName("consequence")
	if consequence == nil {
		return
	}

	blockStart := int(consequence.StartPoint().Row) + 1
	blockEnd := int(consequence.EndPoint().Row) + 1

	// if let Some(x) = y is a guard block that protects x
	ctx.emit(core.Fact{
		Predicate: "guards_block",
		Args: []interface{}{
			core.MangleAtom("/" + valueText),
			core.MangleAtom(checkType),
			ctx.path,
			int64(blockStart),
			int64(blockEnd),
		},
	})

	// Also emit as safe extraction pattern
	ctx.emit(core.Fact{
		Predicate: "safe_access",
		Args: []interface{}{
			core.MangleAtom("/" + valueText),
			core.MangleAtom("/if_let"),
			ctx.path,
			int64(line),
		},
	})
}

func (ctx *rustExtractionCtx) extractRustMatch(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	valueNode := n.ChildByFieldName("value")
	if valueNode == nil {
		return
	}

	varName := ""
	if valueNode.Type() == "identifier" {
		varName = ctx.getText(valueNode)
	}

	if varName == "" {
		return
	}

	// match expressions exhaustively handle all cases - it's safe
	ctx.emit(core.Fact{
		Predicate: "safe_access",
		Args: []interface{}{
			core.MangleAtom("/" + varName),
			core.MangleAtom("/match_exhaustive"),
			ctx.path,
			int64(line),
		},
	})
}

func (ctx *rustExtractionCtx) extractRustTryOperator(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	// The ? operator propagates errors - marks error as checked via early return
	inner := n.ChildByFieldName("inner")
	if inner == nil && n.ChildCount() > 0 {
		inner = n.Child(0)
	}

	if inner == nil {
		return
	}

	varName := ""
	if inner.Type() == "identifier" {
		varName = ctx.getText(inner)
	} else if inner.Type() == "call_expression" || inner.Type() == "method_call_expression" {
		// For call?.method() patterns, extract the call
		varName = ctx.getText(inner)
		// Truncate for readability
		if len(varName) > 30 {
			varName = varName[:30] + "..."
		}
	}

	if varName != "" {
		// ? operator is error checked via propagation
		ctx.emit(core.Fact{
			Predicate: "error_checked_return",
			Args: []interface{}{
				core.MangleAtom("/" + varName),
				ctx.path,
				int64(line),
			},
		})
	}
}

func (ctx *rustExtractionCtx) hasEarlyReturn(block *sitter.Node) bool {
	if block == nil {
		return false
	}

	for i := 0; i < int(block.ChildCount()); i++ {
		child := block.Child(i)
		nodeType := child.Type()
		if nodeType == "return_expression" || nodeType == "macro_invocation" {
			// Check for panic!, unreachable!, etc.
			if nodeType == "macro_invocation" {
				text := ctx.getText(child)
				if strings.HasPrefix(text, "panic!") ||
					strings.HasPrefix(text, "unreachable!") ||
					strings.HasPrefix(text, "return") {
					return true
				}
			} else {
				return true
			}
		}
	}
	return false
}

func (ctx *rustExtractionCtx) extractRustFieldAccess(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	valueNode := n.ChildByFieldName("value")
	if valueNode == nil {
		return
	}

	varName := ""
	if valueNode.Type() == "identifier" {
		varName = ctx.getText(valueNode)
	}

	if varName == "" || varName == "_" || varName == "self" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	ctx.emit(core.Fact{
		Predicate: "uses",
		Args: []interface{}{
			ctx.path,
			core.MangleAtom("/" + ctx.currentFunc),
			core.MangleAtom("/" + varName),
			int64(line),
		},
	})
}

func (ctx *rustExtractionCtx) extractRustCall(n *sitter.Node) {
	if ctx.currentFunc == "" {
		return
	}

	line := int(n.StartPoint().Row) + 1

	funcName := ""
	if n.Type() == "call_expression" {
		funcNode := n.ChildByFieldName("function")
		if funcNode != nil {
			if funcNode.Type() == "identifier" {
				funcName = ctx.getText(funcNode)
			}
		}
	} else if n.Type() == "method_call_expression" {
		nameNode := n.ChildByFieldName("name")
		if nameNode != nil {
			funcName = ctx.getText(nameNode)
		}
	}

	if funcName == "" {
		return
	}

	callsiteID := fmt.Sprintf("%s:%s:%d", ctx.currentFunc, funcName, line)

	args := n.ChildByFieldName("arguments")
	if args == nil {
		return
	}

	argPos := 0
	for i := 0; i < int(args.ChildCount()); i++ {
		arg := args.Child(i)
		if arg.Type() == "identifier" {
			varName := ctx.getText(arg)
			if varName != "" && varName != "_" {
				ctx.emit(core.Fact{
					Predicate: "call_arg",
					Args: []interface{}{
						core.MangleAtom("/" + callsiteID),
						int64(argPos),
						core.MangleAtom("/" + varName),
						ctx.path,
						int64(line),
					},
				})
			}
			argPos++
		}
	}
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
