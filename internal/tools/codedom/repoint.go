package codedom

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"
)

// =============================================================================
// REPOINT
// =============================================================================
// A change with a blast radius is carried out by the tool, not by the model
// hand-editing. Ladder run R4-1 moved five types from perception to
// articulation -- one conceptual change -- as about 200 tool calls over 11
// files in 75 minutes, 8 of those files editing an import block by hand.
// repoint rewrites every use of one package-level name to another, derives the
// imports of every file it touched, parses them all, and writes them all or
// none.
//
// It is syntactic: uses are found from the parse (qualified selectors in every
// importer, bare identifiers in the declaring package), not from type
// information, so it repoints package-level names and not methods, whose uses
// go through values of types only a type checker knows.

// RepointTool rewrites every use of one package-level name to another.
func RepointTool() *tools.Tool {
	return &tools.Tool{
		Name: "repoint",
		Description: "Rewrite every use of a package-level Go name (function, type, var, const) to another one, across the workspace, in one transaction: " +
			"qualifiers and imports are rewritten and derived in every touched file, every file must parse, and all are written or none. " +
			"paths is the write set: every file holding a use must be listed, and a call that finds uses elsewhere is refused with the list. " +
			"Methods are not repointed (their uses need type information).",
		Category: tools.CategoryCode,
		Priority: 81,
		Effect:   tools.EffectWrite,
		Execute:  executeRepoint,
		Schema: tools.ToolSchema{
			Required: []string{"from", "to", "paths"},
			Properties: map[string]tools.Property{
				"from":  {Type: "string", Description: "The name whose uses move: a ref or package-qualified name (perception.PiggybackEnvelope)"},
				"to":    {Type: "string", Description: "The declared name they should use instead (articulation.PiggybackEnvelope)"},
				"paths": {Type: "array", Description: "Every file holding a use of from: the transaction's write set. find_symbol, callers_of or a refused call lists them.", Items: &tools.PropertyItems{Type: "string"}},
			},
		},
	}
}

// repointTarget is the name uses are rewritten to.
type repointTarget struct {
	sym        StructureSymbol
	dir        string
	importPath string
	pkg        string
	name       string
}

func executeRepoint(ctx context.Context, args map[string]any) (string, error) {
	from, _ := args["from"].(string)
	to, _ := args["to"].(string)
	paths := parseStringArray(args["paths"])
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return "", fmt.Errorf("from and to are required")
	}
	provider := optionalStructureProvider()
	if provider == nil {
		return "", fmt.Errorf("repoint needs the structure index, and none is registered in this process")
	}
	fromSyms, uses, _, err := provider.Uses(ctx, from)
	if err != nil {
		return "", err
	}
	if len(fromSyms) != 1 {
		return "", fmt.Errorf("from %q must name one element", from)
	}
	if fromSyms[0].Kind == string(codemodel.KindMethod) {
		return "", fmt.Errorf("%s is a method; repoint moves package-level names only", fromSyms[0].Ref)
	}
	target, err := resolveRepointTarget(ctx, provider, to)
	if err != nil {
		return "", err
	}
	if len(uses) == 0 {
		return "", fmt.Errorf("%s has no uses outside its own declaration; nothing to repoint", fromSyms[0].Ref)
	}
	planned, touched, err := planRepoint(ctx, fromSyms[0], uses, target, paths, nil)
	if err != nil {
		return "", err
	}
	if err := commitFiles(ctx, planned); err != nil {
		return "", err
	}
	for _, t := range touched {
		tools.RecordEdit(ctx, editedElements(t.rel, t.before, t.out)...)
	}
	logging.Tools("repoint: %s -> %s, %d uses in %d files", fromSyms[0].Ref, target.sym.Ref, len(uses), len(planned))
	return renderRepoint(fmt.Sprintf("repoint %s -> %s", fromSyms[0].Ref, target.sym.Ref), uses, touched), nil
}

func resolveRepointTarget(ctx context.Context, provider StructureProvider, to string) (*repointTarget, error) {
	syms, _, err := provider.Resolve(ctx, to)
	if err != nil {
		return nil, err
	}
	switch len(syms) {
	case 0:
		return nil, fmt.Errorf("to %q names no declaration; declare it first (insert_element or create_file)", to)
	case 1:
	default:
		return nil, refAmbiguity(to, syms)
	}
	sym := syms[0]
	if sym.Kind == string(codemodel.KindMethod) || !strings.HasSuffix(sym.File, ".go") {
		return nil, fmt.Errorf("to %s must be a package-level Go name", sym.Ref)
	}
	pkg, name, _ := strings.Cut(sym.ID, ".")
	importers, err := importPathOf(ctx, provider, sym.File)
	if err != nil {
		return nil, err
	}
	return &repointTarget{sym: sym, dir: filepath.ToSlash(filepath.Dir(sym.File)), importPath: importers, pkg: pkg, name: name}, nil
}

// importPathOf asks the index for the import path of the package a file is in.
func importPathOf(ctx context.Context, provider StructureProvider, file string) (string, error) {
	path, _, _, err := provider.Importers(ctx, filepath.ToSlash(filepath.Dir(file)))
	return path, err
}

// touchedFile is one file of a multi-file transaction, validated.
type touchedFile struct {
	rel    string
	lf     *loadedFile
	before *codemodel.File
	out    *codemodel.Outcome
	sites  int
}

// planRepoint rewrites every use in memory, validates each file, and returns
// the writes. finish, when set, runs on each file's result before it is
// planned (delete_element uses it to delete the element in its own file).
func planRepoint(ctx context.Context, from StructureSymbol, uses []StructureUse, target *repointTarget, paths []string,
	finish func(rel string, f *codemodel.File) (*codemodel.Outcome, error)) ([]plannedWrite, []*touchedFile, error) {

	byFile := make(map[string][]StructureUse)
	for _, u := range uses {
		byFile[u.File] = append(byFile[u.File], u)
	}
	declared := make(map[string]bool)
	for _, p := range paths {
		lf, err := loadFile(ctx, p)
		if err != nil {
			return nil, nil, fmt.Errorf("paths: %w", err)
		}
		declared[lf.rel] = true
	}
	var missing []string
	for file := range byFile {
		if !declared[file] {
			missing = append(missing, file)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("uses of %s are in files paths does not list: %s. Every file the call rewrites must be in paths; call again with them added", from.Ref, strings.Join(missing, ", "))
	}

	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	if finish != nil && byFile[from.File] == nil {
		files = append(files, from.File)
	}
	sort.Strings(files)

	var planned []plannedWrite
	var touched []*touchedFile
	for _, file := range files {
		lf, err := loadFile(ctx, file)
		if err != nil {
			return nil, nil, err
		}
		model, ok := codemodel.Parse(lf.rel, string(lf.data))
		if !ok || model.Language != codemodel.LangGo {
			return nil, nil, fmt.Errorf("%s is not a Go file", lf.rel)
		}
		out := &codemodel.Outcome{File: model}
		if fileUses := byFile[file]; len(fileUses) > 0 {
			out, err = rewriteUses(ctx, lf.rel, model, fileUses, target)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", lf.rel, err)
			}
		}
		if finish != nil && file == from.File {
			done, err := finish(lf.rel, out.File)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", lf.rel, err)
			}
			done.Touched = append(out.Touched, done.Touched...)
			done.Imports.Added = append(out.Imports.Added, done.Imports.Added...)
			done.Imports.Removed = append(out.Imports.Removed, done.Imports.Removed...)
			done.Imports.Notes = append(out.Imports.Notes, done.Imports.Notes...)
			out = done
		}
		mode := os.FileMode(0o644)
		if info, err := os.Stat(lf.abs); err == nil {
			mode = info.Mode().Perm()
		}
		data := []byte(tactile.NormalizeLineEnding(out.File.Source, tactile.DetectLineEnding(lf.data)))
		planned = append(planned, plannedWrite{abs: lf.abs, rel: lf.rel, orig: lf.data, data: data, mode: mode})
		touched = append(touched, &touchedFile{rel: lf.rel, lf: lf, before: model, out: out, sites: len(byFile[file])})
	}
	return planned, touched, nil
}

// rewriteUses rewrites the uses in one file as one change spanning the first
// to the last, so the model validates the whole file once: every element
// holding a use is targeted, every other element must come out unchanged.
func rewriteUses(ctx context.Context, rel string, model *codemodel.File, uses []StructureUse, target *repointTarget) (*codemodel.Outcome, error) {
	sort.Slice(uses, func(i, j int) bool { return uses[i].Start < uses[j].Start })
	dir := filepath.ToSlash(filepath.Dir(rel))
	qualifier := ""
	if dir != target.dir {
		qualifier = target.pkg
		for _, imp := range model.Imports {
			if imp.Path == target.importPath {
				qualifier = imp.LocalName()
				break
			}
			if imp.LocalName() == target.pkg {
				return nil, fmt.Errorf("it already imports %s as %s, the name %s would need; rename that import first", imp.Path, target.pkg, target.importPath)
			}
		}
	}
	replacement := target.name
	if qualifier != "" {
		replacement = qualifier + "." + target.name
	}
	lo, hi := uses[0].Start, uses[len(uses)-1].End
	if lo < 0 || hi > len(model.Source) || lo > hi {
		return nil, fmt.Errorf("the index's use positions are stale; retry")
	}
	var sb strings.Builder
	cursor := lo
	targeted := make(map[string]bool)
	for _, u := range uses {
		if u.Start < cursor || u.End > len(model.Source) {
			return nil, fmt.Errorf("overlapping or stale use positions; retry")
		}
		sb.WriteString(model.Source[cursor:u.Start])
		sb.WriteString(replacement)
		cursor = u.End
		for i := range model.Elements {
			e := &model.Elements[i]
			if u.Start >= e.Start && u.Start < e.End {
				targeted[e.Key] = true
			}
		}
	}
	sb.WriteString(model.Source[cursor:hi])
	keys := make([]string, 0, len(targeted))
	for k := range targeted {
		keys = append(keys, k)
	}
	res := pinnedResolver{Resolver: importResolver(ctx, rel), qualifier: qualifier, path: target.importPath}
	return codemodel.Apply(model, codemodel.Change{Start: lo, End: hi, Text: sb.String()}, keys, res)
}

// pinnedResolver answers the one qualifier a repoint introduces with the path
// it means, and defers every other question to the workspace resolver.
type pinnedResolver struct {
	codemodel.Resolver
	qualifier, path string
}

func (r pinnedResolver) ResolveQualifier(q string) (string, []string) {
	if q == r.qualifier && r.qualifier != "" {
		return r.path, []string{r.path}
	}
	if r.Resolver == nil {
		return "", nil
	}
	return r.Resolver.ResolveQualifier(q)
}

func (r pinnedResolver) DeclaredInPackage(name string) bool {
	if r.Resolver == nil {
		return false
	}
	return r.Resolver.DeclaredInPackage(name)
}

func (r pinnedResolver) ModulePath() string {
	if r.Resolver == nil {
		return ""
	}
	return r.Resolver.ModulePath()
}

// repointAndDelete is delete_element with replace_with: the remaining uses
// are repointed and the element deleted, in one transaction.
func repointAndDelete(ctx context.Context, re *resolvedElement, uses []StructureUse, replaceWith string, paths []string) (string, error) {
	provider := optionalStructureProvider()
	if re.model.Language != codemodel.LangGo || re.elem.Kind == codemodel.KindMethod {
		return "", fmt.Errorf("replace_with repoints package-level Go names only")
	}
	target, err := resolveRepointTarget(ctx, provider, replaceWith)
	if err != nil {
		return "", err
	}
	fromSyms, _, err := provider.Resolve(ctx, RefOf(re.file.rel, re.elem, re.refs))
	if err != nil || len(fromSyms) != 1 {
		return "", fmt.Errorf("the index cannot resolve %s: %v", re.elem.Key, err)
	}
	key := re.elem.Key
	finish := func(rel string, f *codemodel.File) (*codemodel.Outcome, error) {
		e := f.Element(key)
		if e == nil {
			return nil, fmt.Errorf("%s vanished while its uses were rewritten", key)
		}
		return codemodel.Apply(f, deletionChange(f, e), []string{key}, importResolver(ctx, rel))
	}
	planned, touched, err := planRepoint(ctx, fromSyms[0], uses, target, paths, finish)
	if err != nil {
		return "", err
	}
	if err := commitFiles(ctx, planned); err != nil {
		return "", err
	}
	for _, t := range touched {
		tools.RecordEdit(ctx, editedElements(t.rel, t.before, t.out)...)
	}
	return renderRepoint(fmt.Sprintf("delete_element %s, its uses repointed to %s", fromSyms[0].Ref, target.sym.Ref), uses, touched), nil
}

// renderRepoint is a multi-file edit's answer: per file, the rewritten sites,
// the imports that moved, and every touched element's new ref and rev.
func renderRepoint(title string, uses []StructureUse, touched []*touchedFile) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %d uses rewritten in %d files; every file parses and was written.\n", title, len(uses), len(touched))
	for _, t := range touched {
		fmt.Fprintf(&sb, "\n%s: %d sites", t.rel, t.sites)
		if len(t.out.Imports.Added) > 0 {
			fmt.Fprintf(&sb, "; imports added %s", strings.Join(t.out.Imports.Added, ", "))
		}
		if len(t.out.Imports.Removed) > 0 {
			fmt.Fprintf(&sb, "; imports dropped %s", strings.Join(t.out.Imports.Removed, ", "))
		}
		sb.WriteString("\n")
		for _, note := range t.out.Imports.Notes {
			fmt.Fprintf(&sb, "  -- import note: %s\n", note)
		}
		for _, key := range t.out.Removed {
			fmt.Fprintf(&sb, "  -- removed %s\n", key)
		}
		for _, e := range t.out.Touched {
			if e.Kind == codemodel.KindHeader {
				continue
			}
			fmt.Fprintf(&sb, "  %s  %s  %d-%d  rev %s\n", RefOf(t.rel, e, nil), e.Kind, e.StartLine, e.EndLine, e.Revision)
		}
	}
	return sb.String()
}
