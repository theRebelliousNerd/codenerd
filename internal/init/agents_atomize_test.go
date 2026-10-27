package init

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// longSectionedMarkdown mimics a planner-written domain body: several
// heading sections, each itself longer than the atom cap.
func longSectionedMarkdown() string {
	var b strings.Builder
	for s := 1; s <= 4; s++ {
		fmt.Fprintf(&b, "## Concept Number %d\n\n", s)
		for p := 0; p < 6; p++ {
			b.WriteString(strings.Repeat(fmt.Sprintf("concept %d paragraph %d explains a concrete pattern with pitfalls. ", s, p), 6))
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func TestBuildPromptsYAML_LongBodiesAreAtomizedUnderCap(t *testing.T) {
	long := longSectionedMarkdown()
	require.Greater(t, utf8.RuneCountInString(long), 4*maxAgentAtomContentChars)

	yml := buildPromptsYAML("goexpert", "GoExpert", "Go specialist", "    - concurrency", "concurrency", long, long)
	require.NoError(t, validatePromptsYAML([]byte(yml), "goexpert"))

	var atoms []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(yml), &atoms))
	ids := make([]string, len(atoms))
	for i, a := range atoms {
		ids[i] = a.ID
		assert.LessOrEqual(t, utf8.RuneCountInString(a.Content), maxAgentAtomContentChars, "atom %s over cap", a.ID)
	}
	// Base ids survive as the first piece; sub-atoms hang under them.
	assert.Contains(t, ids, "goexpert/identity")
	assert.Contains(t, ids, "goexpert/methodology")
	assert.Contains(t, ids, "goexpert/domain")
	assert.Greater(t, len(ids), 3)
	for _, title := range []string{"Concept Number 1", "Concept Number 2", "Concept Number 3", "Concept Number 4"} {
		assert.Contains(t, yml, title, "section %q lost in atomization", title)
	}
}

func TestBuildPromptsYAML_ShortBodiesStayThreeAtoms(t *testing.T) {
	yml := buildPromptsYAML("goexpert", "GoExpert", "Go specialist", "    - concurrency", "concurrency",
		"## Method\nWork carefully.", "## Domain\nKnow things.")
	require.NoError(t, validatePromptsYAML([]byte(yml), "goexpert"))
	assert.Equal(t,
		[]string{"goexpert/identity", "goexpert/methodology", "goexpert/domain"},
		parseAtomIDs(t, []byte(yml)))
}

func TestValidatePromptsYAML_RejectsOversizeAtom(t *testing.T) {
	yml := buildPromptsYAML("goexpert", "GoExpert", "Go specialist", "    - concurrency", "concurrency",
		"short method", "short domain")
	bloated := strings.Replace(yml, "short domain", strings.Repeat("x", maxAgentAtomContentChars+1), 1)
	err := validatePromptsYAML([]byte(bloated), "goexpert")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "over the")
	assert.Contains(t, err.Error(), "goexpert/domain")
}

func TestResplitPromptsYAML_SplitsOversizeAndPreservesFields(t *testing.T) {
	long := longSectionedMarkdown()
	original := fmt.Sprintf(`- id: "goexpert/identity"
  category: "identity"
  priority: 100
  is_mandatory: true
  content: |
    You are GoExpert.
- id: "goexpert/methodology"
  category: "methodology"
  priority: 80
  is_mandatory: false
  depends_on: ["goexpert/identity"]
  content: |
    Work carefully.
- id: "goexpert/domain"
  category: "domain"
  priority: 70
  is_mandatory: false
  depends_on: ["goexpert/identity", "goexpert/methodology"]
  description: "Domain knowledge"
  intent_verbs: ["/fix"]
  content_concise: |
    Short form.
  content: |
%s`, indentForYAMLBlock(long, "    "))

	fixed, changed, err := resplitPromptsYAML([]byte(original))
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, validatePromptsYAML(fixed, "goexpert"))

	text := string(fixed)
	// Unknown fields survive on the pieces; the concise variant stays only
	// on the base atom it describes.
	assert.Contains(t, text, "intent_verbs")
	assert.Equal(t, 1, strings.Count(text, "content_concise"))
	for _, title := range []string{"Concept Number 1", "Concept Number 4"} {
		assert.Contains(t, text, title)
	}

	// Nothing oversize => unchanged.
	_, changed, err = resplitPromptsYAML(fixed)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestSplitMarkdownSections_UnbrokenTextStillSplits(t *testing.T) {
	body := strings.Repeat("word ", 2000) // 10000 chars, no headings or paragraphs
	pieces := splitMarkdownSections(body, maxAgentAtomContentChars)
	require.Greater(t, len(pieces), 1)
	total := 0
	for _, p := range pieces {
		assert.NotEmpty(t, strings.TrimSpace(p.Body))
		assert.LessOrEqual(t, utf8.RuneCountInString(p.Body), maxAgentAtomContentChars)
		total += len(strings.Fields(p.Body))
	}
	assert.Equal(t, 2000, total, "no words may be lost or duplicated")
}
