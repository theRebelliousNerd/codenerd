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
	"github.com/smacker/go-tree-sitter/rust"
)

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
			Args: []any{
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
			Args: []any{
				core.MangleAtom("/" + varName),
				core.MangleAtom("/option_check"),
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
	} else if !isNone {
		ctx.emit(core.Fact{
			Predicate: "guards_block",
			Args: []any{
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
		Args: []any{
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
		Args: []any{
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
		Args: []any{
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
			Args: []any{
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
		Args: []any{
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
