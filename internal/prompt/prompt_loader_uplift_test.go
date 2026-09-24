package prompt

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The world-state mapping lives in four places: WorldStates, hasWorldState,
// GenerateFacts, and the AllContextDimensions vocabulary. They must agree on
// every context, or selection sees a different world through each path.
func TestWorldStateCopies_Agree(t *testing.T) {
	full := NewCompilationContext()
	full.FailingTestCount = 2
	full.DiagnosticCount = 1
	full.IsLargeRefactor = true
	full.HasSecurityIssues = true
	full.HasNewFiles = true
	full.IsHighChurn = true
	full.HasReflectionHits = true
	full.PreviousAttemptNoToolCall = true

	some := NewCompilationContext()
	some.DiagnosticCount = 3
	some.HasNewFiles = true

	for name, cc := range map[string]*CompilationContext{"full": full, "some": some, "zero": NewCompilationContext()} {
		t.Run(name, func(t *testing.T) {
			states := cc.WorldStates()
			for state := range KnownWorldStates() {
				want := slices.Contains(states, state)
				assert.Equal(t, want, hasWorldState(cc, state), "hasWorldState(%q) disagrees with WorldStates()", state)
			}
			facts := cc.GenerateFacts(FactStyle{Predicate: "current_context", UseShort: true, ForceAtoms: true})
			var factStates []string
			for _, f := range facts {
				s, ok := f.(string)
				if !ok {
					continue
				}
				if rest, found := strings.CutPrefix(s, "current_context(/state, /"); found {
					factStates = append(factStates, strings.TrimSuffix(rest, ")"))
				}
			}
			assert.ElementsMatch(t, states, factStates, "GenerateFacts world states disagree with WorldStates()")
		})
	}
}

// tagRoundTripFixture carries one distinctive value per persistence dimension,
// pre-normalized exactly as canonical YAML loads produce.
func tagRoundTripFixture() *PromptAtom {
	return &PromptAtom{
		ID: "uplift/tagged", Version: 1, Category: CategoryProtocol,
		Content: "tagged content", TokenCount: 4, ContentHash: "hash",
		Priority:         50,
		OperationalModes: []string{"active"},
		CampaignPhases:   []string{"planning"},
		BuildLayers:      []string{"domain_core"},
		InitPhases:       []string{"analysis"},
		NorthstarPhases:  []string{"vision"},
		OuroborosStages:  []string{"detection"},
		IntentVerbs:      []string{"fix"},
		ShardTypes:       []string{"coder"},
		Languages:        []string{"go"},
		Frameworks:       []string{"bubbletea"},
		Models:           []string{"claude_opus_4"},
		Providers:        []string{"anthropic"},
		WorldStates:      []string{"failing_tests"},
		DependsOn:        []string{"dep/id"},
		ConflictsWith:    []string{"conflict/id"},
		RequiresTools:    []string{"read_file"},
	}
}

func tagFieldAccessors() map[string]func(*PromptAtom) []string {
	return map[string]func(*PromptAtom) []string{
		"mode":            func(a *PromptAtom) []string { return a.OperationalModes },
		"phase":           func(a *PromptAtom) []string { return a.CampaignPhases },
		"layer":           func(a *PromptAtom) []string { return a.BuildLayers },
		"init_phase":      func(a *PromptAtom) []string { return a.InitPhases },
		"northstar_phase": func(a *PromptAtom) []string { return a.NorthstarPhases },
		"ouroboros_stage": func(a *PromptAtom) []string { return a.OuroborosStages },
		"intent":          func(a *PromptAtom) []string { return a.IntentVerbs },
		"shard":           func(a *PromptAtom) []string { return a.ShardTypes },
		"lang":            func(a *PromptAtom) []string { return a.Languages },
		"framework":       func(a *PromptAtom) []string { return a.Frameworks },
		"model":           func(a *PromptAtom) []string { return a.Models },
		"provider":        func(a *PromptAtom) []string { return a.Providers },
		"state":           func(a *PromptAtom) []string { return a.WorldStates },
		"depends_on":      func(a *PromptAtom) []string { return a.DependsOn },
		"conflicts_with":  func(a *PromptAtom) []string { return a.ConflictsWith },
		"requires_tool":   func(a *PromptAtom) []string { return a.RequiresTools },
	}
}

// Every dimension ContextTags emits must land in an atom field through the
// appendTag reader. A writer-side addition without a reader case (the exact
// drift that silently dropped models/providers/requires_tool on every DB
// round-trip) fails here instead of leaking unpinned atoms into selection.
func TestContextTags_ReaderParity(t *testing.T) {
	atom := tagRoundTripFixture()
	fields := tagFieldAccessors()
	require.Len(t, atom.ContextTags(), len(fields), "fixture must carry one value per dimension")

	var compiler JITPromptCompiler
	got := &PromptAtom{}
	seen := make(map[string]int)
	for _, tag := range atom.ContextTags() {
		seen[tag.Dimension]++
		accessor, ok := fields[tag.Dimension]
		if !ok {
			t.Errorf("dimension %q emitted but not reader-routed; add an appendTag case and a field accessor", tag.Dimension)
			continue
		}
		compiler.appendTag(got, tag.Dimension, tag.Tag)
		_ = accessor
	}
	require.Len(t, seen, len(fields), "reader must route every emitted dimension")

	for dim, accessor := range fields {
		vals := accessor(got)
		require.Len(t, vals, 1, "dimension %q did not land in its field", dim)
	}
	assert.Equal(t, []string{"claude_opus_4"}, got.Models)
	assert.Equal(t, []string{"anthropic"}, got.Providers)
	assert.Equal(t, []string{"read_file"}, got.RequiresTools)
}

// All three tag-table writers must preserve every dimension through a real
// SQLite round-trip, and the loaded atom must still gate behaviorally: pins
// admit the matching vendor and exclude others, tool gates admit a catalog
// with the tool and block one without it.
func TestContextTags_RoundTripThroughDB(t *testing.T) {
	ctx := context.Background()
	writers := map[string]func(ctx context.Context, loader *AtomLoader, db *sql.DB, atoms []*PromptAtom) error{
		"store": func(ctx context.Context, loader *AtomLoader, db *sql.DB, atoms []*PromptAtom) error {
			return loader.StoreAtom(ctx, db, atoms[0])
		},
		"replace": func(ctx context.Context, loader *AtomLoader, db *sql.DB, atoms []*PromptAtom) error {
			return loader.ReplaceAtoms(ctx, db, atoms)
		},
		"reconcile": func(ctx context.Context, loader *AtomLoader, db *sql.DB, atoms []*PromptAtom) error {
			_, err := ReconcilePromptCorpus(ctx, db, atoms)
			return err
		},
	}

	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "roundtrip.db")
			db, err := sql.Open("sqlite3", dbPath)
			require.NoError(t, err)
			defer db.Close()

			loader := NewAtomLoader(nil)
			require.NoError(t, loader.EnsureSchema(ctx, db))
			require.NoError(t, write(ctx, loader, db, []*PromptAtom{tagRoundTripFixture()}))

			compiler, err := NewJITPromptCompiler()
			require.NoError(t, err)
			loaded, err := compiler.loadAtomsFromDB(ctx, db)
			require.NoError(t, err)
			require.Len(t, loaded, 1)
			got := loaded[0]

			want := tagRoundTripFixture()
			fields := tagFieldAccessors()
			for dim, accessor := range fields {
				assert.Equal(t, accessor(want), accessor(got), "dimension %q lost on round-trip", dim)
			}

			// Behavioral: the surviving pins still gate.
			matchCC := NewCompilationContext()
			matchCC.ShardType = "/coder"
			matchCC.Language = "/go"
			matchCC.IntentVerb = "/fix"
			matchCC.OperationalMode = "/active"
			matchCC.CampaignPhase = "/planning"
			matchCC.BuildLayer = "/domain_core"
			matchCC.InitPhase = "/analysis"
			matchCC.NorthstarPhase = "/vision"
			matchCC.OuroborosStage = "/detection"
			matchCC.Frameworks = []string{"/bubbletea"}
			matchCC.FailingTestCount = 1
			matchCC.WithProviderModel("anthropic", "claude-opus-4")
			matchCC.AvailableTools = []string{"read_file", "search_code"}
			assert.True(t, got.MatchesContext(matchCC), "loaded atom must match its own pinned context")
			assert.True(t, atomToolSatisfied(got, availableToolSet(matchCC)))

			otherVendor := NewCompilationContext()
			*otherVendor = *matchCC
			otherVendor.WithProviderModel("openai", "gpt-4o")
			assert.False(t, got.MatchesContext(otherVendor), "provider pin must exclude other vendors after round-trip")

			noTool := NewCompilationContext()
			*noTool = *matchCC
			noTool.AvailableTools = []string{"search_code"}
			assert.False(t, atomToolSatisfied(got, availableToolSet(noTool)), "requires_tools must block a catalog without the tool")
		})
	}
}

// hasWorldState is a hand-written switch over the live world-state vocabulary.
// Every value KnownWorldStates admits must be activatable, or a YAML-valid
// atom silently never matches on any Go selection path.
func TestHasWorldState_CoversKnownWorldStates(t *testing.T) {
	activators := map[string]func(*CompilationContext){
		"failing_tests":      func(cc *CompilationContext) { cc.FailingTestCount = 1 },
		"diagnostics":        func(cc *CompilationContext) { cc.DiagnosticCount = 1 },
		"large_refactor":     func(cc *CompilationContext) { cc.IsLargeRefactor = true },
		"security_issues":    func(cc *CompilationContext) { cc.HasSecurityIssues = true },
		"new_files":          func(cc *CompilationContext) { cc.HasNewFiles = true },
		"high_churn":         func(cc *CompilationContext) { cc.IsHighChurn = true },
		"reflection_hits":    func(cc *CompilationContext) { cc.HasReflectionHits = true },
		"no_tool_call_retry": func(cc *CompilationContext) { cc.PreviousAttemptNoToolCall = true },
		"authoring_mangle":   func(cc *CompilationContext) { cc.DerivedNeeds = []string{"authoring_mangle"} },
		"envelope_tool_requests": func(cc *CompilationContext) {
			cc.DerivedNeeds = []string{"envelope_tool_requests"}
		},
		"envelope_knowledge_requests": func(cc *CompilationContext) {
			cc.DerivedNeeds = []string{"envelope_knowledge_requests"}
		},
	}
	known := KnownWorldStates()
	require.NotEmpty(t, known)
	for state := range known {
		activate, ok := activators[state]
		if !ok {
			t.Errorf("world state %q is YAML-valid but hasWorldState has no case for it; add one", state)
			continue
		}
		cc := NewCompilationContext()
		activate(cc)
		assert.True(t, hasWorldState(cc, state), "state %q must be activatable", state)
		assert.False(t, hasWorldState(NewCompilationContext(), state), "state %q must not match a zero context", state)
	}
}

// Evolved files bypass the canonical schema parser, so the loader applies its
// post-conditions by hand: corrupt files are skipped (never fatal, never
// silent) and valid ones come out normalized with computed fields filled.
func TestEvolvedAtomLoad_ValidatesAndBackfills(t *testing.T) {
	dir := t.TempDir()
	pending := filepath.Join(dir, "prompts", "evolved", "pending")
	require.NoError(t, os.MkdirAll(pending, 0755))

	valid := NewPromptAtom("evolved/valid", CategoryProtocol, "learned guidance content here")
	valid.ShardTypes = []string{"/coder"}
	valid.TokenCount = 0
	valid.ContentHash = ""
	wrapper := struct {
		Atom *PromptAtom `yaml:"atom"`
	}{Atom: valid}
	data, err := yaml.Marshal(&wrapper)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pending, "valid.yaml"), data, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pending, "invalid.yaml"), []byte("atom:\n  id: evolved/broken\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pending, "broken.yaml"), []byte("{{{\n"), 0644))

	mgr := NewEvolvedAtomManager(dir)
	require.NoError(t, mgr.Reload())
	assert.Equal(t, 1, mgr.Count(), "only the valid file loads; corrupt files skip")

	got := mgr.Get("evolved/valid")
	require.NotNil(t, got)
	assert.Equal(t, []string{"coder"}, got.ShardTypes, "selectors must be normalized on load")
	assert.Greater(t, got.TokenCount, 0, "token count must be backfilled")
	assert.NotEmpty(t, got.ContentHash, "content hash must be backfilled")
	assert.True(t, got.MatchesContext(func() *CompilationContext {
		cc := NewCompilationContext()
		cc.ShardType = "/coder"
		return cc
	}()))
}
