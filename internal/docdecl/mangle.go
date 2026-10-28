package docdecl

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MangleDecls returns predicate names introduced by a Decl line. A Decl
// inside a `#` comment, a `//` comment, or a block comment is not a
// declaration — the corpus is full of notes that mention Decl without
// making one.
func MangleDecls(src string) map[string]bool {
	names := map[string]bool{}
	inBlock := false
	for _, line := range SplitLines(src) {
		t := line
		if inBlock {
			if i := strings.Index(t, "*/"); i >= 0 {
				inBlock = false
				t = t[i+2:]
			} else {
				continue
			}
		}
		for {
			i := strings.Index(t, "/*")
			if i < 0 {
				break
			}
			j := strings.Index(t[i+2:], "*/")
			if j < 0 {
				inBlock = true
				t = t[:i]
				break
			}
			t = t[:i] + " " + t[i+2+j+2:]
		}
		if i := strings.Index(t, "#"); i >= 0 {
			t = t[:i]
		}
		if i := strings.Index(t, "//"); i >= 0 {
			t = t[:i]
		}
		t = strings.TrimSpace(t)
		if !strings.HasPrefix(t, "Decl") {
			continue
		}
		rest := t[len("Decl"):]
		if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		if name := predIdent(strings.TrimSpace(rest)); name != "" {
			names[name] = true
		}
	}
	return names
}

// LoadPredicates collects Decl names from the .mg corpus under root. Crash
// dumps under dot-directories (cmd/nerd/chat/.nerd/debug/debug_program_ERROR.mg
// re-declares the kernel) and testdata fixtures are not the corpus: counting
// them would make a predicate that has no real Decl look declared.
func LoadPredicates(root string) (map[string]bool, error) {
	names := map[string]bool{}
	root = filepath.Clean(root)
	first := true
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !first && SkipCorpusDir(d.Name()) {
				return filepath.SkipDir
			}
			first = false
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".mg") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for n := range MangleDecls(string(b)) {
			names[n] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}

// SkipCorpusDir reports directories that are not part of the source corpus.
func SkipCorpusDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor" || name == "node_modules"
}

func predIdent(s string) string {
	if s == "" || !isIdentStart(s[0]) {
		return ""
	}
	i := 1
	for i < len(s) && isIdentCont(s[i]) {
		i++
	}
	return s[:i]
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isIdentCont(b byte) bool { return isIdentStart(b) || (b >= '0' && b <= '9') }

// SplitLines splits text into lines. A leading byte-order mark is dropped,
// CRLF and lone CR are line breaks, and one trailing newline does not make
// an extra empty line.
func SplitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "\uFEFF")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}
