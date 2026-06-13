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
	"github.com/smacker/go-tree-sitter/python"
)

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
		Args: []any{
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
			Args: []any{
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
			Args: []any{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/none_check"),
				ctx.path,
				int64(line),
			},
		})
		// Emit dominance
		ctx.emit(core.Fact{
			Predicate: "guard_dominates",
			Args: []any{
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
			Args: []any{
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
		Args: []any{
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
		Args: []any{
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
					Args: []any{
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
