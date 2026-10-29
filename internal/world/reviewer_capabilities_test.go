package world

import (
	"codenerd/internal/core"
	"context"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReviewerCapabilities(t *testing.T) {
	// 1. Initialize Kernel (loads default schemas automatically)
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to initialize kernel: %v", err)
	}

	// Determine absolute path to defaults directory
	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(filepath.Dir(filename))) // codenerd/internal/world -> internal -> codenerd
	defaultsDir := filepath.Join(rootDir, "internal", "core", "defaults")

	// Load ONLY reviewer.mg (Schemas are already loaded by NewRealKernel)
	policies := []string{
		"reviewer.mg",
	}

	for _, p := range policies {
		fullPath := filepath.Join(defaultsDir, p)
		if err := kernel.LoadPolicyFile(fullPath); err != nil {
			t.Fatalf("Failed to load policy %s: %v", fullPath, err)
		}
	}

	ctx := context.Background()
	_ = ctx

	// 2. Setup Mock Facts

	// --- Scenario A: Hero Risk ---
	setupHeroRisk := []core.Fact{
		// git_history(File, Hash, Author, AgeDays (int), Message)
		{Predicate: "git_history", Args: []any{"hero.go", "hash1", "TheHero", int64(10), "msg"}},
		{Predicate: "git_history", Args: []any{"hero.go", "hash2", "TheHero", int64(4), "msg"}},
		{Predicate: "churn_rate", Args: []any{"hero.go", 10}},
		{Predicate: "cyclomatic_complexity", Args: []any{"hero.go", "ComplexFunc", int64(20)}},
	}

	// --- Scenario B: Shotgun Surgery ---
	setupShotgun := []core.Fact{
		{Predicate: "git_history", Args: []any{"shotgunA.go", "c1", "Auth", int64(1), "m"}},
		{Predicate: "git_history", Args: []any{"shotgunB.go", "c1", "Auth", int64(1), "m"}},

		{Predicate: "git_history", Args: []any{"shotgunA.go", "c2", "Auth", int64(2), "m"}},
		{Predicate: "git_history", Args: []any{"shotgunB.go", "c2", "Auth", int64(2), "m"}},

		{Predicate: "git_history", Args: []any{"shotgunA.go", "c3", "Auth", int64(3), "m"}},
		{Predicate: "git_history", Args: []any{"shotgunB.go", "c3", "Auth", int64(3), "m"}},
	}

	// Near miss: co-committed as often, but one imports the other.
	setupCoupledLinked := []core.Fact{
		{Predicate: "git_history", Args: []any{"linkedA.go", "c4", "Auth", int64(1), "m"}},
		{Predicate: "git_history", Args: []any{"linkedB.go", "c4", "Auth", int64(1), "m"}},
		{Predicate: "git_history", Args: []any{"linkedA.go", "c5", "Auth", int64(2), "m"}},
		{Predicate: "git_history", Args: []any{"linkedB.go", "c5", "Auth", int64(2), "m"}},
		{Predicate: "git_history", Args: []any{"linkedA.go", "c6", "Auth", int64(3), "m"}},
		{Predicate: "git_history", Args: []any{"linkedB.go", "c6", "Auth", int64(3), "m"}},
		{Predicate: "dependency_link", Args: []any{"linkedB.go", "linkedA.go", "codenerd/linked"}},
	}

	// --- Scenario C: Layer Leakage ---
	// dependency_link is file level, as the world scanner resolves it
	// (internal/world/dependency_links.go): importer file, imported file,
	// import path.
	setupLayerLeakage := []core.Fact{
		{Predicate: "symbol_graph", Args: []any{"ID_Lib", "function", "public", "internal/lib/lib.go", "func ID_Lib()"}},
		{Predicate: "symbol_graph", Args: []any{"ID_App", "function", "public", "cmd/app/main.go", "func ID_App()"}},
		{Predicate: "dependency_link", Args: []any{"internal/lib/lib.go", "cmd/app/main.go", "codenerd/cmd/app"}},
		// Near miss: a library importing a library.
		{Predicate: "symbol_graph", Args: []any{"ID_Lib2", "function", "public", "internal/lib2/lib.go", "func ID_Lib2()"}},
		{Predicate: "dependency_link", Args: []any{"internal/lib2/lib.go", "internal/lib/lib.go", "codenerd/internal/lib"}},
	}

	// Assert All
	allFacts := append(setupHeroRisk, setupShotgun...)
	allFacts = append(allFacts, setupCoupledLinked...)
	allFacts = append(allFacts, setupLayerLeakage...)

	for _, f := range allFacts {
		if err := kernel.Assert(f); err != nil {
			t.Fatalf("Failed to assert fact %v: %v", f, err)
		}
	}

	// 3. Evaluate
	// Using Query String
	findings, err := kernel.Query("raw_finding(File, Line, Sev, Cat, Rule, Msg)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	foundHero := false
	foundShotgun := false
	foundLayer := false

	t.Logf("Found %d findings:", len(findings))
	for _, f := range findings {
		rule, ok := f.Args[4].(string)
		if !ok {
			t.Logf("  Rule arg is not string? %T", f.Args[4])
			continue
		}

		var msg string
		if m, ok := f.Args[5].(string); ok {
			msg = m
		}

		t.Logf("  Rule: %s, Msg: %v", rule, msg)
		file, _ := f.Args[0].(string)

		switch rule {
		case "HERO_RISK":
			foundHero = true
		case "SHOTGUN_SURGERY":
			if file == "linkedA.go" || file == "linkedB.go" {
				t.Errorf("SHOTGUN_SURGERY for %s: the pair imports each other", file)
			}
			foundShotgun = true
		case "LAYER_LEAKAGE":
			if file != "internal/lib/lib.go" {
				t.Errorf("LAYER_LEAKAGE for %s: it imports no entrypoint", file)
			}
			foundLayer = true
		}
	}

	if !foundHero {
		t.Error("Did not detect HERO_RISK")
	}
	if !foundShotgun {
		t.Error("Did not detect SHOTGUN_SURGERY")
	}
	if !foundLayer {
		t.Error("Did not detect LAYER_LEAKAGE")
	}
}
