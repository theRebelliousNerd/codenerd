package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const projectionFixture = `# orientation projection
Decl oriented_head(Head) bound [/string].
Decl oriented_document(Path, Digest) bound [/string, /string].
Decl orient_role_pending(Path, Rank) bound [/string, /number].
oriented_head("abc123").
oriented_document("Docs/a.md", "d1").
oriented_document("Docs/quote \"b\".md", "d2\\n").
orient_role_pending("Docs/a.md", 3).
orient_role_pending("Docs/c.md", -17).
`

func writeProjection(t *testing.T, workspace, text string) string {
	t.Helper()
	dir := filepath.Join(workspace, ".nerd", "orientation")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "orientation.mg")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A parse remembered across processes is that parse: the cache's answer for
// the same text equals the parser's, a changed text is parsed again, and an
// unreadable cache costs only the parse.
func TestParsedProjectionCacheAnswersAsTheParser(t *testing.T) {
	ws := t.TempDir()
	path := writeProjection(t, ws, projectionFixture)
	wantDecls, wantFacts, err := parseProjectionText(path, []byte(projectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	parses := 0
	parse := func(text string) func() (string, []Fact, error) {
		return func() (string, []Fact, error) {
			parses++
			return parseProjectionText(path, []byte(text))
		}
	}
	cache := projectionCachePath(path)
	for i := 0; i < 2; i++ {
		decls, facts, err := cachedProjectionParse(cache, []byte(projectionFixture), parse(projectionFixture))
		if err != nil {
			t.Fatal(err)
		}
		if decls != wantDecls || !reflect.DeepEqual(facts, wantFacts) {
			t.Fatalf("pass %d: cache answered %q %v, parser %q %v", i, decls, facts, wantDecls, wantFacts)
		}
	}
	if parses != 1 {
		t.Fatalf("the same text was parsed %d times, want once", parses)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatalf("no cache at %s: %v", cache, err)
	}

	changed := projectionFixture + `oriented_document("Docs/new.md", "d3").` + "\n"
	_, facts, err := cachedProjectionParse(cache, []byte(changed), parse(changed))
	if err != nil {
		t.Fatal(err)
	}
	if parses != 2 || len(facts) != len(wantFacts)+1 {
		t.Fatalf("a changed text was not parsed again (parses %d, rows %d)", parses, len(facts))
	}

	if err := os.WriteFile(cache, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, facts, err := cachedProjectionParse(cache, []byte(changed), parse(changed)); err != nil || len(facts) != len(wantFacts)+1 || parses != 3 {
		t.Fatalf("an unreadable cache: rows %d, parses %d, err %v", len(facts), parses, err)
	}
}

// A kernel booting on the projection gets the same rows whether its process
// parsed the projection or read the remembered parse.
func TestKernelBootsTheSameRowsFromTheRememberedParse(t *testing.T) {
	ws := t.TempDir()
	path := writeProjection(t, ws, projectionFixture)
	_, parsed, err := parseProjectionText(path, []byte(projectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := cachedProjectionParse(projectionCachePath(path), []byte(projectionFixture), func() (string, []Fact, error) {
		return parseProjectionText(path, []byte(projectionFixture))
	}); err != nil {
		t.Fatal(err)
	}
	orientationProjectionCacheMu.Lock()
	delete(orientationProjectionCache, path) // a new process: nothing in memory
	orientationProjectionCacheMu.Unlock()
	k, err := NewRealKernelWithWorkspace(ws)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range parsed {
		rows, err := k.Query(want.Predicate)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range rows {
			if reflect.DeepEqual(r.Args, want.Args) {
				found = true
			}
		}
		if !found {
			t.Fatalf("booted kernel lacks %v (has %v)", want, rows)
		}
	}
}
