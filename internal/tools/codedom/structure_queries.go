package codedom

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// The structural verbs for the questions the R8 audit found only grep
// answering: who imports a package (3 of the 4 greps across 12 runs), where a
// piece of text sits in the code (a brief quotes a log message; the model's
// first moves were whole-file reads of the files it might be in), and where a
// Mangle predicate is declared, derived and read (the one .mg read in those
// runs was a guessed path that did not exist).

// ImportersOfTool lists the files importing a package.
func ImportersOfTool() *tools.Tool {
	return &tools.Tool{
		Name:          "importers_of",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "List every Go file in the workspace that imports a package, with the line and the name it imports it under. Use this instead of grep to answer who depends on a package.",
		Category:      tools.CategoryCode,
		Priority:      90,
		Effect:        tools.EffectRead,
		Execute:       executeImportersOf,
		Schema: tools.ToolSchema{
			Required: []string{"package"},
			Properties: map[string]tools.Property{
				"package": {Type: "string", Description: "A workspace directory (internal/features), an import path (codenerd/internal/features, gopkg.in/yaml.v3) or a package name declared in one directory"},
				"offset":  {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executeImportersOf(ctx context.Context, args map[string]any) (string, error) {
	pkg, _ := args["package"].(string)
	if strings.TrimSpace(pkg) == "" {
		return "", fmt.Errorf("package is required")
	}
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	path, importers, index, err := provider.Importers(ctx, pkg)
	if err != nil {
		return "", err
	}
	rows := make([]string, 0, len(importers))
	for _, imp := range importers {
		rows = append(rows, fmt.Sprintf("%s:%d  imports %q as %s", imp.File, imp.Line, path, imp.Name))
	}
	logging.Tools("importers_of: %q (%s) -> %d files", pkg, path, len(importers))
	return renderPage(rows, pageArgs(args), fmt.Sprintf("files importing %s:", path), index,
		"none: no parsed Go file in the workspace imports it."), nil
}

// FindTextTool finds text in string literals, comments or identifiers and
// answers with the element holding each hit.
func FindTextTool() *tools.Tool {
	return &tools.Tool{
		Name:          "find_text",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description: "Find text inside the string literals, comments or identifiers of every Go and Mangle file (a log or error message, a TODO, a config key), answered as the ref of the element holding each hit, its file and line, and the literal or comment line. " +
			"Use this instead of grep over code: the answer is an address to pass to get_element, not a line range to read.",
		Category: tools.CategoryCode,
		Priority: 89,
		Effect:   tools.EffectRead,
		Execute:  executeFindText,
		Schema: tools.ToolSchema{
			Required: []string{"text"},
			Properties: map[string]tools.Property{
				"text":   {Type: "string", Description: "Text to find, case-sensitive, as it appears in the literal or comment"},
				"in":     {Type: "string", Description: "strings, comments, identifiers, or all; default strings and comments", Enum: []any{"strings", "comments", "identifiers", "all"}},
				"path":   {Type: "string", Description: "Optional workspace-relative directory (searched recursively) or file"},
				"offset": {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executeFindText(ctx context.Context, args map[string]any) (string, error) {
	text, _ := args["text"].(string)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	in, _ := args["in"].(string)
	path, _ := args["path"].(string)
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	hits, index, err := provider.FindText(ctx, text, in, path)
	if err != nil {
		return "", err
	}
	rows := make([]string, 0, len(hits))
	for _, h := range hits {
		ref := h.Ref
		if ref == "" {
			ref = "(between declarations)"
		}
		rows = append(rows, fmt.Sprintf("%s:%d  %s  in %s  %s", h.File, h.Line, ref, h.In, h.Text))
	}
	logging.Tools("find_text: %q in=%q path=%q -> %d hits", text, in, path, len(hits))
	where := "the workspace"
	if strings.TrimSpace(path) != "" {
		where = path
	}
	return renderPage(rows, pageArgs(args), fmt.Sprintf("%q in %s:", text, where), index,
		"none. The search is case-sensitive and covers string literals and comments unless in says otherwise; find_symbol finds declarations by name."), nil
}

// PredicateOutlineTool shows where a Mangle predicate is declared, derived
// and read.
func PredicateOutlineTool() *tools.Tool {
	return &tools.Tool{
		Name:          "predicate_outline",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral, tools.CategoryResearch},
		Description:   "Show every Mangle statement concerning a predicate, across all .mg files: its Decl, every rule and fact deriving it, and every rule reading it, each with file, lines and ref. The Mangle counterpart of find_symbol and callers_of.",
		Category:      tools.CategoryCode,
		Priority:      88,
		Effect:        tools.EffectRead,
		Execute:       executePredicateOutline,
		Schema: tools.ToolSchema{
			Required: []string{"predicate"},
			Properties: map[string]tools.Property{
				"predicate": {Type: "string", Description: "Predicate name, with or without its arity (modified_function or modified_function/2)"},
				"offset":    {Type: "integer", Description: "Row to continue from when a previous answer said there were more"},
			},
		},
	}
}

func executePredicateOutline(ctx context.Context, args map[string]any) (string, error) {
	pred, _ := args["predicate"].(string)
	if strings.TrimSpace(pred) == "" {
		return "", fmt.Errorf("predicate is required")
	}
	provider, err := structureProvider()
	if err != nil {
		return "", err
	}
	rows, index, err := provider.PredicateOutline(ctx, pred)
	if err != nil {
		return "", err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Role+"  "+symbolRow(r.Symbol))
	}
	logging.Tools("predicate_outline: %q -> %d statements", pred, len(rows))
	return renderPage(out, pageArgs(args), fmt.Sprintf("Mangle statements concerning %s:", pred), index,
		"none: no Mangle file declares, derives or reads a predicate of that name."), nil
}
