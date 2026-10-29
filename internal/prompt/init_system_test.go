package prompt

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

const (
	initResearcherPrefix    = "init/researcher/"
	initResearcherMissionID = "init/researcher/mission"
	// Small single-concern budget: init atoms must stay tiny.
	// Researcher guidance atoms are 1-2 sentences (<500 chars);
	// 2000 chars / 500 tokens leaves headroom without permitting bloat.
	maxInitAtomChars  = 2000
	maxInitAtomTokens = 500
)

func loadInitResearcherAtoms(t *testing.T) []*PromptAtom {
	t.Helper()
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	var inits []*PromptAtom
	for _, a := range corpus.All() {
		if strings.HasPrefix(a.ID, initResearcherPrefix) {
			inits = append(inits, a)
		}
	}
	if len(inits) == 0 {
		t.Fatalf("embedded corpus has no %q atoms", initResearcherPrefix)
	}
	return inits
}

func TestInitResearcherAtoms_AreSmallSingleConcern(t *testing.T) {
	inits := loadInitResearcherAtoms(t)
	for _, a := range inits {
		t.Run(a.ID, func(t *testing.T) {
			if err := a.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if len(a.Content) == 0 {
				t.Fatalf("empty content")
			}
			if len(a.Content) > maxInitAtomChars {
				t.Fatalf("content %d chars exceeds single-concern budget %d", len(a.Content), maxInitAtomChars)
			}
			if got := EstimateTokens(a.Content); got > maxInitAtomTokens {
				t.Fatalf("tokens %d exceeds budget %d", got, maxInitAtomTokens)
			}
			// Single concern: small atom, but structured guidance may use
			// blank-line separated sections (headings, steps). Size budgets above
			// enforce smallness; here only reject excessive sectioning.
			if strings.Count(strings.TrimSpace(a.Content), "\n\n") > 5 {
				t.Fatalf("content has too many blank-line sections, split into smaller atoms")
			}
			if len(a.ShardTypes) == 0 {
				t.Fatalf("missing shard_types pin")
			}
		})
	}
}

func TestInitResearcherMission_IsMandatoryPersona(t *testing.T) {
	inits := loadInitResearcherAtoms(t)
	byID := make(map[string]*PromptAtom, len(inits))
	for _, a := range inits {
		byID[a.ID] = a
	}
	mission, ok := byID[initResearcherMissionID]
	if !ok {
		t.Fatalf("missing mission atom %q", initResearcherMissionID)
	}
	if !mission.IsMandatory {
		t.Fatalf("mission must be mandatory")
	}
	if len(mission.InitPhases) == 0 {
		t.Fatalf("mission must list init_phases it serves")
	}
	// Every other init/researcher atom continues the mission.
	for _, a := range inits {
		if a.ID == initResearcherMissionID {
			continue
		}
		found := false
		for _, dep := range a.DependsOn {
			if dep == initResearcherMissionID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("atom %q must depend on %q", a.ID, initResearcherMissionID)
		}
		if len(a.InitPhases) == 0 {
			t.Fatalf("atom %q has no init_phases, want at least 1 for single concern", a.ID)
		}
	}
}

func TestParseAgentPromptsYAML_SmallSingleConcern(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantID  string
		wantErr bool
	}{
		{
			name:    "canonical small agent atom",
			yaml:    "- schema_version: 1\n  version: 1\n  id: test/agent/small\n  category: methodology\n  is_mandatory: false\n  shard_types: [/coder]\n  content: Small single-concern guidance.\n",
			wantID:  "test/agent/small",
			wantErr: false,
		},
		{
			name:    "empty content rejected",
			yaml:    "- schema_version: 1\n  version: 1\n  id: test/agent/empty\n  category: methodology\n  is_mandatory: false\n  content: ''\n",
			wantErr: true,
		},
		{
			name:    "unknown field rejected",
			yaml:    "id: test/agent/bad\ncategory: identity\nis_mandatory: false\ncontent: body\ntyop: true\n",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, _, err := ParsePromptAtomYAML([]byte(tt.yaml), "prompts.yaml", nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(parsed) != 1 {
				t.Fatalf("got %d atoms, want 1", len(parsed))
			}
			atom := parsed[0].Atom
			if atom.ID != tt.wantID {
				t.Fatalf("id = %q, want %q", atom.ID, tt.wantID)
			}
			if err := atom.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if len(atom.Content) > maxInitAtomChars {
				t.Fatalf("agent atom %d chars exceeds %d", len(atom.Content), maxInitAtomChars)
			}
		})
	}
}

func TestLoadAgentPrompts_MissingFile_ReturnsZero(t *testing.T) {
	nerdDir := t.TempDir()
	n, err := LoadAgentPrompts(context.Background(), "no-such-agent", nerdDir, nil)
	if err != nil {
		t.Fatalf("expected nil error for missing prompts.yaml, got %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d, want 0", n)
	}
}

func TestLoadAgentPrompts_MissingDB_Errors(t *testing.T) {
	nerdDir := t.TempDir()
	agentDir := filepath.Join(nerdDir, "agents", "demo")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "- schema_version: 1\n  version: 1\n  id: demo/small\n  category: methodology\n  is_mandatory: false\n  content: Small guidance.\n"
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write prompts.yaml: %v", err)
	}
	_, err := LoadAgentPrompts(context.Background(), "demo", nerdDir, nil)
	if err == nil {
		t.Fatalf("expected missing knowledge DB error, got nil")
	}
	if !strings.Contains(err.Error(), "knowledge database does not exist") {
		t.Fatalf("error = %v, want missing knowledge DB", err)
	}
}

func TestAtomLoader_LoadFromYAML_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "prompts.yaml")
	yaml := "- schema_version: 1\n  version: 1\n  id: demo/one\n  category: methodology\n  is_mandatory: false\n  content: First small atom.\n- schema_version: 1\n  version: 1\n  id: demo/two\n  category: safety\n  is_mandatory: true\n  content: Second small atom.\n"
	if err := os.WriteFile(yamlPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	loader := NewAtomLoader(nil)
	ctx := context.Background()
	if err := loader.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	n, err := loader.LoadFromYAML(ctx, yamlPath, db)
	if err != nil {
		t.Fatalf("LoadFromYAML: %v", err)
	}
	if n != 2 {
		t.Fatalf("got %d, want 2", n)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("db count = %d, want 2", count)
	}
}

func TestLoadProjectPrompts_Deprecated_ReturnsZero(t *testing.T) {
	nerdDir := t.TempDir()
	n, err := LoadProjectPrompts(context.Background(), nerdDir, nil)
	if err != nil {
		t.Fatalf("empty nerdDir: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d, want 0", n)
	}
	// Legacy files are not loaded; they warn for migration instead.
	promptsDir := filepath.Join(nerdDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "legacy.yaml"), []byte("id: x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	n, err = LoadProjectPrompts(context.Background(), nerdDir, nil)
	if err != nil {
		t.Fatalf("legacy dir: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d, want 0 for deprecated project prompts", n)
	}
}

func TestReloadAllPrompts_EmptyNerdDir_ReturnsZero(t *testing.T) {
	nerdDir := t.TempDir()
	n, err := ReloadAllPrompts(context.Background(), nerdDir, nil)
	if err != nil {
		t.Fatalf("ReloadAllPrompts: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d, want 0", n)
	}
}

func TestPromptAtom_Validate_RejectsEmptyContent(t *testing.T) {
	tests := map[string]struct {
		atom    *PromptAtom
		wantErr bool
	}{
		"empty content": {atom: NewPromptAtom("test/empty", CategoryMethodology, ""), wantErr: true},
		"empty id":      {atom: NewPromptAtom("", CategoryMethodology, "body"), wantErr: true},
		"valid small":   {atom: NewPromptAtom("test/small", CategoryMethodology, "small body"), wantErr: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.atom.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}
