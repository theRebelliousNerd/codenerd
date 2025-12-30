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
		{Predicate: "git_history", Args: []interface{}{"hero.go", "hash1", "TheHero", int64(10), "msg"}},
		{Predicate: "git_history", Args: []interface{}{"hero.go", "hash2", "TheHero", int64(4), "msg"}},
		{Predicate: "churn_rate", Args: []interface{}{"hero.go", 10}},
		{Predicate: "cyclomatic_complexity", Args: []interface{}{"hero.go", "ComplexFunc", int64(20)}},
	}

	// --- Scenario B: Shotgun Surgery ---
	setupShotgun := []core.Fact{
		{Predicate: "git_history", Args: []interface{}{"shotgunA.go", "c1", "Auth", int64(1), "m"}},
		{Predicate: "git_history", Args: []interface{}{"shotgunB.go", "c1", "Auth", int64(1), "m"}},

		{Predicate: "git_history", Args: []interface{}{"shotgunA.go", "c2", "Auth", int64(2), "m"}},
		{Predicate: "git_history", Args: []interface{}{"shotgunB.go", "c2", "Auth", int64(2), "m"}},

		{Predicate: "git_history", Args: []interface{}{"shotgunA.go", "c3", "Auth", int64(3), "m"}},
		{Predicate: "git_history", Args: []interface{}{"shotgunB.go", "c3", "Auth", int64(3), "m"}},
	}

	// --- Scenario C: Layer Leakage ---
	setupLayerLeakage := []core.Fact{
		{Predicate: "symbol_graph", Args: []interface{}{"ID_Lib", "function", "public", "internal/lib/lib.go", int64(1)}},
		{Predicate: "symbol_graph", Args: []interface{}{"ID_App", "function", "public", "cmd/app/main.go", int64(1)}},
		{Predicate: "dependency_link", Args: []interface{}{"ID_Lib", "ID_App", "call"}},

		// Mock string_contains because it's not virtual in this test environment
		{Predicate: "string_contains", Args: []interface{}{"internal/lib/lib.go", "internal/"}},
		{Predicate: "string_contains", Args: []interface{}{"cmd/app/main.go", "cmd/"}},
	}

	// --- Scenario D: Zombie Test ---
	setupZombie := []core.Fact{
		// file_topology(Path, Hash, Language, LastModified, IsTestFile)
		{Predicate: "file_topology", Args: []interface{}{"test_zombie.go", "hash", core.MangleAtom("/go"), int64(0), core.MangleAtom("/true")}},
		{Predicate: "symbol_graph", Args: []interface{}{"ID_Test", "function", "public", "test_zombie.go", int64(1)}},
	}

	// Assert All
	allFacts := append(setupHeroRisk, setupShotgun...)
	allFacts = append(allFacts, setupLayerLeakage...)
	allFacts = append(allFacts, setupZombie...)

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
	foundZombie := false

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

		switch rule {
		case "HERO_RISK":
			foundHero = true
		case "SHOTGUN_SURGERY":
			foundShotgun = true
		case "LAYER_LEAKAGE":
			foundLayer = true
		case "ZOMBIE_TEST":
			foundZombie = true
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
	if !foundZombie {
		t.Error("Did not detect ZOMBIE_TEST")
	}
}
