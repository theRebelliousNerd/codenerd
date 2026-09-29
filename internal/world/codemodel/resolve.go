package codemodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// resolveImports fills Import.Resolved and the import binds' files. Python
// and TS/JS both record the specifier during the walk, before any path is
// known, so the bind ids that name an export can only be assigned after this.
func (s *scrape) resolveImports() {
	if s.lang == LangPython {
		// Python imports were resolved during the walk: the filesystem
		// answers don't depend on other imports in the file.
		return
	}
	for i := range s.imports {
		imp := &s.imports[i]
		if imp.Resolved != "" || imp.Spec == "" {
			continue
		}
		if isRelativeSpec(imp.Spec) {
			if file := resolveRelativeJS(filepath.Dir(s.abs), imp.Spec); file != "" {
				imp.Resolved = displayPath(s.root, file)
				imp.Path = imp.Resolved
			}
		}
	}
	// Bind files were set from the resolved path at walk time for JS, because
	// resolveTSImport ran then. Nothing more to copy.
}

func displayPath(root, abs string) string {
	abs = filepath.Clean(abs)
	if root == "" {
		return filepath.ToSlash(abs)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return filepath.ToSlash(abs)
	}
	return rel
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// resolvePythonModule finds the file a Python import names. level is the
// number of leading dots; an absolute import walks ancestors of the file
// and takes the nearest one that contains the module.
func resolvePythonModule(absFile, root, module string, level int) string {
	if absFile == "" {
		return ""
	}
	dir := filepath.Dir(absFile)
	if level > 0 {
		base := dir
		for i := 1; i < level; i++ {
			parent := filepath.Dir(base)
			if parent == base {
				return ""
			}
			base = parent
		}
		if module == "" {
			return ""
		}
		return probePython(base, module)
	}
	if module == "" {
		return ""
	}
	// `import os.path` probes os/path.py and, failing that, os.py.
	cur := dir
	stop := ""
	if root != "" {
		stop = filepath.Clean(root)
	}
	for i := 0; i < 32; i++ {
		if file := probePython(cur, module); file != "" {
			return file
		}
		if stop != "" && filepath.Clean(cur) == stop {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return ""
}

func probePython(dir, dotted string) string {
	if dotted == "" {
		return ""
	}
	rel := filepath.Join(strings.Split(dotted, ".")...)
	candidates := []string{
		filepath.Join(dir, rel+".py"),
		filepath.Join(dir, rel, "__init__.py"),
		filepath.Join(dir, rel+".pyi"),
		filepath.Join(dir, rel, "__init__.pyi"),
	}
	for _, c := range candidates {
		if isFile(c) {
			return c
		}
	}
	return ""
}

var jsSourceExts = []string{".ts", ".tsx", ".d.ts", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}

func isRelativeSpec(spec string) bool {
	return spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")
}

// resolveTSImport resolves an ESM or require specifier. Relative specifiers
// are resolved from the file's directory; bare specifiers go through the
// nearest tsconfig or jsconfig, including extends.
func resolveTSImport(absFile, root, spec string) string {
	if absFile == "" || spec == "" {
		return ""
	}
	dir := filepath.Dir(absFile)
	if isRelativeSpec(spec) {
		return resolveRelativeJS(dir, spec)
	}
	cfg := loadNearestTSConfig(dir, root)
	if cfg == nil {
		return ""
	}
	if rel, ok := matchTSPaths(cfg.paths, spec); ok {
		base := cfg.base
		if base == "" {
			base = filepath.Dir(cfg.path)
		}
		if file := resolveRelativeJS(base, rel); file != "" {
			return file
		}
		if file := resolveRelativeJS(base, "./"+rel); file != "" {
			return file
		}
	}
	if cfg.base != "" {
		if file := resolveRelativeJS(cfg.base, spec); file != "" {
			return file
		}
		if file := resolveRelativeJS(cfg.base, "./"+spec); file != "" {
			return file
		}
	}
	return ""
}

func resolveRelativeJS(dir, spec string) string {
	base := filepath.Clean(filepath.Join(dir, filepath.FromSlash(spec)))
	ext := filepath.Ext(spec)
	if ext != "" {
		if isFile(base) {
			return base
		}
		// A `.js` specifier in a TypeScript tree usually names the `.ts` source.
		switch ext {
		case ".js":
			for _, alt := range []string{".ts", ".tsx", ".d.ts"} {
				if isFile(strings.TrimSuffix(base, ext) + alt) {
					return strings.TrimSuffix(base, ext) + alt
				}
			}
		case ".jsx":
			if isFile(strings.TrimSuffix(base, ext) + ".tsx") {
				return strings.TrimSuffix(base, ext) + ".tsx"
			}
		}
		return ""
	}
	if isFile(base) {
		return base
	}
	for _, e := range jsSourceExts {
		if isFile(base + e) {
			return base + e
		}
	}
	for _, e := range jsSourceExts {
		p := filepath.Join(base, "index"+e)
		if isFile(p) {
			return p
		}
	}
	return ""
}

type tsConfig struct {
	path  string
	base  string
	paths map[string][]string
}

func loadNearestTSConfig(dir, root string) *tsConfig {
	stop := ""
	if root != "" {
		stop = filepath.Clean(root)
	}
	cur := dir
	for i := 0; i < 32; i++ {
		for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
			p := filepath.Join(cur, name)
			if isFile(p) {
				return loadTSConfig(p, map[string]bool{})
			}
		}
		if stop != "" && filepath.Clean(cur) == stop {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return nil
}

func loadTSConfig(path string, seen map[string]bool) *tsConfig {
	path = filepath.Clean(path)
	if seen[path] {
		return nil
	}
	seen[path] = true
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stripJSONC(string(data))), &doc); err != nil {
		return nil
	}
	cfg := &tsConfig{path: path, paths: map[string][]string{}}
	if raw, ok := doc["extends"]; ok {
		var ext string
		if json.Unmarshal(raw, &ext) == nil && ext != "" && isRelativeSpec(ext) {
			parent := filepath.Join(filepath.Dir(path), filepath.FromSlash(ext))
			if info, err := os.Stat(parent); err == nil && info.IsDir() {
				parent = filepath.Join(parent, "tsconfig.json")
			} else if !strings.HasSuffix(strings.ToLower(parent), ".json") {
				parent += ".json"
			}
			if p := loadTSConfig(parent, seen); p != nil {
				cfg.base = p.base
				cfg.paths = p.paths
			}
		}
	}
	var opts map[string]json.RawMessage
	if raw, ok := doc["compilerOptions"]; ok {
		_ = json.Unmarshal(raw, &opts)
	}
	if raw, ok := opts["baseUrl"]; ok {
		var base string
		if json.Unmarshal(raw, &base) == nil {
			if base == "" || base == "." {
				cfg.base = filepath.Dir(path)
			} else {
				cfg.base = filepath.Join(filepath.Dir(path), filepath.FromSlash(base))
			}
		}
	}
	if raw, ok := opts["paths"]; ok {
		var paths map[string][]string
		if json.Unmarshal(raw, &paths) == nil && paths != nil {
			cfg.paths = paths
		}
	}
	return cfg
}

// matchTSPaths applies the longest matching paths pattern. A star matches
// across slashes, the way TypeScript's path mapping does.
func matchTSPaths(paths map[string][]string, spec string) (string, bool) {
	bestScore := -1
	best := ""
	for pat, targets := range paths {
		if len(targets) == 0 || targets[0] == "" {
			continue
		}
		star := strings.Index(pat, "*")
		var captured string
		score := 0
		switch {
		case star < 0:
			if pat != spec {
				continue
			}
			score = len(pat) + 1
		default:
			prefix, suffix := pat[:star], pat[star+1:]
			if !strings.HasPrefix(spec, prefix) || !strings.HasSuffix(spec[len(prefix):], suffix) {
				continue
			}
			rest := spec[len(prefix):]
			if len(suffix) > len(rest) {
				continue
			}
			captured = rest[:len(rest)-len(suffix)]
			score = len(prefix)
		}
		if score > bestScore {
			bestScore = score
			repl := targets[0]
			if star >= 0 {
				repl = strings.Replace(repl, "*", captured, 1)
			}
			best = repl
		}
	}
	if bestScore < 0 {
		return "", false
	}
	return best, true
}

// stripJSONC removes comments and trailing commas so encoding/json can read
// a tsconfig. Strings are left intact, including comment-like text inside them.
func stripJSONC(s string) string {
	var b strings.Builder
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			b.WriteByte(c)
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) {
				b.WriteByte('\n')
			}
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < len(s) {
				i++
			}
			continue
		}
		b.WriteByte(c)
	}
	out := b.String()
	// Trailing commas before a close. Applied outside strings: the comment
	// strip already copied string contents through unchanged, and a comma
	// inside a string is followed by more string, not by } or ].
	var c strings.Builder
	inStr = false
	esc = false
	for i := 0; i < len(out); i++ {
		ch := out[i]
		if inStr {
			c.WriteByte(ch)
			if esc {
				esc = false
				continue
			}
			if ch == '\\' {
				esc = true
				continue
			}
			if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			c.WriteByte(ch)
			continue
		}
		if ch == ',' {
			j := i + 1
			for j < len(out) && (out[j] == ' ' || out[j] == '\t' || out[j] == '\n' || out[j] == '\r') {
				j++
			}
			if j < len(out) && (out[j] == '}' || out[j] == ']') {
				continue
			}
		}
		c.WriteByte(ch)
	}
	return c.String()
}
