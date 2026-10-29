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
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

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
		Args: []any{
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
					Args: []any{
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
			Args: []any{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/null_check"),
				ctx.path,
				int64(line),
			},
		})
		ctx.emit(core.Fact{
			Predicate: "guard_dominates",
			Args: []any{
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
			Args: []any{
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
		Args: []any{
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
		Args: []any{
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
			Args: []any{
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
