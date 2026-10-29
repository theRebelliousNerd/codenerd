package prompt

import (
	"errors"
	"strings"
	"testing"
)

// FuzzParsePromptAtomYAML is the prompt corpus's first fuzz gate (gap G7): the
// strict atom parser is the one decoder every route shares (validator,
// embedded corpus, synchronizer), so it must never panic on hostile YAML, and
// whatever it accepts must satisfy the invariants the rest of the compiler
// assumes. Under `go test` the seed corpus runs as regression cases.
func FuzzParsePromptAtomYAML(f *testing.F) {
	for _, seed := range []string{
		"- id: a/b\n  category: identity\n  content: hello\n",
		"- id: a/b\n  category: identity\n  content_file: missing.md\n",
		"- id: a/b\n  category: nonsense\n  content: x\n",
		"- id: ''\n  category: identity\n  content: x\n",
		"- id: a/b\n  category: identity\n  content: x\n  unknown_field: 1\n",
		"- id: a/b\n  category: identity\n  content: x\n- id: a/b\n  category: identity\n  content: y\n",
		"not: [a list",
		"",
		"- id: a/b\n  category: language\n  languages: [/go]\n  priority: 999999999999\n  content: x\n",
		"- &x id: a\n  category: *x\n",
	} {
		f.Add(seed)
	}
	noContent := func(string, string) ([]byte, error) { return nil, errors.New("no content files in fuzzing") }
	f.Fuzz(func(t *testing.T, data string) {
		parsed, _, err := ParsePromptAtomYAML([]byte(data), "fuzz.yaml", noContent)
		if err != nil {
			return
		}
		seen := make(map[string]bool, len(parsed))
		for _, record := range parsed {
			atom := record.Atom
			if atom == nil {
				t.Fatalf("accepted a record with no atom: %q", data)
			}
			if strings.TrimSpace(atom.ID) == "" {
				t.Fatalf("accepted an atom with an empty id: %q", data)
			}
			if seen[atom.ID] {
				t.Fatalf("accepted duplicate atom id %q: %q", atom.ID, data)
			}
			seen[atom.ID] = true
			if strings.TrimSpace(string(atom.Category)) == "" {
				t.Fatalf("accepted atom %q with no category: %q", atom.ID, data)
			}
		}
	})
}
