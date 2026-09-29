package codedom

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/workspace"
	"codenerd/internal/world/codemodel"
)

// executeScriptRepoint renames a Python, TypeScript or JavaScript declaration
// to a new identifier. Scope resolution decides which occurrences are the
// name: a same-named local is not one of them, and an aliased import keeps
// its local spelling. to is the identifier, not an existing declaration.
func executeScriptRepoint(ctx context.Context, from StructureSymbol, to string, paths []string) (string, error) {
	to = strings.TrimSpace(to)
	if !codemodel.ValidRename(to) {
		return "", fmt.Errorf("to %q is not an identifier; a Python, TypeScript or JavaScript rename takes the new name, not a ref", to)
	}
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	models, err := loadScriptModels(ctx, root)
	if err != nil {
		return "", err
	}
	decl := modelByPath(models, from.File)
	if decl == nil {
		return "", fmt.Errorf("%s is not a file in this workspace", from.File)
	}
	if e := decl.Element(from.Key); e == nil {
		return "", fmt.Errorf("%s has no element %s", from.File, from.Key)
	} else if e.Name == to {
		return "", fmt.Errorf("%s is already named %s", from.Ref, to)
	}
	sites, byFile, err := scriptSites(decl, from.Key, models)
	if err != nil {
		return "", err
	}
	if len(sites) == 0 {
		return "", fmt.Errorf("%s has no identifier to rename", from.Ref)
	}
	declared, err := declaredPaths(ctx, paths)
	if err != nil {
		return "", err
	}
	var missing []string
	for file := range byFile {
		if !declared[slashPath(file)] {
			missing = append(missing, file)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return "", fmt.Errorf("uses of %s are in files paths does not list: %s. Every file the call rewrites must be in paths; call again with them added", from.Ref, strings.Join(missing, ", "))
	}

	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	sort.Strings(files)
	var planned []plannedWrite
	var touched []*touchedFile
	var uses []StructureUse
	for _, file := range files {
		lf, err := loadFile(ctx, file)
		if err != nil {
			return "", err
		}
		model := modelByPath(models, lf.rel)
		if model == nil {
			return "", fmt.Errorf("%s is not a Python, TypeScript or JavaScript file", lf.rel)
		}
		if codemodel.Normalize(string(lf.data)) != model.Source {
			return "", fmt.Errorf("%s changed while it was being read; retry", lf.rel)
		}
		out, err := codemodel.ApplyRename(model, to, byFile[file])
		if err != nil {
			return "", fmt.Errorf("%s: %w", lf.rel, err)
		}
		mode := os.FileMode(0o644)
		if info, statErr := os.Stat(lf.abs); statErr == nil {
			mode = info.Mode().Perm()
		}
		data := []byte(tactile.NormalizeLineEnding(out.File.Source, tactile.DetectLineEnding(lf.data)))
		planned = append(planned, plannedWrite{abs: lf.abs, rel: lf.rel, orig: lf.data, data: data, mode: mode})
		touched = append(touched, &touchedFile{rel: lf.rel, lf: lf, before: model, out: out, sites: len(byFile[file])})
		for _, site := range byFile[file] {
			uses = append(uses, StructureUse{
				File: site.Path, Line: site.Line, Start: site.Start, End: site.End,
				Match: "exact", Ref: enclosingRef(model, site.Start),
			})
		}
	}
	if err := commitFiles(ctx, planned); err != nil {
		return "", err
	}
	for _, t := range touched {
		tools.RecordEdit(ctx, editedElements(t.rel, t.before, t.out)...)
	}
	logging.Tools("repoint: %s -> %s, %d sites in %d files", from.Ref, to, len(sites), len(planned))
	return renderRepoint(fmt.Sprintf("repoint %s -> %s", from.Ref, to), uses, touched), nil
}

// scriptUsesOutside lists uses of a script declaration that sit outside its
// own span. The declaration's name is inside the span; an import specifier
// or a call in another function is not.
func scriptUsesOutside(ctx context.Context, re *resolvedElement) ([]StructureUse, error) {
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		return nil, err
	}
	models, err := loadScriptModels(ctx, root)
	if err != nil {
		return nil, err
	}
	decl := modelByPath(models, re.file.rel)
	if decl == nil {
		return nil, fmt.Errorf("%s is not a file in this workspace", re.file.rel)
	}
	e := decl.Element(re.elem.Key)
	if e == nil {
		return nil, fmt.Errorf("%s has no element %s", re.file.rel, re.elem.Key)
	}
	sites, _, err := scriptSites(decl, e.Key, models)
	if err != nil {
		return nil, err
	}
	var uses []StructureUse
	for _, site := range sites {
		if site.Path == decl.Path && site.Start >= e.Start && site.End <= e.End {
			continue
		}
		ref := site.Path
		if f := modelByPath(models, site.Path); f != nil {
			ref = enclosingRef(f, site.Start)
		}
		uses = append(uses, StructureUse{
			File: site.Path, Line: site.Line, Start: site.Start, End: site.End,
			Match: "exact", Ref: ref,
		})
	}
	return uses, nil
}

func scriptSites(decl *codemodel.File, key string, models map[string]*codemodel.File) ([]codemodel.RenameSite, map[string][]codemodel.RenameSite, error) {
	var others []*codemodel.File
	for _, m := range models {
		if m.Path != decl.Path {
			others = append(others, m)
		}
	}
	sites, err := codemodel.RenameSites(decl, key, others)
	if err != nil {
		return nil, nil, err
	}
	byFile := make(map[string][]codemodel.RenameSite)
	for _, site := range sites {
		byFile[site.Path] = append(byFile[site.Path], site)
	}
	return sites, byFile, nil
}

func declaredPaths(ctx context.Context, paths []string) (map[string]bool, error) {
	declared := make(map[string]bool, len(paths))
	for _, p := range paths {
		lf, err := loadFile(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("paths: %w", err)
		}
		declared[slashPath(lf.rel)] = true
	}
	return declared, nil
}

// loadScriptModels parses every Python, TypeScript and JavaScript file the
// workspace admits. The same admission the structure index uses is what makes
// a rename see the uses the index would report, and no file the index skips.
func loadScriptModels(ctx context.Context, root string) (map[string]*codemodel.File, error) {
	mem, err := workspace.For(root)
	if err != nil {
		return nil, err
	}
	if err := mem.Refresh(); err != nil {
		return nil, err
	}
	out := map[string]*codemodel.File{}
	err = mem.Walk(ctx, func(rel string, d fs.DirEntry) error {
		if d.IsDir() || rel == "" || !codemodel.IsScriptPath(rel) {
			return nil
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		canonical := types.CanonicalPath(root, abs)
		if tools.IsSecretPath(canonical) {
			return nil
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		f, ok := codemodel.ParseRoot(canonical, abs, root, string(data))
		if !ok || f == nil {
			return nil
		}
		out[slashPath(f.Path)] = f
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func modelByPath(models map[string]*codemodel.File, path string) *codemodel.File {
	return models[slashPath(path)]
}

func slashPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// enclosingRef is the innermost element whose span holds start, as the ref
// the other tools print.
func enclosingRef(f *codemodel.File, start int) string {
	bestSpan := int(^uint(0) >> 1)
	var best *codemodel.Element
	for i := range f.Elements {
		e := &f.Elements[i]
		if e.Kind == codemodel.KindSyntaxError {
			continue
		}
		if start >= e.Start && start < e.End && e.End-e.Start < bestSpan {
			best = e
			bestSpan = e.End - e.Start
		}
	}
	if best == nil {
		return f.Path
	}
	return RefOf(f.Path, best, nil)
}
