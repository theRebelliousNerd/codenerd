package retrieval

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// TIER 3: TYPESCRIPT / JAVASCRIPT AND RUST IMPORT EXPANDERS
// =============================================================================
//
// Tier 3 followed Go and Python imports only, so on a TypeScript or Rust
// workspace the import tier was empty and its budget went unspent while the
// modules an issue's files pull in sat one edge away. Only edges that land in
// the workspace are followed, for the reason the Go expander gives: a package
// from npm or crates.io cannot be edited, which is what this context is for.

// importScanLimit bounds how much of a file the regex expanders read: imports
// sit at the top, and a minified bundle must not be scanned line by line.
const importScanLimit = 256 << 10

// jsSpecifierPattern matches the module specifier of an ES import or export
// (`import x from "./a"`, `import "./a"`, `export * from "./a"`), a dynamic
// `import("./a")` and a CommonJS `require("./a")`.
var jsSpecifierPattern = regexp.MustCompile(`(?:\bfrom\s*|\bimport\s*\(?\s*|\brequire\s*\(\s*)["']([^"']+)["']`)

// jsResolveExtensions are tried, in order, for a specifier without one.
var jsResolveExtensions = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx", ".mjs", ".cjs"}

func isJSLike(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return true
	}
	return false
}

// jsImportNeighbors resolves a TS/JS file's relative specifiers to workspace
// files. Bare specifiers ("react", "@scope/pkg") name node_modules packages and
// are not followed; path aliases from tsconfig are not resolved either.
func (b *TieredContextBuilder) jsImportNeighbors(filePath string) []string {
	var out []string
	seen := map[string]bool{filePath: true}
	for _, spec := range scanSubmatches(filePath, jsSpecifierPattern) {
		if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
			continue
		}
		if resolved := b.resolveJSSpecifier(filepath.Dir(filePath), spec); resolved != "" && !seen[resolved] {
			seen[resolved] = true
			out = append(out, resolved)
		}
	}
	return out
}

// resolveJSSpecifier applies the resolution a bundler or tsc applies to a
// relative specifier: the path as written, then with each extension, then as
// a directory with an index file.
func (b *TieredContextBuilder) resolveJSSpecifier(dir, spec string) string {
	base := filepath.Join(dir, filepath.FromSlash(spec))
	if !b.insideWorkspace(base) {
		return ""
	}
	if isRegularFile(base) {
		return base
	}
	// A TS source imported with a .js suffix (the ESM convention) lives in a
	// .ts file of the same stem.
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	candidates := make([]string, 0, 2*len(jsResolveExtensions))
	for _, ext := range jsResolveExtensions {
		candidates = append(candidates, base+ext)
	}
	if stem != base {
		for _, ext := range jsResolveExtensions {
			candidates = append(candidates, stem+ext)
		}
	}
	for _, ext := range jsResolveExtensions {
		candidates = append(candidates, filepath.Join(base, "index"+ext))
	}
	for _, c := range candidates {
		if isRegularFile(c) {
			return c
		}
	}
	return ""
}

// rustModPattern matches an out-of-line module declaration, `mod name;`
// (optionally pub / pub(crate)); an inline `mod name { ... }` names no file.
var rustModPattern = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?mod\s+([A-Za-z_][A-Za-z0-9_]*)\s*;`)

// rustUsePattern matches a `use crate::a::b` or `use super::a` path.
var rustUsePattern = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?use\s+((?:crate|super|self)(?:::[A-Za-z_][A-Za-z0-9_]*)+)`)

// rustImportNeighbors resolves a Rust file's `mod` declarations and its
// `use crate::` / `use super::` paths to the files that define those modules.
// External crates are not followed.
func (b *TieredContextBuilder) rustImportNeighbors(filePath string) []string {
	var out []string
	seen := map[string]bool{filePath: true}
	add := func(p string) {
		if p != "" && !seen[p] && b.insideWorkspace(p) {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, name := range scanSubmatches(filePath, rustModPattern) {
		add(rustModuleFile(rustChildDir(filePath), name))
	}
	for _, path := range scanSubmatches(filePath, rustUsePattern) {
		add(b.resolveRustUse(filePath, path))
	}
	return out
}

// rustChildDir is where the files of a module's children live: beside a
// crate root or a mod.rs, in the directory named after the file otherwise.
func rustChildDir(filePath string) string {
	dir := filepath.Dir(filePath)
	switch filepath.Base(filePath) {
	case "lib.rs", "main.rs", "mod.rs":
		return dir
	}
	return filepath.Join(dir, strings.TrimSuffix(filepath.Base(filePath), ".rs"))
}

// rustModuleFile is the file defining module name under dir: name.rs or
// name/mod.rs.
func rustModuleFile(dir, name string) string {
	for _, c := range []string{filepath.Join(dir, name+".rs"), filepath.Join(dir, name, "mod.rs")} {
		if isRegularFile(c) {
			return c
		}
	}
	return ""
}

// resolveRustUse maps a use path onto the deepest module file it names:
// `crate::a::b::Item` is a/b.rs when it exists, else a.rs (Item and b being
// items of a).
func (b *TieredContextBuilder) resolveRustUse(filePath, usePath string) string {
	segments := strings.Split(usePath, "::")
	var dir string
	switch segments[0] {
	case "crate":
		root := rustCrateRoot(filePath)
		if root == "" {
			return ""
		}
		dir = root
	case "self":
		dir = rustChildDir(filePath)
	case "super":
		dir = filepath.Dir(rustChildDir(filePath))
		for len(segments) > 1 && segments[1] == "super" {
			dir = filepath.Dir(dir)
			segments = segments[1:]
		}
	}
	best := ""
	for _, name := range segments[1:] {
		f := rustModuleFile(dir, name)
		if f == "" {
			break
		}
		best = f
		dir = filepath.Join(dir, name)
	}
	return best
}

// rustCrateRoot is the directory of the crate root (lib.rs or main.rs) that
// owns filePath: the nearest ancestor src/ holding one, the way cargo lays a
// crate out.
func rustCrateRoot(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		if filepath.Base(dir) == "src" && (isRegularFile(filepath.Join(dir, "lib.rs")) || isRegularFile(filepath.Join(dir, "main.rs"))) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// scanSubmatches returns the first capture group of every line of the head of
// path that pattern matches.
func scanSubmatches(path string, pattern *regexp.Regexp) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	scanner := bufio.NewScanner(io.LimitReader(f, importScanLimit))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		for _, m := range pattern.FindAllStringSubmatch(scanner.Text(), -1) {
			if len(m) > 1 && m[1] != "" {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// insideWorkspace reports whether path lies under the builder's workspace, so
// a `../../..` specifier cannot pull a file from outside it.
func (b *TieredContextBuilder) insideWorkspace(path string) bool {
	root, err := filepath.Abs(b.workDir)
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, abs)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
