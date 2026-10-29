package core

import (
	"bufio"
	"path"
	"strings"
	"testing"
)

// TestIntentSchemaFiles_ShouldLoadEveryFileWithContent — defaults/policy is
// loaded by directory walk, defaults/schema by the hand-maintained list in
// intent_defaults.go. A file added to (or left behind in) schema/ is therefore
// silently unloaded: intent.mg, the pre-modularisation monolith, sat there
// with 184 fact lines for months, read by nothing at runtime but still an
// input to cmd/tools/corpus_builder. Every schema file that carries facts
// must be in the list; comment-only files are documentation and may stay.
func TestIntentSchemaFiles_ShouldLoadEveryFileWithContent(t *testing.T) {
	loaded := make(map[string]bool)
	for _, f := range DefaultIntentSchemaFiles() {
		loaded[path.Base(f)] = true
		if _, err := coreLogic.ReadFile("defaults/" + f); err != nil {
			t.Errorf("DefaultIntentSchemaFiles lists %s, which is not embedded: %v", f, err)
		}
	}

	entries, err := coreLogic.ReadDir("defaults/schema")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mg") {
			continue
		}
		if loaded[e.Name()] {
			continue
		}
		data, err := coreLogic.ReadFile("defaults/schema/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			t.Errorf("defaults/schema/%s carries content (%q) but is not in DefaultIntentSchemaFiles; nothing loads it at runtime", e.Name(), line)
			break
		}
	}
}
