package session

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/experts"
	"codenerd/internal/types"
)

// reviewCounter answers every review with NO FINDINGS and keeps who was asked.
type reviewCounter struct {
	mu    sync.Mutex
	asked []string
}

func (c *reviewCounter) SpawnConsultation(_ context.Context, agent, _ string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asked = append(c.asked, agent)
	return "NO FINDINGS", nil
}

const touchBefore = "package main\n\nimport (\n\t\"strings\"\n\t\"unicode\"\n)\n\nfunc Label(s string) string {\n\treturn strings.ToUpper(s)\n}\n\nfunc Letter(r rune) bool {\n\treturn unicode.IsLetter(r)\n}\n"

// The turn owes two critics, one per import of the file, but its change edits
// only the line that calls strings: the strings critic reviews, the unicode
// critic is not asked. A change that touches nothing of either still has the
// review of a critic recorded with no technology.
func TestExpertCritique_TheChangeDealsTheReview(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	ws := e.config.WorkspaceRoot
	after := strings.Replace(touchBefore, "strings.ToUpper(s)", "strings.ToUpper(strings.TrimSpace(s))", 1)
	if err := os.WriteFile(filepath.Join(ws, "main.go"), []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.kernel.AssertBatch([]types.Fact{
		{Predicate: "technology_import_exact", Args: []any{"strings", types.MangleAtom("/textkit")}},
		{Predicate: "technology_import_exact", Args: []any{"unicode", types.MangleAtom("/unikit")}},
	}); err != nil {
		t.Fatal(err)
	}
	reviews := &reviewCounter{}
	e.SetExpertConsultant(reviews)
	result := mutationResult()
	result.WrittenPaths = []string{"main.go"}
	result.PreWriteContents = map[string]PreImage{"main.go": existed(touchBefore)}
	e.recordExpertCritics(result, experts.Selection{Needs: []experts.Need{
		{Agent: "TextKitExpert", Technologies: []string{"textkit"}, Files: []string{"main.go"}, Critique: true},
		{Agent: "UniKitExpert", Technologies: []string{"unikit"}, Files: []string{"main.go"}, Critique: true},
		{Agent: "LegacyExpert", Files: []string{"main.go"}, Critique: true},
	}})
	if _, err := e.reviewWithShardExperts(context.Background(), nil, "", nil, nil, nil, result); err != nil {
		t.Fatal(err)
	}
	slices.Sort(reviews.asked)
	if !slices.Equal(reviews.asked, []string{"legacyexpert", "textkitexpert"}) {
		t.Fatalf("reviewed by %v, want the strings critic and the critic with no technology, not the unicode one", reviews.asked)
	}
}

func TestGoChangeImports(t *testing.T) {
	t.Parallel()
	got := func(before, after string) []string {
		out := goChangeImports(before, after)
		slices.Sort(out)
		return out
	}
	edited := strings.Replace(touchBefore, "strings.ToUpper(s)", "strings.ToLower(s)", 1)
	if g := got(touchBefore, edited); !slices.Equal(g, []string{"strings"}) {
		t.Errorf("an edited strings call touches %v, want [strings]", g)
	}
	removed := strings.Replace(touchBefore, "\treturn unicode.IsLetter(r)\n", "\treturn r != 0\n", 1)
	if g := got(touchBefore, removed); !slices.Equal(g, []string{"unicode"}) {
		t.Errorf("a removed unicode call touches %v, want [unicode]", g)
	}
	added := strings.Replace(touchBefore, "\t\"unicode\"\n", "\t\"unicode\"\n\t\"sort\"\n", 1) + "\nvar _ = sort.Ints\n"
	if g := got(touchBefore, added); !slices.Contains(g, "sort") {
		t.Errorf("an added import is not touched: %v", g)
	}
	unknown := strings.Replace(touchBefore, "return strings.ToUpper(s)", "return other.Name(s)", 1)
	if g := got(touchBefore, unknown); !slices.Contains(g, "unicode") || !slices.Contains(g, "strings") {
		t.Errorf("a selector no import accounts for must touch every implicit import: %v", g)
	}
	for path, want := range map[string]string{"gopkg.in/yaml.v3": "yaml", "github.com/x/mangle-go/ast": "ast", "example.com/lib/v2": "lib", "github.com/charmbracelet/bubbletea": "bubbletea"} {
		if g := importDefaultName(path); g != want {
			t.Errorf("importDefaultName(%q) = %q, want %q", path, g, want)
		}
	}
}
