package init

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/prompt"

	_ "github.com/mattn/go-sqlite3"
)

// ---------------------------------------------------------------------------
// Framework detection (ProjectProfile.Framework was never assigned)
// ---------------------------------------------------------------------------

func TestDetectFrameworkFromDependencies_WhenNoDependencies_ShouldReturnEmpty(t *testing.T) {
	if got := detectFrameworkFromDependencies(nil); got != "" {
		t.Errorf("detectFrameworkFromDependencies(nil) = %q, want empty", got)
	}
}

func TestDetectFrameworkFromDependencies_WhenWebFrameworkPresent_ShouldPickIt(t *testing.T) {
	tests := []struct {
		name string
		deps []DependencyInfo
		want string
	}{
		{
			name: "gin outranks the router and ORM around it",
			deps: []DependencyInfo{
				{Name: "gorilla", Type: "direct"},
				{Name: "gin", Type: "direct"},
				{Name: "gorm", Type: "direct"},
			},
			want: "gin",
		},
		{
			name: "meta-framework outranks the view library it ships",
			deps: []DependencyInfo{
				{Name: "react", Type: "direct"},
				{Name: "nextjs", Type: "direct"},
			},
			want: "nextjs",
		},
		{
			name: "transitively detected meta-framework still outranks a direct view library",
			deps: []DependencyInfo{
				{Name: "vue", Type: "direct"},
				{Name: "nuxt", Type: "transitive"},
			},
			want: "nuxt",
		},
		{
			name: "python web framework",
			deps: []DependencyInfo{{Name: "sqlalchemy", Type: "direct"}, {Name: "fastapi", Type: "direct"}},
			want: "fastapi",
		},
		{
			name: "rust web framework",
			deps: []DependencyInfo{{Name: "tokio", Type: "transitive"}, {Name: "axum", Type: "transitive"}},
			want: "axum",
		},
		{
			name: "a TUI binary has no web framework, so the TUI library is the answer",
			deps: []DependencyInfo{{Name: "cobra", Type: "direct"}, {Name: "bubbletea", Type: "direct"}},
			want: "bubbletea",
		},
		{
			name: "nothing framework-shaped",
			deps: []DependencyInfo{{Name: "testify", Type: "transitive"}, {Name: "uuid", Type: "transitive"}},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectFrameworkFromDependencies(tt.deps); got != tt.want {
				t.Errorf("detectFrameworkFromDependencies() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectFrameworkFromDependencies_WhenCalledTwice_ShouldBeDeterministic(t *testing.T) {
	// Map iteration order must not leak into profile.mg.
	deps := []DependencyInfo{
		{Name: "echo", Type: "direct"},
		{Name: "fiber", Type: "direct"},
		{Name: "gin", Type: "direct"},
	}
	first := detectFrameworkFromDependencies(deps)
	for range 20 {
		if got := detectFrameworkFromDependencies(deps); got != first {
			t.Fatalf("framework detection is not deterministic: %q then %q", first, got)
		}
	}
}

func TestBuildProjectProfile_WhenGoModDeclaresGin_ShouldPopulateFramework(t *testing.T) {
	workspace := t.TempDir()
	goMod := "module demo\n\ngo 1.24\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.10.0\n)\n"
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	profile := ini.buildProjectProfile()

	if profile.Framework != "gin" {
		t.Fatalf("profile.Framework = %q, want \"gin\"", profile.Framework)
	}
}

func TestGenerateFactsFile_WhenFrameworkDetected_ShouldEmitProjectFrameworkFact(t *testing.T) {
	workspace := t.TempDir()
	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	path := filepath.Join(workspace, "profile.mg")

	if _, err := ini.generateFactsFile(path, ProjectProfile{
		ProjectID: "p1", Name: "demo", Language: "go", Framework: "gin",
	}); err != nil {
		t.Fatalf("generateFactsFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read facts: %v", err)
	}
	if !strings.Contains(string(data), "project_framework(/gin).") {
		t.Errorf("profile.mg has no project_framework fact:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// Monorepo manifest discovery (was two hardcoded glob levels)
// ---------------------------------------------------------------------------

func TestFindManifestFiles_WhenModulesAreDeeplyNested_ShouldFindThemAndSkipVendorTrees(t *testing.T) {
	workspace := t.TempDir()
	write := func(rel string) {
		full := filepath.Join(workspace, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("package.json")
	write("apps/web/package.json")                // depth 2 — the old globs saw this
	write("packages/scope/ui/package.json")       // depth 3 — they did not
	write("apps/web/frontend/admin/package.json") // depth 4 — nor this
	write("node_modules/react/package.json")      // must never be treated as ours
	write("apps/web/node_modules/left-pad/package.json")
	write("vendor/thing/package.json")
	write("services/api/deep/deeper/even/package.json") // depth 5 — past the bound

	found := findManifestFiles(workspace, []string{"package.json"}, maxManifestDepth)

	rels := make(map[string]bool, len(found))
	for _, path := range found {
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			t.Fatalf("rel: %v", err)
		}
		rels[filepath.ToSlash(rel)] = true
	}

	for _, want := range []string{
		"package.json",
		"apps/web/package.json",
		"packages/scope/ui/package.json",
		"apps/web/frontend/admin/package.json",
	} {
		if !rels[want] {
			t.Errorf("manifest %s was not discovered; found %v", want, keysOf(rels))
		}
	}
	for _, unwanted := range []string{
		"node_modules/react/package.json",
		"apps/web/node_modules/left-pad/package.json",
		"vendor/thing/package.json",
		"services/api/deep/deeper/even/package.json",
	} {
		if rels[unwanted] {
			t.Errorf("manifest %s should have been skipped; found %v", unwanted, keysOf(rels))
		}
	}
}

func TestDetectDependencies_WhenMonorepoModuleIsThreeLevelsDeep_ShouldSeeItsDependencies(t *testing.T) {
	workspace := t.TempDir()
	modDir := filepath.Join(workspace, "services", "billing", "api")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	goMod := "module demo/services/billing/api\n\ngo 1.24\n\nrequire github.com/gin-gonic/gin v1.10.0\n"
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	deps := ini.detectDependencies()

	found := false
	for _, dep := range deps {
		if dep.Name == "gin" {
			found = true
		}
	}
	if !found {
		t.Fatalf("gin not detected in a depth-3 monorepo module; deps = %+v", deps)
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ---------------------------------------------------------------------------
// Project atoms must reach the JIT corpus
// ---------------------------------------------------------------------------

func TestIngestProjectAtomsIntoCorpus_WhenAtomsBuilt_ShouldBeQueryableAndSurviveReconcile(t *testing.T) {
	workspace := t.TempDir()
	ini := &Initializer{config: InitConfig{Workspace: workspace}}

	profile := ProjectProfile{
		Language:        "go",
		BuildSystem:     "go",
		TestDirectories: []string{"internal"},
		Dependencies:    []DependencyInfo{{Name: "bubbletea", Type: "direct"}},
	}
	atoms := ini.buildProjectAtoms(profile)
	if len(atoms) == 0 {
		t.Fatal("buildProjectAtoms produced nothing for a Go project with tests and a build system")
	}
	ini.projectAtoms = atoms

	dbPath := filepath.Join(workspace, "corpus.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open corpus db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	loader := prompt.NewAtomLoader(nil)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	ingested, err := ini.ingestProjectAtomsIntoCorpus(ctx, db)
	if err != nil {
		t.Fatalf("ingestProjectAtomsIntoCorpus: %v", err)
	}
	if ingested != len(atoms) {
		t.Fatalf("ingested %d atoms, want %d", ingested, len(atoms))
	}

	if got := countCorpusAtom(t, db, "project/go/conventions"); got != 1 {
		t.Errorf("project/go/conventions rows = %d, want 1", got)
	}

	// The language selector is what lets the compiler pick this atom for a Go
	// task; without the tag row it is inert even though it is present.
	var tagged int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM atom_context_tags WHERE atom_id = ? AND dimension = 'lang' AND tag = '/go'`,
		"project/go/conventions",
	).Scan(&tagged); err != nil {
		t.Fatalf("query tags: %v", err)
	}
	if tagged != 1 {
		t.Errorf("language selector rows = %d, want 1", tagged)
	}

	// Reconciliation restores the shipped corpus. Project atoms carry no
	// source_file, which is exactly what keeps them out of the obsolete sweep.
	embedded, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	if _, err := prompt.ReconcilePromptCorpus(ctx, db, embedded.All()); err != nil {
		t.Fatalf("ReconcilePromptCorpus: %v", err)
	}
	if got := countCorpusAtom(t, db, "project/go/conventions"); got != 1 {
		t.Errorf("project atom was swept away by corpus reconciliation (rows = %d)", got)
	}
}

func TestInitializePromptDatabase_WhenProjectAtomsPending_ShouldIngestThemIntoCorpusDB(t *testing.T) {
	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	ini.projectAtoms = ini.buildProjectAtoms(ProjectProfile{Language: "go", BuildSystem: "go"})

	if err := ini.initializePromptDatabase(context.Background(), nerdDir); err != nil {
		t.Fatalf("initializePromptDatabase: %v", err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(nerdDir, "prompts", "corpus.db"))
	if err != nil {
		t.Fatalf("open corpus db: %v", err)
	}
	defer db.Close()

	if got := countCorpusAtom(t, db, "project/go/conventions"); got != 1 {
		t.Errorf("project atoms are still invisible to the JIT compiler (rows = %d)", got)
	}
}

func countCorpusAtom(t *testing.T, db *sql.DB, atomID string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?", atomID).Scan(&count); err != nil {
		t.Fatalf("count atom %s: %v", atomID, err)
	}
	return count
}

func TestComputeContentHash_WhenContentDiffersButLengthMatches_ShouldDiffer(t *testing.T) {
	// The old implementation hashed the *length*, so these collided and the
	// corpus would serve stale content after an edit.
	a := computeContentHash("project/go/conventions", "use errors.Is")
	b := computeContentHash("project/go/conventions", "use errors.As")
	if a == b {
		t.Fatalf("content hash collided for equal-length content: %s", a)
	}
	if len(a) != 64 {
		t.Errorf("content hash length = %d, want a 64-char sha256 hex digest", len(a))
	}
}

// ---------------------------------------------------------------------------
// Tool needs are recorded as Mangle facts instead of printed and discarded
// ---------------------------------------------------------------------------

func TestProjectToolNeedFacts_WhenToolsDetected_ShouldEmitDeclaredMissingToolForFacts(t *testing.T) {
	facts := projectToolNeedFacts([]ToolGenerationRequest{
		{Name: "go_build_tool", Purpose: "build"},
		{Name: "go_test_tool", Purpose: "test"},
		{Name: "go_build_tool", Purpose: "duplicate"},
	})

	if len(facts) != 2 {
		t.Fatalf("facts = %v, want the duplicate collapsed", facts)
	}
	for _, fact := range facts {
		// missing_tool_for(Intent, Capability) bound [/name, /name]
		if !strings.HasPrefix(fact, "missing_tool_for(/project_init, /") || !strings.HasSuffix(fact, ").") {
			t.Errorf("fact %q does not match the Declared missing_tool_for shape", fact)
		}
	}
}

func TestGenerateFactsFile_WhenGoProject_ShouldRecordToolNeedsForTheKernel(t *testing.T) {
	workspace := t.TempDir()
	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	path := filepath.Join(workspace, "profile.mg")

	if _, err := ini.generateFactsFile(path, ProjectProfile{
		ProjectID: "p1", Name: "demo", Language: "go",
	}); err != nil {
		t.Fatalf("generateFactsFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read facts: %v", err)
	}
	if !strings.Contains(string(data), "missing_tool_for(/project_init, /go_build_tool).") {
		t.Errorf("profile.mg does not record the project's tool needs:\n%s", data)
	}
}

func TestGenerateProjectTools_WhenGoProject_ShouldReportRecordedNeedsWithoutGeneratingTools(t *testing.T) {
	ini := &Initializer{config: InitConfig{Workspace: t.TempDir()}}

	recorded, err := ini.generateProjectTools(context.Background(), "", ProjectProfile{Language: "go"})
	if err != nil {
		t.Fatalf("generateProjectTools: %v", err)
	}
	if len(recorded) == 0 {
		t.Fatal("no tool needs recorded for a Go project")
	}
	for _, name := range recorded {
		if strings.TrimSpace(name) == "" {
			t.Errorf("recorded need has an empty name: %v", recorded)
		}
	}
}

// ---------------------------------------------------------------------------
// Package hygiene
// ---------------------------------------------------------------------------

// Kernel evaluation faults dump a Mangle program for debugging. That dump used
// to land inside the scanned source tree, where the scanner indexed it and the
// symbol graph grew predicates "defined_in" a crash artifact.
func TestPackageTree_WhenScanned_ShouldContainNoMangleDebugDumps(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".mg") {
			t.Errorf("unexpected Mangle file %q in the init package tree; kernel dumps belong under .nerd/debug/", name)
		}
	}
}
