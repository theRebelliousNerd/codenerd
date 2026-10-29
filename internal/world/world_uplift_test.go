package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// MapFileAs reads fsPath but must label EVERY fact with the canonical
// identity: deep scans run outside the workspace would otherwise store
// absolute paths for multilang dataflow facts while every sibling fact
// carries the canonical label (same file, two identities).

var upliftDataflowPredicates = map[string]bool{
	"assigns": true, "uses": true, "call_arg": true,
	"guards_block": true, "guards_return": true, "guard_dominates": true,
	"function_scope": true, "error_checked_block": true, "error_checked_return": true,
}

func writeUpliftFile(t *testing.T, name, content string) (fsPath, factPath string) {
	t.Helper()
	dir := t.TempDir()
	fsPath = filepath.Join(dir, name)
	if err := os.WriteFile(fsPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return fsPath, "rel/" + name
}

func assertCanonicalLabels(t *testing.T, facts []core.Fact, fsPath, factPath string, needDataflow bool) {
	t.Helper()
	seenFactPath := false
	seenDataflow := false
	for _, f := range facts {
		for _, a := range f.Args {
			s, ok := a.(string)
			if !ok {
				continue
			}
			if s == fsPath || strings.Contains(s, fsPath) {
				t.Errorf("predicate %s leaks absolute path %q", f.Predicate, s)
			}
			if s == factPath {
				seenFactPath = true
			}
		}
		if upliftDataflowPredicates[f.Predicate] {
			seenDataflow = true
		}
	}
	if len(facts) == 0 {
		t.Fatal("no facts mapped; pin is vacuous")
	}
	if !seenFactPath {
		t.Errorf("no fact carries the canonical label %q", factPath)
	}
	if needDataflow && !seenDataflow {
		t.Errorf("no dataflow facts mapped; relabel pin is vacuous (predicates: %v)", upliftDataflowPredicates)
	}
}

func TestMapFileAs_Python_LabelsDataflowCanonically(t *testing.T) {
	fsPath, factPath := writeUpliftFile(t, "mod.py", "def render(name):\n    title = name.strip()\n    print(title)\n    return title\n")
	c := NewCartographer()
	defer c.Close()
	facts, err := c.MapFileAs(fsPath, factPath)
	if err != nil {
		t.Fatalf("MapFileAs: %v", err)
	}
	assertCanonicalLabels(t, facts, fsPath, factPath, true)
}

func TestMapFileAs_Go_LabelsDataflowCanonically(t *testing.T) {
	fsPath, factPath := writeUpliftFile(t, "mod.go", "package mod\n\nfunc Render(name string) string {\n\ttitle := name\n\tprintln(title)\n\treturn title\n}\n")
	c := NewCartographer()
	defer c.Close()
	facts, err := c.MapFileAs(fsPath, factPath)
	if err != nil {
		t.Fatalf("MapFileAs: %v", err)
	}
	assertCanonicalLabels(t, facts, fsPath, factPath, true)
}
