package prompt

import (
	"database/sql"
	"path/filepath"
	"testing"

	"codenerd/internal/core"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestCorpus creates a temporary SQLite database with a valid schema and sample data.
func createTestCorpus(t *testing.T) *core.PredicateCorpus {
	t.Helper()

	// Create a temp file for the DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_corpus.db")

	// Open DB directly to set up schema
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE predicates (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE,
			arity INTEGER,
			type TEXT,
			category TEXT,
			description TEXT,
			safety_level TEXT,
			domain TEXT,
			section TEXT,
			source_file TEXT,
			activation_priority INTEGER,
			serialization_order INTEGER
		);
		CREATE TABLE predicate_domains (
			predicate_id INTEGER,
			domain TEXT,
			PRIMARY KEY (predicate_id, domain),
			FOREIGN KEY (predicate_id) REFERENCES predicates(id)
		);
	`)
	require.NoError(t, err)

	// Sample predicates to insert
	// We need predicates for domains: core, safety, routing, shard_lifecycle, shard_coder, shard_tester, campaign
	predicates := []struct {
		name     string
		arity    int
		domain   string
		domains  []string // Additional domains in predicate_domains table
		category string
		desc     string
	}{
		// Core predicates
		{"core_fact", 1, "core", []string{"core"}, "base", "A core fact"},
		{"file_topology", 3, "core", []string{"core"}, "world", "Filesystem structure"},

		// Safety predicates
		{"permitted", 1, "safety", []string{"safety"}, "security", "Allowed actions"},
		{"dangerous", 1, "safety", []string{"safety"}, "security", "Dangerous actions"},

		// Routing predicates
		{"next_action", 1, "routing", []string{"routing"}, "dispatch", "The next action"},

		// Shard Lifecycle
		{"shard_status", 2, "shard_lifecycle", []string{"shard_lifecycle"}, "meta", "Status of shards"},

		// Coder specific
		{"code_change", 3, "shard_coder", []string{"shard_coder"}, "mutation", "Code modification"},
		{"diagnostic", 4, "diagnostic", []string{"diagnostic", "shard_coder", "shard_tester"}, "analysis", "Compiler errors"},

		// Tester specific
		{"test_result", 3, "shard_tester", []string{"shard_tester"}, "verification", "Test execution result"},

		// Campaign
		{"campaign_goal", 2, "campaign", []string{"campaign"}, "planning", "Current campaign goal"},

		// Other
		{"random_fact", 1, "misc", []string{"misc"}, "misc", "Irrelevant fact"},
	}

	tx, err := db.Begin()
	require.NoError(t, err)

	stmtPred, err := tx.Prepare(`INSERT INTO predicates (name, arity, type, category, description, safety_level, domain, section, source_file) VALUES (?, ?, 'EDB', ?, ?, 'safe', ?, '', '')`)
	require.NoError(t, err)
	defer stmtPred.Close()

	stmtDomain, err := tx.Prepare(`INSERT INTO predicate_domains (predicate_id, domain) VALUES (?, ?)`)
	require.NoError(t, err)
	defer stmtDomain.Close()

	for _, p := range predicates {
		res, err := stmtPred.Exec(p.name, p.arity, p.category, p.desc, p.domain)
		require.NoError(t, err)

		id, err := res.LastInsertId()
		require.NoError(t, err)

		for _, d := range p.domains {
			_, err = stmtDomain.Exec(id, d)
			require.NoError(t, err)
		}
	}

	require.NoError(t, tx.Commit())

	// Create the corpus wrapper
	corpus, err := core.NewPredicateCorpusFromPath(dbPath)
	require.NoError(t, err)

	return corpus
}

func TestPredicateSelector_Select(t *testing.T) {
	corpus := createTestCorpus(t)
	defer corpus.Close()

	selector := NewPredicateSelector(corpus)

	t.Run("Empty Context", func(t *testing.T) {
		// Should return core predicates + safety + routing by default logic (though Select adds them specifically)
		ctx := SelectionContext{}
		preds, err := selector.Select(ctx)
		require.NoError(t, err)

		names := make(map[string]bool)
		for _, p := range preds {
			names[p.Name] = true
		}

		assert.Contains(t, names, "core_fact")
		assert.Contains(t, names, "permitted") // Safety is always included
		assert.Contains(t, names, "next_action") // Routing is always included
		assert.NotContains(t, names, "code_change") // Specific to coder
	})

	t.Run("Shard Context - Coder", func(t *testing.T) {
		ctx := SelectionContext{
			ShardType: "/coder",
		}
		preds, err := selector.Select(ctx)
		require.NoError(t, err)

		names := make(map[string]bool)
		for _, p := range preds {
			names[p.Name] = true
		}

		assert.Contains(t, names, "code_change")
		assert.Contains(t, names, "diagnostic")
		assert.Contains(t, names, "shard_status") // from shard_lifecycle
	})

	t.Run("Intent Context - Fix", func(t *testing.T) {
		ctx := SelectionContext{
			IntentVerb: "/fix",
		}
		preds, err := selector.Select(ctx)
		require.NoError(t, err)

		names := make(map[string]bool)
		for _, p := range preds {
			names[p.Name] = true
		}

		// /fix maps to diagnostic, world_model
		assert.Contains(t, names, "diagnostic")
	})

	t.Run("Campaign Context", func(t *testing.T) {
		ctx := SelectionContext{
			CampaignPhase: "/planning",
		}
		preds, err := selector.Select(ctx)
		require.NoError(t, err)

		names := make(map[string]bool)
		for _, p := range preds {
			names[p.Name] = true
		}

		assert.Contains(t, names, "campaign_goal")
	})

	t.Run("Relevance Sorting", func(t *testing.T) {
		// Core (1.0) > Safety (0.95) > Shard (0.9) > Intent (0.85) > Campaign (0.8) > Routing (0.7)
		ctx := SelectionContext{
			ShardType:     "/coder",
			IntentVerb:    "/fix",
			CampaignPhase: "/executing",
		}
		preds, err := selector.Select(ctx)
		require.NoError(t, err)

		// Check that order respects relevance
		// "core_fact" (1.0) should be before "permitted" (0.95)
		// "permitted" (0.95) should be before "code_change" (0.9)

		var coreIdx, safetyIdx, shardIdx int = -1, -1, -1

		for i, p := range preds {
			if p.Name == "core_fact" { coreIdx = i }
			if p.Name == "permitted" { safetyIdx = i }
			if p.Name == "code_change" { shardIdx = i }
		}

		assert.True(t, coreIdx < safetyIdx, "Core should be before Safety")
		assert.True(t, safetyIdx < shardIdx, "Safety should be before Shard")
	})
}

func TestPredicateSelector_FormatForPrompt(t *testing.T) {
	corpus := createTestCorpus(t)
	defer corpus.Close()

	selector := NewPredicateSelector(corpus)

	// Manually create selected predicates to test formatting
	selected := []SelectedPredicate{
		{Name: "pred1", Arity: 2, Domain: "domA", Description: "Description 1", Relevance: 0.9},
		{Name: "pred2", Arity: 1, Domain: "domB", Description: "Description 2", Relevance: 0.8},
		{Name: "pred3", Arity: 3, Domain: "domA", Description: "Description 3 is very long and should be truncated if it exceeds 60 characters which this string definitely does.", Relevance: 0.7},
	}

	output := selector.FormatForPrompt(selected)

	assert.Contains(t, output, "## Available Mangle Predicates")
	assert.Contains(t, output, "### domA")
	assert.Contains(t, output, "- `pred1/2` - Description 1")
	// The truncation happens at 60 chars.
	// "Description 3 is very long and should be truncated if it exc" is 60 chars.
	assert.Contains(t, output, "- `pred3/3` - Description 3 is very long and should be truncated if it exc...")
	assert.Contains(t, output, "### domB")
	assert.Contains(t, output, "- `pred2/1` - Description 2")
}

func TestPredicateSelector_MaxPredicates(t *testing.T) {
	corpus := createTestCorpus(t)
	defer corpus.Close()

	selector := NewPredicateSelector(corpus)
	selector.SetMaxPredicates(2) // Very low limit

	ctx := SelectionContext{
		ShardType: "/coder", // Should pull in many predicates
	}

	preds, err := selector.Select(ctx)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(preds), 2)
}

func TestPredicateSelector_SelectForMangleGeneration(t *testing.T) {
	corpus := createTestCorpus(t)
	defer corpus.Close()

	selector := NewPredicateSelector(corpus)

	preds, err := selector.SelectForMangleGeneration("/coder", "/implement")
	require.NoError(t, err)

	// Should include routing and shard_lifecycle
	names := make(map[string]bool)
	for _, p := range preds {
		names[p.Name] = true
	}

	assert.Contains(t, names, "next_action")
	assert.Contains(t, names, "shard_status")
}
