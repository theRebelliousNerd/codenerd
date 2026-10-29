package orient

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
	"codenerd/internal/workspace"
)

// treatmentTree writes files (relative path -> body) under a fresh non-git
// root and returns the root.
func treatmentTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func memberFiles(t *testing.T, root string) (string, []string) {
	t.Helper()
	m, err := workspace.Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	return m.Root(), m.Files()
}

// deriveFor runs the census and the policy over root with policy applied.
func deriveFor(t *testing.T, root string, policy config.OrientTreatmentConfig) *workspace.TreatmentArtifact {
	t.Helper()
	canon, files := memberFiles(t, root)
	cfg := config.DefaultOrientConfig()
	cfg.Treatment = &policy
	if probs := cfg.Check("orient"); len(probs) > 0 {
		t.Fatalf("config: %v", probs)
	}
	census, err := measureTreatmentCensus(context.Background(), canon, files, cfg.TreatmentPolicy())
	if err != nil {
		t.Fatal(err)
	}
	art, err := deriveTreatment(context.Background(), cfg, census)
	if err != nil {
		t.Fatal(err)
	}
	return art
}

func rowsOf(art *workspace.TreatmentArtifact) map[string]workspace.TreatmentRow {
	out := map[string]workspace.TreatmentRow{}
	for _, r := range art.Treatments {
		out[r.Dir] = r
	}
	return out
}

func conflictsOf(art *workspace.TreatmentArtifact) map[string]workspace.TreatmentConflict {
	out := map[string]workspace.TreatmentConflict{}
	for _, c := range art.Conflicts {
		out[c.Dir] = c
	}
	return out
}

func TestTreatmentCensusMeasuresBoundedDirectories(t *testing.T) {
	t.Parallel()
	root := treatmentTree(t, map[string]string{
		"a/one.go":         "package a\n",
		"a/two.json":       "{}",
		"a/b/c/d/deep.go":  "package d\n",
		"a/b/c/d/e/x.go":   "package e\n",
		"top.go":           "package top\n",
		"z/README":         "plain",
		"z/y/x/w/v/u.json": "[]",
	})
	canon, files := memberFiles(t, root)
	policy := config.OrientTreatmentConfig{CensusMaxDepth: 2, HeaderReadBudget: 2, HeaderSamplesPerDir: 1, GeneratedMinSamples: 1}.WithDefaults()
	census, err := measureTreatmentCensus(context.Background(), canon, files, policy)
	if err != nil {
		t.Fatal(err)
	}
	stats := map[string][2]int64{}
	exts := map[string]int64{}
	samples := 0
	for _, f := range census.facts {
		switch f.Predicate {
		case "tree_stats":
			stats[f.Args[0].(string)] = [2]int64{f.Args[1].(int64), f.Args[2].(int64)}
		case "tree_ext":
			exts[f.Args[0].(string)+" "+f.Args[1].(string)] = f.Args[2].(int64)
		case "tree_header_sample":
			samples++
		case "tree_depth":
			if f.Args[1].(int64) > 2 {
				t.Errorf("row deeper than the census bound: %v", f)
			}
		}
	}
	// Depth 2 keeps a, a/b, z, z/y; deeper files count toward them.
	if len(stats) != 4 {
		t.Fatalf("census dirs %v, want a, a/b, z, z/y", stats)
	}
	if got := stats["a"]; got[0] != 4 {
		t.Errorf("a counts %v, want 4 files in its subtree", got)
	}
	if got := stats["a/b"]; got[0] != 2 {
		t.Errorf("a/b counts %v, want 2", got)
	}
	if exts["a .go"] != 3 || exts["a .json"] != 1 {
		t.Errorf("extension mix %v", exts)
	}
	if census.summary.Files != 7 {
		t.Errorf("census files %d, want 7", census.summary.Files)
	}
	if census.summary.HeaderReads != 2 || census.summary.HeaderReadBudget != 2 {
		t.Errorf("header reads %d of %d, want exactly the budget", census.summary.HeaderReads, census.summary.HeaderReadBudget)
	}
	if census.summary.UnsampledDirectories == 0 {
		t.Error("a spent budget must report unsampled directories")
	}
	if samples != 4 {
		t.Errorf("header sample rows %d, want one per census dir", samples)
	}
}

func TestTreatmentInheritanceIsSegmentAware(t *testing.T) {
	t.Parallel()
	root := treatmentTree(t, map[string]string{
		"data/x/rows.go":   "package x\n",
		"database/conn.go": "package database\n",
	})
	art := deriveFor(t, root, config.OrientTreatmentConfig{Overrides: []config.TreatmentOverride{
		{Path: "data", Treatment: config.TreatmentIndexNamesOnly, Basis: config.TreatmentBasisOperational},
	}})
	rows := rowsOf(art)
	if r := rows["data/x"]; r.Treatment != workspace.TreatmentIndexNamesOnly || r.Scope != "data" || r.Authority != "/operator" {
		t.Fatalf("data/x %+v, want names-only inherited from data", r)
	}
	if r, ok := rows["database"]; ok {
		t.Fatalf("data governs database: %+v", r)
	}
}

func TestTreatmentAuthorityBeforeSpecificity(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"a/b/c/f.go": "package c\n",
		"p/q/f.go":   "package q\n",
		"s/t/f.go":   "package t\n",
		"u/v/f.go":   "package v\n",
	}
	root := treatmentTree(t, files)
	art := deriveFor(t, root, config.OrientTreatmentConfig{Overrides: []config.TreatmentOverride{
		// A hard parent exclusion dominates an operator child promotion.
		{Path: "a", Treatment: config.TreatmentExclude, Basis: config.TreatmentBasisAccessRestriction},
		{Path: "a/b", Treatment: config.TreatmentIndexAndParse, Basis: config.TreatmentBasisOperational},
		// An operator child override refines an operational parent.
		{Path: "p", Treatment: config.TreatmentIndexNamesOnly, Basis: config.TreatmentBasisOperational},
		{Path: "p/q", Treatment: config.TreatmentIndexAndParse, Basis: config.TreatmentBasisOperational},
		// Restrictions compose: the strictest holds, wherever it is stated.
		{Path: "s", Treatment: config.TreatmentIndexNamesOnly, Basis: config.TreatmentBasisAccessRestriction},
		{Path: "s/t", Treatment: config.TreatmentExclude, Basis: config.TreatmentBasisAccessRestriction},
		{Path: "u", Treatment: config.TreatmentExclude, Basis: config.TreatmentBasisAccessRestriction},
		{Path: "u/v", Treatment: config.TreatmentIndexNamesOnly, Basis: config.TreatmentBasisAccessRestriction},
	}})
	rows := rowsOf(art)
	for dir, want := range map[string]struct {
		t     workspace.Treatment
		scope string
		auth  string
	}{
		"a/b":   {workspace.TreatmentExclude, "a", "/access_restriction"},
		"a/b/c": {workspace.TreatmentExclude, "a", "/access_restriction"},
		"p":     {workspace.TreatmentIndexNamesOnly, "p", "/operator"},
		"p/q":   {workspace.TreatmentIndexAndParse, "p/q", "/operator"},
		"s":     {workspace.TreatmentIndexNamesOnly, "s", "/access_restriction"},
		"s/t":   {workspace.TreatmentExclude, "s/t", "/access_restriction"},
		"u/v":   {workspace.TreatmentExclude, "u", "/access_restriction"},
	} {
		r := rows[dir]
		if r.Treatment != want.t || r.Scope != want.scope || r.Authority != want.auth {
			t.Errorf("%s = %+v, want %+v", dir, r, want)
		}
	}
	if len(art.Conflicts) != 0 {
		t.Errorf("unexpected conflicts %+v", art.Conflicts)
	}
}

func TestTreatmentInferenceAndConflicts(t *testing.T) {
	t.Parallel()
	gen := "// Code generated by protoc-gen-go. DO NOT EDIT.\npackage api\n"
	big := strings.Repeat("x", 600)
	root := treatmentTree(t, map[string]string{
		"vendor/lib/a.go":         "package lib\n",
		"vendor/lib/deep/b.go":    "package deep\n",
		"api/a_server.go":         "package api\n",
		"api/b_handler.go":        "package api\n",
		"api/gen/a.pb.go":         gen,
		"api/gen/b.pb.go":         gen,
		"api/gen/c.pb.go":         gen,
		"api/gen/d.pb.go":         gen,
		"seeds/one.csv":           big,
		"seeds/two.json":          big,
		"tests/fixtures/x.py":     "def f(): pass\n",
		"tests/fixtures/y.py":     "def g(): pass\n",
		"tests/fixtures/sub/z.py": "def h(): pass\n",
		"golden/testdata/a.txt":   "expected",
		"src/main.go":             "package main\n",
	})
	art := deriveFor(t, root, config.OrientTreatmentConfig{DataMinBytes: 1000})
	rows := rowsOf(art)
	expect := func(dir string, want workspace.Treatment, reason string) {
		t.Helper()
		r, ok := rows[dir]
		if !ok || r.Treatment != want {
			t.Errorf("%s = %+v, want %s", dir, r, want)
			return
		}
		if reason != "" && !strings.Contains(strings.Join(r.Reasons, ","), reason) {
			t.Errorf("%s reasons %v, want %s", dir, r.Reasons, reason)
		}
		if r.Authority != "/inferred" {
			t.Errorf("%s authority %s", dir, r.Authority)
		}
	}
	expect("vendor", workspace.TreatmentIndexNamesOnly, "/name_convention")
	expect("vendor/lib/deep", workspace.TreatmentIndexNamesOnly, "/name_convention")
	expect("api/gen", workspace.TreatmentIndexNamesOnly, "/generated_headers")
	expect("seeds", workspace.TreatmentIndexNamesOnly, "/data_extension_mix")
	expect("golden/testdata", workspace.TreatmentIndexNamesOnly, "/name_convention")
	for _, dir := range []string{"src", "api", "tests", "golden"} {
		if r, ok := rows[dir]; ok {
			t.Errorf("unknown %s must stay unclassified, got %+v", dir, r)
		}
	}
	for _, r := range art.Treatments {
		if r.Treatment == workspace.TreatmentExclude {
			t.Errorf("inference derived an exclusion: %+v", r)
		}
	}
	conflicts := conflictsOf(art)
	for _, dir := range []string{"tests/fixtures", "tests/fixtures/sub"} {
		c, ok := conflicts[dir]
		if !ok || c.Scope != "tests/fixtures" || len(c.Treatments) != 2 {
			t.Errorf("%s conflict %+v, want fixtures name contested by source mix", dir, c)
		}
		if _, ok := rows[dir]; ok {
			t.Errorf("%s has a row despite its conflict", dir)
		}
	}
	// An operator decision resolves the conflict for the scope.
	art = deriveFor(t, root, config.OrientTreatmentConfig{DataMinBytes: 1000, Overrides: []config.TreatmentOverride{
		{Path: "tests/fixtures", Treatment: config.TreatmentIndexAndParse, Basis: config.TreatmentBasisOperational},
		{Path: "nowhere/at/all", Treatment: config.TreatmentExclude, Basis: config.TreatmentBasisOperational},
	}})
	if r := rowsOf(art)["tests/fixtures/sub"]; r.Treatment != workspace.TreatmentIndexAndParse || r.Authority != "/operator" {
		t.Errorf("operator resolution %+v", r)
	}
	if len(conflictsOf(art)) != 0 {
		t.Errorf("conflicts remain %+v", art.Conflicts)
	}
	if len(art.UnmatchedOverrides) != 1 || art.UnmatchedOverrides[0] != "nowhere/at/all" {
		t.Errorf("unmatched overrides %v", art.UnmatchedOverrides)
	}
}

func writeTreatmentConfig(t *testing.T, root string, policy *config.OrientTreatmentConfig) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"orient": map[string]any{"treatment": policy}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd", "config.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshTreatmentArtifactIsDeterministicAndGenerationBound(t *testing.T) {
	t.Parallel()
	root := treatmentTree(t, map[string]string{
		"third_party/lib/a.go": "package lib\n",
		"src/main.go":          "package main\n",
	})
	writeTreatmentConfig(t, root, &config.OrientTreatmentConfig{})
	ctx := context.Background()
	first, err := refreshTreatment(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Written || first.Rows == 0 {
		t.Fatalf("first refresh %+v", first)
	}
	m, err := workspace.For(root)
	if err != nil {
		t.Fatal(err)
	}
	path := workspace.TreatmentArtifactPath(m.Root())
	body1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	art, err := workspace.ReadTreatmentArtifact(m.Root())
	if err != nil {
		t.Fatal(err)
	}
	if art.SchemaVersion != workspace.TreatmentArtifactSchema || art.Workspace != m.Root() ||
		art.Generation != m.Generation() || !strings.HasPrefix(art.PolicyDigest, "sha256:") || !strings.HasPrefix(art.MeasurementDigest, "sha256:") {
		t.Fatalf("envelope %+v", art)
	}
	second, err := refreshTreatment(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if second.Written {
		t.Fatal("an unchanged generation and policy re-ran the census")
	}
	// Rebuilt from scratch, the same inputs encode to the same bytes.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := refreshTreatment(ctx, root); err != nil {
		t.Fatal(err)
	}
	body2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body1, body2) {
		t.Fatalf("artifact is not deterministic:\n%s\n---\n%s", body1, body2)
	}
	// A new member moves the generation; an edited override moves the policy.
	if err := os.WriteFile(filepath.Join(root, "src", "extra.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	third, err := refreshTreatment(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !third.Written || third.Generation == first.Generation {
		t.Fatalf("membership change not re-derived: %+v", third)
	}
	writeTreatmentConfig(t, root, &config.OrientTreatmentConfig{Overrides: []config.TreatmentOverride{
		{Path: "src", Treatment: config.TreatmentExclude, Basis: config.TreatmentBasisOperational},
	}})
	fourth, err := refreshTreatment(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !fourth.Written {
		t.Fatal("override edit not re-derived")
	}
	if r := rowsOf(mustArtifact(t, m.Root()))["src"]; r.Treatment != workspace.TreatmentExclude {
		t.Fatalf("src = %+v after the override", r)
	}
	// An invalid override is refused, never applied.
	writeTreatmentConfig(t, root, &config.OrientTreatmentConfig{Overrides: []config.TreatmentOverride{
		{Path: "src", Treatment: config.TreatmentIndexAndParse, Basis: config.TreatmentBasisAccessRestriction},
	}})
	if _, err := refreshTreatment(ctx, root); err == nil {
		t.Fatal("an access restriction granting index_and_parse was accepted")
	}
}

func mustArtifact(t *testing.T, root string) *workspace.TreatmentArtifact {
	t.Helper()
	art, err := workspace.ReadTreatmentArtifact(root)
	if err != nil {
		t.Fatal(err)
	}
	return art
}

func TestTreatmentRulesLoadInTheFullProgram(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultOrientConfig()
	e, err := NewEngine(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.Assert([]types.Fact{
		{Predicate: "tree_dir", Args: []any{"vendor"}},
		{Predicate: "tree_depth", Args: []any{"vendor", int64(1)}},
		{Predicate: "tree_name", Args: []any{"vendor", "vendor"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.Evaluate(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err := e.Query("tree_treatment")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || types.ExtractString(rows[0].Args[1]) != "/index_names_only" {
		t.Fatalf("full program treatment rows %v", rows)
	}
}

// Refresh is the production path: boot and every incremental scan call it,
// and it must publish the artifact on a cached no-stale refresh too.
func TestRefreshPublishesTreatmentArtifact(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runGit(t, root, "init")
	for rel, body := range map[string]string{"DESIGN.md": "# Design\n", "pkg/testdata/case.json": "{}", "pkg/lib.go": "package pkg\n"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "origin")
	s, err := Measure(ctx, root, config.DefaultOrientConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	e, err := s.Engine(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := s.Save(root, e); err != nil {
		t.Fatal(err)
	}
	first, err := Refresh(ctx, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Refreshed || !first.Treatment.Written {
		t.Fatalf("first refresh %+v, want an unchanged orientation and a written artifact", first)
	}
	m, err := workspace.For(root)
	if err != nil {
		t.Fatal(err)
	}
	if r := rowsOf(mustArtifact(t, m.Root()))["pkg/testdata"]; r.Treatment != workspace.TreatmentIndexNamesOnly {
		t.Fatalf("pkg/testdata = %+v", r)
	}
	second, err := Refresh(ctx, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Treatment.Written || second.Treatment.Generation != first.Treatment.Generation {
		t.Fatalf("memoized refresh re-derived treatment: %+v", second.Treatment)
	}
}
