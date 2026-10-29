package testfacts

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"codenerd/internal/types"
)

// locator turns a path the toolchain printed into the file identity the
// world model joins on: workspace-relative, slash-separated, cleaned
// (types.CanonicalPath). dir is the directory `go test` ran in, which for
// the gates is the workspace root.
//
// Go 1.26 prints three spellings for one file (observed 2026-09-28):
//
//   - a test log is the basename only (`x_test.go:5: boom`); testing.decorate
//     truncates at the last separator. The directory is the package's, from
//     the event's import path plus the module path in dir's go.mod.
//   - a compiler diagnostic is relative to dir (`.\bf_test.go:5:33:` at the
//     module root, `sub\bf_test.go:5:33:` when the package is a subdirectory).
//     Joining the package directory onto that path doubles the subdirectory.
//   - a panic frame is absolute (`C:/work/my module/x_test.go:5`), and the
//     space is part of the path.
//
// A file that is not under dir keeps its absolute slash form. Rewriting it
// to a relative path would give it a workspace identity it does not have,
// and the join would hit a different file.
type locator struct {
	workspace  string
	moduleRoot string
	modulePath string
}

func newLocator(dir string) locator {
	root, mod, ok := readModule(dir)
	if !ok {
		return locator{workspace: dir}
	}
	return locator{workspace: dir, moduleRoot: root, modulePath: mod}
}

// file canonicalises one reported path. packageBase is true for a test-log
// line, whose path is a basename relative to the package directory, and
// false for a compiler line or a stack frame.
func (l locator) file(importPath, reported string, packageBase bool) string {
	s := strings.ReplaceAll(strings.TrimSpace(reported), `\`, "/")
	if s == "" {
		return ""
	}
	if absSlash(s) {
		if l.workspace == "" {
			return path.Clean(s)
		}
		// Outside the workspace this returns the absolute slash path.
		return types.CanonicalPath(l.workspace, s)
	}
	rel := path.Clean(s)
	if packageBase && !strings.Contains(s, "/") {
		if pkg, ok := l.packageDir(importPath); ok && pkg != "." {
			rel = path.Clean(path.Join(pkg, rel))
		}
	}
	if l.workspace == "" {
		return rel
	}
	// A diagnostic that climbs out of dir (`../pkg/mod/...`) is resolved
	// and then kept absolute when it lands outside the workspace.
	if strings.HasPrefix(rel, "../") || rel == ".." {
		abs := filepath.Join(l.workspace, filepath.FromSlash(rel))
		return types.CanonicalPath(l.workspace, strings.ReplaceAll(abs, `\`, "/"))
	}
	return types.CanonicalPath(l.workspace, rel)
}

// packageDir is the package's directory relative to the workspace, or "."
// when the package is the module root. A package whose import path is not
// under the module (a dependency) has no workspace directory; the caller
// then keeps the basename, which is all the toolchain printed.
func (l locator) packageDir(importPath string) (string, bool) {
	if l.modulePath == "" || l.workspace == "" || importPath == "" {
		return "", false
	}
	var relMod string
	switch {
	case importPath == l.modulePath:
		relMod = "."
	case strings.HasPrefix(importPath, l.modulePath+"/"):
		relMod = importPath[len(l.modulePath)+1:]
	default:
		return "", false
	}
	abs := l.moduleRoot
	if relMod != "." {
		abs = filepath.Join(l.moduleRoot, filepath.FromSlash(relMod))
	}
	rel := types.CanonicalPath(l.workspace, abs)
	if rel == "" || absSlash(rel) || strings.HasPrefix(rel, "../") || rel == ".." {
		return "", false
	}
	return rel, true
}

// readModule finds the go.mod that covers dir, the way the go command does:
// dir itself, then its parents. A go.mod that does not name a module stops
// the walk; climbing into a parent module would anchor files to the wrong root.
func readModule(dir string) (root, module string, ok bool) {
	if dir == "" {
		return "", "", false
	}
	cur := dir
	for {
		data, err := os.ReadFile(filepath.Join(cur, "go.mod"))
		if err == nil {
			if mp := parseModuleLine(data); mp != "" {
				return cur, mp, true
			}
			return "", "", false
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", "", false
		}
		cur = parent
	}
}

// parseModuleLine reads a go.mod module path, including the parenthesised
// form. The first non-comment statement has to be the module line.
func parseModuleLine(data []byte) string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripGoComment(lines[i]))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "module" {
			return ""
		}
		if len(fields) >= 2 && fields[1] != "(" {
			return strings.Trim(fields[1], `"`)
		}
		for j := i + 1; j < len(lines); j++ {
			inner := strings.TrimSpace(stripGoComment(lines[j]))
			if inner == "" || inner == "(" {
				continue
			}
			if inner == ")" {
				return ""
			}
			return strings.Trim(strings.Fields(inner)[0], `"`)
		}
		return ""
	}
	return ""
}

func stripGoComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

// absSlash reports a slash-normalised absolute path on either POSIX or
// Windows. It matches the rule in types.CanonicalPath; that helper is
// unexported, and a basename must not be sent down the absolute branch.
func absSlash(p string) bool {
	if strings.HasPrefix(p, "/") {
		return true
	}
	return len(p) >= 3 && p[1] == ':' && p[2] == '/'
}
