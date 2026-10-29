package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"codenerd/internal/core"
	"codenerd/internal/types"
	"codenerd/internal/world"
	"codenerd/internal/world/codemodel"
)

// A task's structural preload: the code its brief names, pushed into the brief
// before the model asks for it (G8 / CTX-B4). Which files, packages and
// elements, and in what form, is the kernel's decision
// (policy/campaign_preload.mg, task_preload/3); this file measures what the
// brief names against the structure index, asks, and renders the answer the
// way the structural tools print it, each row with its element revision.

// The forms task_preload hands a target in.
const (
	preloadOutline   = "/outline"
	preloadCount     = "/count"
	preloadElement   = "/element"
	preloadSignature = "/signature"
)

// preloadMeasure is what Go measured for the asked task's preload, and the
// index answers the rendering reads back.
type preloadMeasure struct {
	facts    []core.Fact
	outlines map[string][]world.StructSymbol // canonical path -> declarations
	shapes   map[string]string               // canonical path -> /file | /package
	elements map[string]world.StructSymbol   // ref -> the one element it names
}

// preloadRevision is the revision a preloaded element is keyed by: the
// structure index's element revision (the hash of the element's own bytes),
// the precondition the edit verbs take. It is the one place that says so.
func preloadRevision(sym world.StructSymbol) string {
	return sym.Revision
}

// briefPathToken matches a word that can name a workspace path: a token with a
// slash or a file extension.
var briefPathToken = regexp.MustCompile("[A-Za-z0-9_.\\-/\\\\]+")

// briefPaths are the words of a brief that name an existing file or directory
// inside the workspace, canonical (workspace-relative, forward slashes). A
// trailing ":line" or ":start-end" citation is dropped.
func briefPaths(brief, workspace string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tok := range briefPathToken.FindAllString(brief, -1) {
		tok = strings.TrimRight(tok, ".-")
		if tok == "" || (!strings.ContainsAny(tok, "/\\") && filepath.Ext(tok) == "") {
			continue
		}
		canonical := types.CanonicalPath(workspace, tok)
		if canonical == "" || canonical == "." || strings.HasPrefix(canonical, "../") || filepath.IsAbs(filepath.FromSlash(canonical)) {
			continue
		}
		if seen[canonical] {
			continue
		}
		if _, err := os.Stat(resolveWorkspacePath(workspace, canonical)); err != nil {
			continue
		}
		seen[canonical] = true
		out = append(out, canonical)
	}
	return out
}

// briefIdentToken matches an identifier, possibly qualified (pkg.Name,
// Recv.Method).
var briefIdentToken = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*`)

// briefIdentifiers are the words of a brief shaped like code identifiers:
// anything inside backticks, and elsewhere a qualified name (pkg.Name), a
// camelCase or MixedCase name with an inner capital, or a snake_case name.
// Words of a path and file names are not identifiers. The index decides which
// of them name an element.
func briefIdentifiers(brief string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(tok string) {
		if tok == "" || seen[tok] || fileNameLike(tok) {
			return
		}
		seen[tok] = true
		out = append(out, tok)
	}
	parts := strings.Split(brief, "`")
	for i, part := range parts {
		quoted := i%2 == 1 && i < len(parts)-1
		for _, word := range strings.Fields(part) {
			if strings.ContainsAny(word, "/\\") {
				continue // a path; briefPaths reads it
			}
			for _, tok := range briefIdentToken.FindAllString(word, -1) {
				if quoted || codeShaped(tok) {
					add(tok)
				}
			}
		}
	}
	return out
}

// fileNameLike reports whether a dotted token is a file name: its last
// segment is a known source or document extension.
func fileNameLike(tok string) bool {
	i := strings.LastIndex(tok, ".")
	if i < 0 {
		return false
	}
	if codemodel.LanguageOf(tok) != "" {
		return true
	}
	switch strings.ToLower(tok[i:]) {
	case ".md", ".markdown", ".txt", ".rst", ".adoc", ".yaml", ".yml", ".json", ".toml", ".mod", ".sum", ".py", ".ts", ".js", ".sh", ".csv", ".html", ".css", ".sql":
		return true
	}
	return false
}

// codeShaped reports whether a bare token reads as an identifier rather than a
// word: qualified, with an inner capital and a lower-case letter, or with an
// underscore between letters.
func codeShaped(tok string) bool {
	if strings.Contains(tok, ".") {
		return true
	}
	hasLower, innerUpper := false, false
	for i, r := range tok {
		if unicode.IsLower(r) {
			hasLower = true
		}
		if i > 0 && unicode.IsUpper(r) {
			innerUpper = true
		}
	}
	if innerUpper && hasLower {
		return true
	}
	return hasLower && strings.Contains(strings.Trim(tok, "_"), "_")
}

// structuralShape measures what the index parses at a canonical path: a file
// it models, or a directory holding Go files it models.
func structuralShape(workspace, canonical string) string {
	info, err := os.Stat(resolveWorkspacePath(workspace, canonical))
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return "/package"
	}
	if codemodel.LanguageOf(canonical) == "" {
		return ""
	}
	return "/file"
}

// measurePreload resolves the asked task's candidates against the structure
// index: the paths its brief names, the code it writes, the code files the
// tasks it depends on wrote, and the identifiers its brief names. Only what the
// index parses is asserted; the kernel decides what of it is preloaded.
func measurePreload(ctx context.Context, idx *world.StructureIndex, asked evidenceTask, arts []measuredArtifact, workspace string) (preloadMeasure, error) {
	pm := preloadMeasure{
		outlines: map[string][]world.StructSymbol{},
		shapes:   map[string]string{},
		elements: map[string]world.StructSymbol{},
	}
	outline := func(canonical string) (bool, error) {
		if _, done := pm.shapes[canonical]; done {
			return true, nil
		}
		shape := structuralShape(workspace, canonical)
		if shape == "" {
			return false, nil
		}
		syms, _, err := idx.Outline(ctx, canonical)
		if err != nil {
			return false, fmt.Errorf("outline %s: %w", canonical, err)
		}
		if len(syms) == 0 {
			return false, nil
		}
		pm.outlines[canonical] = syms
		pm.shapes[canonical] = shape
		pm.facts = append(pm.facts, core.Fact{
			Predicate: "code_outline",
			Args:      []any{canonical, types.MangleAtom(shape), int64(len(syms))},
		})
		return true, nil
	}

	brief := asked.description + "\n" + asked.shardInput
	for _, p := range briefPaths(brief, workspace) {
		ok, err := outline(p)
		if err != nil {
			return pm, err
		}
		if ok {
			pm.facts = append(pm.facts, core.Fact{Predicate: "task_brief_names", Args: []any{asked.id, p}})
		}
	}

	var own []string
	for _, a := range asked.artifacts {
		own = append(own, a.Path)
	}
	own = append(own, asked.writeSet...)
	for _, p := range own {
		if strings.TrimSpace(p) == "" || containsGlobMeta(p) {
			continue
		}
		canonical := types.CanonicalPath(workspace, filepath.ToSlash(p))
		ok, err := outline(canonical)
		if err != nil {
			return pm, err
		}
		if ok {
			pm.facts = append(pm.facts, core.Fact{Predicate: "task_output_path", Args: []any{asked.id, canonical}})
		}
	}

	deps := map[string]bool{}
	for _, d := range asked.dependsOn {
		deps[d] = true
	}
	for _, a := range arts {
		if !deps[a.producer] || codemodel.LanguageOf(a.path) == "" {
			continue
		}
		if _, err := outline(a.path); err != nil {
			return pm, err
		}
	}

	for _, ident := range briefIdentifiers(brief) {
		syms, _, err := idx.Resolve(ctx, ident)
		if err != nil {
			return pm, fmt.Errorf("resolve %s: %w", ident, err)
		}
		if len(syms) > 1 {
			// A short name several declarations share names the one in the
			// code this task is about: the files and packages measured above.
			syms = withinScopes(syms, pm.shapes)
		}
		if len(syms) != 1 || syms[0].Kind == string(codemodel.KindSyntaxError) {
			continue
		}
		sym := syms[0]
		if _, done := pm.elements[sym.Ref]; done {
			continue
		}
		pm.elements[sym.Ref] = sym
		pm.facts = append(pm.facts, core.Fact{
			Predicate: "task_brief_element",
			Args:      []any{asked.id, sym.Ref, int64(sym.EndLine - sym.StartLine + 1), preloadRevision(sym)},
		})
	}
	return pm, nil
}

// withinScopes keeps the declarations in a measured file, or directly in a
// measured package directory.
func withinScopes(syms []world.StructSymbol, scopes map[string]string) []world.StructSymbol {
	var out []world.StructSymbol
	for _, sym := range syms {
		dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(sym.File)))
		if _, ok := scopes[sym.File]; ok {
			out = append(out, sym)
		} else if shape, ok := scopes[dir]; ok && shape == "/package" {
			out = append(out, sym)
		}
	}
	return out
}

// askTaskPreload returns the kernel's task_preload rows for the task.
func (o *Orchestrator) askTaskPreload(taskID string) (map[string]string, error) {
	facts, err := o.kernel.Query("task_preload")
	if err != nil {
		return nil, fmt.Errorf("query task_preload: %w", err)
	}
	forms := map[string]string{}
	for _, f := range facts {
		if factArg(f, 0) != taskID {
			continue
		}
		target, form := factArg(f, 1), factArg(f, 2)
		if target == "" || form == "" {
			return nil, fmt.Errorf("the kernel derived a malformed task_preload row %v", f.Args)
		}
		if prev, ok := forms[target]; ok && prev != form {
			return nil, fmt.Errorf("the kernel derived %s and %s for %s of %s; the policy must decide one", prev, form, target, taskID)
		}
		forms[target] = form
	}
	return forms, nil
}

// outlineRow is one declaration as the structural tools print it.
func outlineRow(sym world.StructSymbol) string {
	row := fmt.Sprintf("%s:%d-%d  %s  %s", sym.File, sym.StartLine, sym.EndLine, sym.Kind, sym.Ref)
	if rev := preloadRevision(sym); rev != "" {
		row += "  rev " + rev
	}
	if sym.Signature != "" {
		row += "  " + sym.Signature
	}
	if sym.Doc != "" {
		row += "  // " + sym.Doc
	}
	return row
}

// elementSource is an element's source, line-numbered the way get_element
// numbers it, read from the file the index located it in.
func elementSource(workspace string, sym world.StructSymbol) (string, error) {
	data, err := os.ReadFile(resolveWorkspacePath(workspace, sym.File))
	if err != nil {
		return "", err
	}
	model, ok := codemodel.Parse(sym.File, string(data))
	if !ok {
		return "", fmt.Errorf("%s is not a file the structure index parses", sym.File)
	}
	return model.NumberedLines(sym.StartLine, sym.EndLine), nil
}

// renderPreload writes the preload section: outlines, then elements, then the
// targets named by count or signature; each group in path or ref order.
func renderPreload(forms map[string]string, pm preloadMeasure, workspace string) (string, map[string]int) {
	counts := map[string]int{}
	if len(forms) == 0 {
		return "", counts
	}
	rank := map[string]int{preloadOutline: 0, preloadElement: 1, preloadCount: 2, preloadSignature: 3}
	targets := make([]string, 0, len(forms))
	for t := range forms {
		targets = append(targets, t)
	}
	sort.Slice(targets, func(i, j int) bool {
		a, b := targets[i], targets[j]
		if rank[forms[a]] != rank[forms[b]] {
			return rank[forms[a]] < rank[forms[b]]
		}
		return a < b
	})
	var sb strings.Builder
	sb.WriteString("## Code this task names (structure index; rev is the element revision an edit verb takes as its precondition)\n")
	for _, t := range targets {
		form := forms[t]
		counts[form]++
		switch form {
		case preloadOutline:
			fmt.Fprintf(&sb, "\n### declarations in %s:\n", t)
			for _, sym := range pm.outlines[t] {
				sb.WriteString(outlineRow(sym))
				sb.WriteString("\n")
			}
		case preloadCount:
			fmt.Fprintf(&sb, "\n### %s: %d declarations (package_outline path=%s pages them)\n", t, len(pm.outlines[t]), t)
		case preloadElement:
			sym := pm.elements[t]
			fmt.Fprintf(&sb, "\n### %s  %s  %s:%d-%d  rev %s\n", sym.Ref, sym.Kind, sym.File, sym.StartLine, sym.EndLine, preloadRevision(sym))
			src, err := elementSource(workspace, sym)
			if err != nil {
				fmt.Fprintf(&sb, "(source unavailable: %v)\n", err)
				continue
			}
			sb.WriteString(src)
		case preloadSignature:
			sym := pm.elements[t]
			fmt.Fprintf(&sb, "\n### %s (%d lines: get_element ref=%s pages it)\n", outlineRow(sym), sym.EndLine-sym.StartLine+1, sym.Ref)
		}
	}
	return sb.String(), counts
}
