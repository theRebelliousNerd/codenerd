package world

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// =============================================================================
// FORMATTING HELPERS
// =============================================================================

// formatNode formats an AST node as a string.
func formatNode(fset *token.FileSet, node ast.Node) string {
	if node == nil {
		return ""
	}

	var sb strings.Builder
	switch n := node.(type) {
	case *ast.Ident:
		sb.WriteString(n.Name)
	case *ast.StarExpr:
		sb.WriteString("*")
		sb.WriteString(formatNode(fset, n.X))
	case *ast.SelectorExpr:
		sb.WriteString(formatNode(fset, n.X))
		sb.WriteString(".")
		sb.WriteString(n.Sel.Name)
	case *ast.ArrayType:
		sb.WriteString("[]")
		sb.WriteString(formatNode(fset, n.Elt))
	case *ast.MapType:
		sb.WriteString("map[")
		sb.WriteString(formatNode(fset, n.Key))
		sb.WriteString("]")
		sb.WriteString(formatNode(fset, n.Value))
	case *ast.ChanType:
		switch n.Dir {
		case ast.SEND:
			sb.WriteString("chan<- ")
		case ast.RECV:
			sb.WriteString("<-chan ")
		default:
			sb.WriteString("chan ")
		}
		sb.WriteString(formatNode(fset, n.Value))
	case *ast.FuncType:
		sb.WriteString("func")
		sb.WriteString(formatFieldList(fset, n.Params))
		if n.Results != nil && len(n.Results.List) > 0 {
			sb.WriteString(" ")
			sb.WriteString(formatFieldList(fset, n.Results))
		}
	case *ast.InterfaceType:
		sb.WriteString("interface{}")
	case *ast.Ellipsis:
		sb.WriteString("...")
		sb.WriteString(formatNode(fset, n.Elt))
	default:
		// Fallback: just note the type
		sb.WriteString("?")
	}
	return sb.String()
}

// formatFieldList formats a field list (params or returns).
func formatFieldList(fset *token.FileSet, fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return "()"
	}

	var parts []string
	for _, field := range fl.List {
		typeStr := formatNode(fset, field.Type)
		if len(field.Names) == 0 {
			// Unnamed parameter/return
			parts = append(parts, typeStr)
		} else {
			// Named parameters
			for _, name := range field.Names {
				parts = append(parts, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		}
	}

	return "(" + strings.Join(parts, ", ") + ")"
}

// =============================================================================
// CONTEXT FORMATTING FOR LLM PROMPTS
// =============================================================================

// FormatForPrompt formats the holographic context for LLM injection.
func (hc *HolographicContext) FormatForPrompt() string {
	var sb strings.Builder

	sb.WriteString("\n## Package Context\n")

	// Package info
	if hc.TargetPkg != "" {
		sb.WriteString(fmt.Sprintf("Package: `%s`\n", hc.TargetPkg))
	}

	// Sibling files
	if len(hc.PackageSiblings) > 0 {
		sb.WriteString(fmt.Sprintf("Sibling files in package: %d\n", len(hc.PackageSiblings)))
		for _, sib := range hc.PackageSiblings {
			sb.WriteString(fmt.Sprintf("  - %s\n", filepath.Base(sib)))
		}
	}

	// Available functions in package scope
	if len(hc.PackageSignatures) > 0 {
		sb.WriteString("\n### Functions Available in Package Scope\n")
		sb.WriteString("These are defined in sibling files and can be called without import:\n```go\n")

		// Sort by exported first, then alphabetically
		sort.Slice(hc.PackageSignatures, func(i, j int) bool {
			if hc.PackageSignatures[i].Exported != hc.PackageSignatures[j].Exported {
				return hc.PackageSignatures[i].Exported
			}
			return hc.PackageSignatures[i].Name < hc.PackageSignatures[j].Name
		})

		for _, sig := range hc.PackageSignatures {
			if sig.Receiver != "" {
				sb.WriteString(fmt.Sprintf("func (%s) %s%s %s  // %s\n",
					sig.Receiver, sig.Name, sig.Params, sig.Returns, sig.File))
			} else {
				sb.WriteString(fmt.Sprintf("func %s%s %s  // %s\n",
					sig.Name, sig.Params, sig.Returns, sig.File))
			}
		}
		sb.WriteString("```\n")
	}

	// Types in package
	if len(hc.PackageTypes) > 0 {
		sb.WriteString("\n### Types Defined in Package\n```go\n")
		for _, t := range hc.PackageTypes {
			switch t.Kind {
			case "struct":
				sb.WriteString(fmt.Sprintf("type %s struct { ... }  // %s:%d, %d fields\n",
					t.Name, t.File, t.Line, len(t.Fields)))
			case "interface":
				sb.WriteString(fmt.Sprintf("type %s interface { ... }  // %s:%d, %d methods\n",
					t.Name, t.File, t.Line, len(t.Methods)))
			default:
				sb.WriteString(fmt.Sprintf("type %s = ...  // %s:%d\n", t.Name, t.File, t.Line))
			}
		}
		sb.WriteString("```\n")
	}

	// Constants
	exportedConsts := make([]ConstDefinition, 0)
	for _, c := range hc.PackageConstants {
		if c.Exported {
			exportedConsts = append(exportedConsts, c)
		}
	}
	if len(exportedConsts) > 0 && len(exportedConsts) < 20 {
		sb.WriteString("\n### Exported Constants/Variables\n```go\n")
		for _, c := range exportedConsts {
			kind := "var"
			if c.IsConst {
				kind = "const"
			}
			sb.WriteString(fmt.Sprintf("%s %s  // %s\n", kind, c.Name, c.File))
		}
		sb.WriteString("```\n")
	}

	// Architectural context
	sb.WriteString("\n## Architectural Context\n")
	if hc.Layer != "" {
		sb.WriteString(fmt.Sprintf("- Layer: %s\n", hc.Layer))
	}
	if hc.Module != "" {
		sb.WriteString(fmt.Sprintf("- Module: %s\n", hc.Module))
	}
	if hc.Role != "" {
		sb.WriteString(fmt.Sprintf("- Role: %s\n", hc.Role))
	}
	if hc.SystemPurpose != "" {
		sb.WriteString(fmt.Sprintf("- Purpose: %s\n", hc.SystemPurpose))
	}
	if hc.HasTests {
		sb.WriteString("- Has corresponding test file: yes\n")
	}

	// Call graph (if populated)
	if len(hc.CallGraph) > 0 && len(hc.CallGraph) < 20 {
		sb.WriteString("\n### Call Relationships\n")
		for _, edge := range hc.CallGraph {
			sb.WriteString(fmt.Sprintf("- %s → %s\n", edge.Caller, edge.Callee))
		}
	}

	return sb.String()
}

// FormatSignaturesCompact returns a compact signature list for context injection.
func (hc *HolographicContext) FormatSignaturesCompact() string {
	if len(hc.PackageSignatures) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Package-scope symbols:\n")

	for _, sig := range hc.PackageSignatures {
		if sig.Receiver != "" {
			sb.WriteString(fmt.Sprintf("  (%s).%s%s%s [%s]\n",
				sig.Receiver, sig.Name, sig.Params, sig.Returns, sig.File))
		} else {
			sb.WriteString(fmt.Sprintf("  %s%s%s [%s]\n",
				sig.Name, sig.Params, sig.Returns, sig.File))
		}
	}

	return sb.String()
}

var todoPattern = regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX|BUG):?`)

// CountTODOs counts TODO/FIXME comments in file content.
func CountTODOs(content string) int {
	matches := todoPattern.FindAllString(content, -1)
	return len(matches)
}
