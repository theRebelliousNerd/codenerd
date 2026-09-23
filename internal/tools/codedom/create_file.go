package codedom

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"
)

// CreateFileTool creates a new Go or Mangle file, validated as a unit.
//
// Every new Go file in the audited runs (three test files) went through
// write_file, which checks only that the text parses. A file in the wrong
// package, with missing imports, or not gofmt'd was written and found at the
// build. create_file checks what the structure knows: the package clause
// matches the directory's package, the source parses, imports are derived,
// and the answer lists the new file's elements with their refs.
func CreateFileTool() *tools.Tool {
	return &tools.Tool{
		Name: "create_file",
		Description: "Create a new Go or Mangle file. For Go: the package clause must match the other files of the directory (package x, or x_test in a _test.go file), imports are derived from what the code uses (write none), and the file is gofmt'd. " +
			"Refused when the file exists (the element verbs change existing files). Answers with the new file's elements and refs.",
		Category: tools.CategoryCode,
		Priority: 82,
		Effect:   tools.EffectWrite,
		Execute:  executeCreateFile,
		Schema: tools.ToolSchema{
			Required: []string{"path", "source"},
			Properties: map[string]tools.Property{
				"path":   {Type: "string", Description: "Workspace-relative path of the new file (.go or .mg)"},
				"source": {Type: "string", Description: "The complete file: package clause and declarations; imports may be left out"},
			},
		},
	}
}

func executeCreateFile(ctx context.Context, args map[string]any) (string, error) {
	rawPath, _ := args["path"].(string)
	source, _ := args["source"].(string)
	if strings.TrimSpace(rawPath) == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("source is required")
	}
	abs, err := tools.ResolveWorkspacePath(ctx, "", rawPath)
	if err != nil {
		return "", err
	}
	rel := tools.WorkspaceDisplayPath(ctx, abs)
	if tools.IsSecretPath(rel) {
		return "", fmt.Errorf("%s is a secret file (execution.secret_paths); it is not written by a model", rel)
	}
	if _, err := os.Stat(abs); err == nil {
		return "", fmt.Errorf("%s already exists; the element verbs (edit_element, replace_element, insert_element) change an existing file", rel)
	}
	lang := codemodel.LanguageOf(rel)
	if lang == "" {
		return "", fmt.Errorf("create_file makes Go and Mangle files, the files CodeDOM parses; %s is neither", rel)
	}
	source = strings.Trim(codemodel.Normalize(source), "\n") + "\n"

	empty := &codemodel.File{Path: rel, Language: lang, Parsed: true}
	nf, _ := codemodel.Parse(rel, source)
	if !nf.Parsed {
		return "", fmt.Errorf("the source does not parse, so nothing was written: %v", nf.Err)
	}
	out := &codemodel.Outcome{File: nf}
	if lang == codemodel.LangGo {
		if err := checkPackageClause(abs, rel, nf.Package); err != nil {
			return "", err
		}
		formatted, err := codemodel.FormatUnit(source, true)
		if err != nil {
			return "", err
		}
		nf = codemodel.ParseGo(rel, formatted+"\n")
		if res := importResolver(ctx, rel); res != nil {
			empty.Source = ""
			derived, rep, err := codemodel.DeriveImports(empty, nf, res)
			if err != nil {
				return "", err
			}
			out.Imports = rep
			nf = codemodel.ParseGo(rel, derived)
			if !nf.Parsed {
				return "", fmt.Errorf("deriving imports broke the file: %v", nf.Err)
			}
		}
		out.File = nf
	}
	for i := range nf.Elements {
		out.Touched = append(out.Touched, &nf.Elements[i])
	}
	if err := commitFiles(ctx, []plannedWrite{{abs: abs, rel: rel, create: true, data: []byte(nf.Source), mode: 0o644}}); err != nil {
		return "", err
	}
	tools.RecordEdit(ctx, editedElements(rel, empty, out)...)
	logging.Tools("create_file completed: %s (%d elements)", rel, len(nf.Elements))

	refs := canonicalRefs(ctx, rel, nf)
	var sb strings.Builder
	fmt.Fprintf(&sb, "create_file: wrote %s (%d lines, parses", rel, nf.LineCount())
	if lang == codemodel.LangGo {
		sb.WriteString(", gofmt'd")
	}
	sb.WriteString(").\n")
	if len(out.Imports.Added) > 0 {
		fmt.Fprintf(&sb, "-- imports added: %s\n", strings.Join(out.Imports.Added, ", "))
	}
	for _, note := range out.Imports.Notes {
		fmt.Fprintf(&sb, "-- import note: %s\n", note)
	}
	for i := range nf.Elements {
		sb.WriteString(symbolRow(symbolOf(rel, &nf.Elements[i], refs)) + "\n")
	}
	return sb.String(), nil
}

// checkPackageClause refuses a new Go file whose package does not match its
// directory's: the other non-test files' package, or that package with _test
// in a _test.go file. A directory with no Go files takes any package.
func checkPackageClause(abs, rel, pkg string) error {
	dir := filepath.Dir(abs)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	found := make(map[string]bool)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.PackageClauseOnly)
		if err != nil || f.Name == nil {
			continue
		}
		found[f.Name.Name] = true
	}
	if len(found) == 0 {
		return nil
	}
	var names []string
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if pkg == n || (strings.HasSuffix(rel, "_test.go") && pkg == n+"_test") {
			return nil
		}
	}
	want := "package " + strings.Join(names, " or ")
	if strings.HasSuffix(rel, "_test.go") {
		want += " (or " + names[0] + "_test)"
	}
	return fmt.Errorf("%s declares package %s, but its directory holds %s; nothing was written", rel, pkg, want)
}
