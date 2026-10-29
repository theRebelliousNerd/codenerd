package init

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// One concern per atom: two small headings that fit together in one cap
// must still become two atoms when perHeading is set (the generator path).
func TestSplitMarkdownSections_PerHeadingKeepsOneConcernPerAtom(t *testing.T) {
	body := "## Retry With Backoff\n\nRetry failed writes with backoff.\n\n## Never Retry Validation\n\nDo not retry validation errors."
	require.Less(t, utf8.RuneCountInString(body), testAtomCap)

	atomic := splitMarkdownSections(body, testAtomCap, true)
	require.Len(t, atomic, 2, "perHeading must keep one concern per atom")
	assert.Contains(t, atomic[0].Body, "Retry With Backoff")
	assert.Contains(t, atomic[1].Body, "Never Retry Validation")

	packed := splitMarkdownSections(body, testAtomCap, false)
	require.Len(t, packed, 1, "non-perHeading packs a body within cap into one atom")
}

// The generator must emit small atoms: every methodology/domain heading is
// its own atom and nothing exceeds the cap.
func TestAgentPromptAtoms_EachHeadingIsItsOwnSmallAtom(t *testing.T) {
	methodology := "## Retry With Backoff\n\nRetry failed writes with backoff.\n\n## Isolate Side Effects\n\nKeep dry-run pure."
	domain := "## Lease Expiry\n\nLeases expire after 30s.\n\n## Quorum Loss\n\nQuorum loss rejects writes."

	atoms := agentPromptAtoms("goexpert", "GoExpert", "Go specialist", "    - concurrency", "concurrency", methodology, domain, testAtomCap)
	require.NoError(t, validatePromptsYAML([]byte(renderPromptsYAML("GoExpert", atoms)), "goexpert", testAtomCap))

	ids := make([]string, 0, len(atoms))
	for _, a := range atoms {
		ids = append(ids, a.ID)
		assert.LessOrEqual(t, utf8.RuneCountInString(a.Content), testAtomCap, "atom %s over cap", a.ID)
		assert.NotEmpty(t, strings.TrimSpace(a.Content))
	}
	assert.Contains(t, ids, "goexpert/identity")
	assert.Contains(t, ids, "goexpert/methodology")
	assert.Contains(t, ids, "goexpert/domain")
	// 1 identity + 2 methodology + 2 domain = 5 atoms.
	assert.Len(t, ids, 5)
	seen := map[string]bool{}
	for _, id := range ids {
		assert.False(t, seen[id], "duplicate atom id %q", id)
		seen[id] = true
	}
}

// Remediation must also be heading-aware: an oversize atom with two headings
// splits at the heading boundary into small atoms that keep unknown fields.
func TestResplitPromptsYAML_SplitsOversizeAtHeadingsIntoSmallAtoms(t *testing.T) {
	half := strings.Repeat("concrete api detail. ", 40) // ~840 chars
	oversize := "## Lease Expiry\n\n" + half + "\n\n## Quorum Loss\n\n" + half
	require.Greater(t, utf8.RuneCountInString(oversize), testAtomCap)

	original := "- id: \"goexpert/identity\"\n" +
		"  category: \"identity\"\n" +
		"  is_mandatory: true\n" +
		"  content: |\n" +
		indentForYAMLBlock("You are GoExpert.", "    ") + "\n" +
		"- id: \"goexpert/domain\"\n" +
		"  category: \"domain\"\n" +
		"  subcategory: \"knowledge\"\n" +
		"  is_mandatory: false\n" +
		"  depends_on: [\"goexpert/identity\"]\n" +
		"  description: \"Domain knowledge\"\n" +
		"  intent_verbs: [\"/fix\"]\n" +
		"  content: |\n" +
		indentForYAMLBlock(oversize, "    ") + "\n"

	fixed, changed, err := resplitPromptsYAML([]byte(original), testAtomCap)
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, validatePromptsYAML(fixed, "goexpert", testAtomCap))

	var atoms []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
	}
	require.NoError(t, yaml.Unmarshal(fixed, &atoms))
	require.Len(t, atoms, 3, "identity plus two heading-split domain atoms")
	assert.Equal(t, "goexpert/identity", atoms[0].ID)
	assert.Equal(t, "goexpert/domain", atoms[1].ID, "first piece keeps the base id")
	for _, a := range atoms {
		assert.LessOrEqual(t, utf8.RuneCountInString(a.Content), testAtomCap, "atom %s over cap", a.ID)
	}
	text := string(fixed)
	assert.Contains(t, text, "Lease Expiry")
	assert.Contains(t, text, "Quorum Loss")
	assert.Contains(t, text, "intent_verbs", "remediation must preserve unknown fields")
}

// The generator contract is two-sided: the model instructions demand one
// section per atom with one concern each under a section limit, and the
// request keeps that limit well under the atom cap so sections stay small
// and selectable on their own.
func TestResearcherTasks_BothBodiesDemandSingleConcernSections(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "prompt", "atoms", "init", "researcher_tasks.yaml"))
	if err != nil {
		// When run from the package dir, the relative path differs.
		data, err = os.ReadFile(filepath.Join("..", "prompt", "atoms", "init", "researcher_tasks.yaml"))
	}
	require.NoError(t, err)
	text := string(data)
	for _, want := range []string{
		"one `## ` section per atom",
		"at most the section limit",
	} {
		assert.Equal(t, 2, strings.Count(text, want), "both methodology and domain prompts must say %q", want)
	}
	assert.Contains(t, text, "states one instruction")
	assert.Contains(t, text, "states one concept")
}

func TestAgentSectionLimit_StaysWellUnderAtomCap(t *testing.T) {
	ws := t.TempDir()
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	cap := ini.atomMaxChars()
	limit := ini.agentSectionLimit()
	require.Greater(t, cap, 0)
	assert.LessOrEqual(t, limit, cap, "section limit must not exceed the atom cap")
	assert.LessOrEqual(t, limit, cap/2+150, "sections stay small: well under the cap")
	assert.GreaterOrEqual(t, limit, 300)
	assert.LessOrEqual(t, limit, 600)

	req := ini.agentPromptRequest(RecommendedAgent{Name: "GoExpert", Description: "Go specialist", Topics: []string{"concurrency"}})
	assert.Contains(t, req, "Section limit: ")
	assert.NotContains(t, req, "Section limit: "+strconv.Itoa(cap)+" characters", "request must not offer the full cap per section")
	assert.Contains(t, req, "Section limit: "+strconv.Itoa(limit)+" characters")
}

func TestAgentSectionLimit_LowCapClampsToFloor(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(`{"jit":{"atom_max_chars":600}}`), 0o600))
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	require.Equal(t, 600, ini.atomMaxChars())
	assert.Equal(t, 300, ini.agentSectionLimit(), "cap/3=200 floors to 300 so sections stay one concern, not slivers")
}

func TestAgentSectionLimit_HighCapClampsToCeiling(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(`{"jit":{"atom_max_chars":2400}}`), 0o600))
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	require.Equal(t, 2400, ini.atomMaxChars())
	assert.Equal(t, 600, ini.agentSectionLimit(), "cap/3=800 ceilings to 600 so sections stay small and selectable")
}

func TestAgentSectionLimit_TinyCapNeverExceedsCap(t *testing.T) {
	ws := t.TempDir()
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	ini.atomCap = 250
	ini.atomCapOnce.Do(func() {})
	require.Equal(t, 250, ini.atomMaxChars())
	assert.Equal(t, 250, ini.agentSectionLimit(), "floored 300 would exceed a tiny cap, so the cap wins")
}

func TestSplitMarkdownSections_FalsePacksSmallHeadingsWhenOversize(t *testing.T) {
	sec := func(h string) string { return "## " + h + "\n\n" + strings.Repeat("detail word ", 20) }
	content := sec("Alpha") + "\n\n" + sec("Beta") + "\n\n" + sec("Gamma")
	cap := 500
	require.Greater(t, utf8.RuneCountInString(content), cap)

	perHeading := splitMarkdownSections(content, cap, true)
	require.Len(t, perHeading, 3, "perHeading keeps one concern per atom")
	for _, p := range perHeading {
		assert.Equal(t, 1, strings.Count(p.Body, "## "), "perHeading piece must hold exactly one heading, got %q", p.Body)
	}

	packed := splitMarkdownSections(content, cap, false)
	require.Greater(t, utf8.RuneCountInString(content), cap)
	assert.Less(t, len(packed), len(perHeading), "packing must combine headings when oversize")
	hasMulti := false
	for _, p := range packed {
		if strings.Count(p.Body, "## ") > 1 {
			hasMulti = true
		}
	}
	assert.True(t, hasMulti, "packed output must contain a multi-heading atom, got %d pieces", len(packed))
}

func TestResplitPromptsYAML_KeepsOneHeadingPerAtom(t *testing.T) {
	sec := func(h string) string { return "## " + h + "\n\n" + strings.Repeat("detail word ", 20) }
	oversize := sec("Alpha") + "\n\n" + sec("Beta") + "\n\n" + sec("Gamma")
	cap := 500
	require.Greater(t, utf8.RuneCountInString(oversize), cap)

	original := "- id: \"goexpert/identity\"\n" +
		"  category: \"identity\"\n" +
		"  is_mandatory: true\n" +
		"  content: |\n" +
		indentForYAMLBlock("You are GoExpert.", "    ") + "\n" +
		"- id: \"goexpert/domain\"\n" +
		"  category: \"domain\"\n" +
		"  subcategory: \"knowledge\"\n" +
		"  is_mandatory: false\n" +
		"  depends_on: [\"goexpert/identity\"]\n" +
		"  description: \"Domain knowledge\"\n" +
		"  content: |\n" +
		indentForYAMLBlock(oversize, "    ") + "\n"

	fixed, changed, err := resplitPromptsYAML([]byte(original), cap)
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, validatePromptsYAML(fixed, "goexpert", cap))

	var atoms []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
	}
	require.NoError(t, yaml.Unmarshal(fixed, &atoms))
	require.Len(t, atoms, 4, "identity plus one atom per heading")
	for _, a := range atoms[1:] {
		assert.Equal(t, 1, strings.Count(a.Content, "## "), "resplit atom %s must hold exactly one heading, got %q", a.ID, a.Content)
		assert.LessOrEqual(t, utf8.RuneCountInString(a.Content), cap)
	}
}

func TestRepairExistingPromptAtoms_RepairsOversizeInPlace(t *testing.T) {
	ws := t.TempDir()
	agentDir := filepath.Join(ws, ".nerd", "agents", "goexpert")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))
	half := strings.Repeat("concrete api detail. ", 40) // ~840 chars
	oversize := "## Lease Expiry\n\n" + half + "\n\n## Quorum Loss\n\n" + half
	require.Greater(t, utf8.RuneCountInString(oversize), testAtomCap)

	original := "- id: \"goexpert/identity\"\n" +
		"  category: \"identity\"\n" +
		"  is_mandatory: true\n" +
		"  content: |\n" +
		indentForYAMLBlock("You are GoExpert.", "    ") + "\n" +
		"- id: \"goexpert/domain\"\n" +
		"  category: \"domain\"\n" +
		"  subcategory: \"knowledge\"\n" +
		"  is_mandatory: false\n" +
		"  depends_on: [\"goexpert/identity\"]\n" +
		"  description: \"Domain knowledge\"\n" +
		"  intent_verbs: [\"/fix\"]\n" +
		"  content: |\n" +
		indentForYAMLBlock(oversize, "    ") + "\n"
	require.Error(t, validatePromptsYAML([]byte(original), "goexpert", testAtomCap), "fixture must start oversize")
	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	require.NoError(t, os.WriteFile(promptsPath, []byte(original), 0o600))

	fixed, err := RepairExistingPromptAtoms(ws)
	require.NoError(t, err)
	assert.Equal(t, 1, fixed, "one oversize file must be rewritten in place")

	refixed, err := os.ReadFile(promptsPath)
	require.NoError(t, err)
	require.NoError(t, validatePromptsYAML(refixed, "goexpert", testAtomCap))

	var atoms []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
	}
	require.NoError(t, yaml.Unmarshal(refixed, &atoms))
	require.Len(t, atoms, 3, "identity plus two heading-split domain atoms")
	assert.Equal(t, "goexpert/identity", atoms[0].ID)
	assert.Equal(t, "goexpert/domain", atoms[1].ID, "first piece keeps the base id")
	for _, a := range atoms {
		assert.LessOrEqual(t, utf8.RuneCountInString(a.Content), testAtomCap, "atom %s over cap", a.ID)
	}
	text := string(refixed)
	assert.Contains(t, text, "Lease Expiry")
	assert.Contains(t, text, "Quorum Loss")
	assert.Contains(t, text, "concrete api detail.")
	assert.Contains(t, text, "intent_verbs", "repair must preserve unknown fields")

	again, err := RepairExistingPromptAtoms(ws)
	require.NoError(t, err)
	assert.Equal(t, 0, again, "repaired file is now within cap and must be left alone")
}

func TestRepairExistingPromptAtoms_LeavesCompliantUntouched(t *testing.T) {
	ws := t.TempDir()
	agentDir := filepath.Join(ws, ".nerd", "agents", "goexpert")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))
	compliant := "- id: \"goexpert/identity\"\n" +
		"  category: \"identity\"\n" +
		"  is_mandatory: true\n" +
		"  content: |\n" +
		indentForYAMLBlock("You are GoExpert.", "    ") + "\n" +
		"- id: \"goexpert/methodology\"\n" +
		"  category: \"methodology\"\n" +
		"  is_mandatory: false\n" +
		"  content: |\n" +
		indentForYAMLBlock("## Retry With Backoff\n\nRetry failed writes with backoff.", "    ") + "\n"
	require.NoError(t, validatePromptsYAML([]byte(compliant), "goexpert", testAtomCap))
	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	require.NoError(t, os.WriteFile(promptsPath, []byte(compliant), 0o600))

	fixed, err := RepairExistingPromptAtoms(ws)
	require.NoError(t, err)
	assert.Equal(t, 0, fixed, "compliant file must not be rewritten")

	after, err := os.ReadFile(promptsPath)
	require.NoError(t, err)
	assert.Equal(t, compliant, string(after), "compliant file bytes must be unchanged")
}

func TestRepairExistingPromptAtoms_MissingAgentsDirReturnsZero(t *testing.T) {
	ws := t.TempDir()
	fixed, err := RepairExistingPromptAtoms(ws)
	require.NoError(t, err)
	assert.Equal(t, 0, fixed, "missing .nerd/agents must repair nothing")
}

func TestAtomSlug_TruncatesByRunesNotBytes(t *testing.T) {
	title := strings.Repeat("é", 60)
	slug := atomSlug(title)
	want := strings.Repeat("é", 48)
	assert.Equal(t, want, slug, "atomSlug must truncate to 48 runes, not 48 bytes, to keep unicode headings intact")
	assert.Equal(t, 48, utf8.RuneCountInString(slug), "truncated slug must be exactly 48 runes")
	assert.True(t, utf8.ValidString(slug), "truncated slug must remain valid UTF-8 (byte slicing can split a rune)")
}
