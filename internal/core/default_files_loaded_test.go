package core

import (
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// readByName are loaded by a path of their own: learned.mg is the seed of the
// learned-rules file (kernel_init.go).
var readByName = map[string]bool{"learned.mg": true}

// onlyComments reports a file with nothing to load: documentation of a schema.
func onlyComments(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "#") {
			return false
		}
	}
	return true
}

// orphanMangleFiles are the .mg files under defaults/ in fsys that nothing in
// loaded reads and that have rules to lose.
func orphanMangleFiles(fsys fs.FS, loaded map[string]bool) ([]string, error) {
	var orphans []string
	err := fs.WalkDir(fsys, "defaults", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".mg" {
			return err
		}
		rel := strings.TrimPrefix(p, "defaults/")
		if loaded[rel] || readByName[rel] {
			return nil
		}
		if data, rerr := fs.ReadFile(fsys, p); rerr == nil && onlyComments(data) {
			return nil
		}
		orphans = append(orphans, rel)
		return nil
	})
	sort.Strings(orphans)
	return orphans, err
}

func TestEveryEmbeddedMangleFileIsLoaded(t *testing.T) {
	loaded := map[string]bool{"schemas.mg": true}
	for _, f := range defaultSchemaFiles {
		loaded[f] = true
	}
	files, err := DefaultPolicyFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		loaded[f] = true
	}
	for _, f := range defaultIntentSchemaFiles {
		loaded[f] = true
	}
	orphans, err := orphanMangleFiles(coreLogic, loaded)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range orphans {
		t.Errorf("defaults/%s is embedded but no loader reads it: its rules never fire", o)
	}
}

func TestOrphanMangleFiles_FindsARuleFileNothingLoads(t *testing.T) {
	fsys := fstest.MapFS{
		"defaults/policy/loaded.mg":  {Data: []byte("a(X) :- b(X).")},
		"defaults/stray_rules.mg":    {Data: []byte("# note\nc(X) :- d(X).\n")},
		"defaults/schema/notes.mg":   {Data: []byte("# only documentation\n\n# of a schema\n")},
		"defaults/learned.mg":        {Data: []byte("x(1).")},
		"defaults/policy/readme.txt": {Data: []byte("not mangle")},
	}
	got, err := orphanMangleFiles(fsys, map[string]bool{"policy/loaded.mg": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "stray_rules.mg" {
		t.Fatalf("orphans = %v, want [stray_rules.mg]: a loaded file, a comment-only file and the learned seed are not orphans", got)
	}
}
