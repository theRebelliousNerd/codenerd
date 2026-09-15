package mangle

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCorpusGate_AllNonAdversarialMangleParses walks every .mg file in the
// repo and requires it to parse with the real Mangle parser, except for
// paths that are invalid by contract:
//
//   - **/mangle-adversarial/** — the stress-tester adversarial suite; its
//     README defines it as "invalid Mangle code patterns" by design.
//   - **/.nerd/** and .nerd/** — runtime state, crash dumps
//     (debug_program_ERROR.mg), and snapshots, never repo content.
//
// A floor on the checked count keeps a broken walk from passing silently.
// The floor counts only files present on a fresh clone (tracked production
// corpus); gitignored skill mirrors add to it when present.
func TestCorpusGate_AllNonAdversarialMangleParses(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".mg") {
			files = append(files, p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	const minChecked = 100 // tracked production corpus alone exceeds this
	checked := 0
	for _, p := range files {
		rel, _ := filepath.Rel(root, p)
		slash := filepath.ToSlash(rel)
		if isExcludedCorpusPath(slash) {
			continue
		}
		checked++
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: read: %v", rel, err)
			continue
		}
		if _, err := ParseUnit(strings.NewReader(string(data))); err != nil {
			t.Errorf("%s: does not parse: %v", rel, err)
		}
	}
	t.Logf("corpus gate: checked=%d total=%d", checked, len(files))
	if checked < minChecked {
		t.Errorf("checked only %d files, want at least %d (walk broken?)", checked, minChecked)
	}
}

// TestCorpusGate_SyntacticFixturesStillFail pins the other direction: files
// under mangle-adversarial/syntactic/ exist to demonstrate syntax violations,
// so each one MUST fail to parse. If one starts parsing, either the fixture
// lost its teeth or the grammar changed — both demand attention.
func TestCorpusGate_SyntacticFixturesStillFail(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []string
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".mg") &&
			strings.Contains(filepath.ToSlash(p), "mangle-adversarial/syntactic/") {
			fixtures = append(fixtures, p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no syntactic fixtures found (walk broken?)")
	}
	for _, p := range fixtures {
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: read: %v", rel, err)
			continue
		}
		if _, err := ParseUnit(strings.NewReader(string(data))); err == nil {
			t.Errorf("%s: parses clean but is supposed to demonstrate a syntax violation", rel)
		}
	}
	t.Logf("syntactic fixtures pinned: %d", len(fixtures))
}

func isExcludedCorpusPath(slashRel string) bool {
	return strings.Contains(slashRel, "mangle-adversarial/") ||
		strings.Contains(slashRel, "/.nerd/") ||
		strings.HasPrefix(slashRel, ".nerd/")
}
