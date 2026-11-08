package main

import (
	"codenerd/internal/prompt"
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestParseYAMLFileLoadsContent(t *testing.T) {
	dir := t.TempDir()
	contentPath := filepath.Join(dir, "content.txt")
	if err := os.WriteFile(contentPath, []byte("content body"), 0644); err != nil {
		t.Fatalf("write content file: %v", err)
	}

	yamlPath := filepath.Join(dir, "atoms.yaml")
	yamlData := []byte(`
- id: /alpha
  category: protocol
  priority: 10
  is_mandatory: true
  content: "inline text"
- id: /beta
  category: protocol
  priority: 20
  is_mandatory: false
  content_file: "content.txt"
`)
	if err := os.WriteFile(yamlPath, yamlData, 0644); err != nil {
		t.Fatalf("write yaml file: %v", err)
	}

	atoms, err := parseYAMLFile(yamlPath, dir)
	if err != nil {
		t.Fatalf("parse YAML: %v", err)
	}
	if len(atoms) != 2 {
		t.Fatalf("expected 2 atoms, got %d", len(atoms))
	}

	found := map[string]ProcessedAtom{}
	for _, atom := range atoms {
		found[atom.ID] = atom
	}

	if found["/alpha"].Content != "inline text" {
		t.Fatalf("unexpected inline content: %q", found["/alpha"].Content)
	}
	if found["/beta"].Content != "content body" {
		t.Fatalf("unexpected content_file content: %q", found["/beta"].Content)
	}
	if found["/beta"].ContentHash == "" {
		t.Fatalf("expected content hash to be set")
	}
	if found["/beta"].TokenCount <= 0 {
		t.Fatalf("expected token count > 0")
	}
	if found["/beta"].SourceFile == "" {
		t.Fatalf("expected source file to be set")
	}
}

func TestValidateAtomsDetectsDuplicatesAndMissingDeps(t *testing.T) {
	atoms := []ProcessedAtom{
		{AtomDefinition: AtomDefinition{ID: "/alpha"}, SourceFile: "a.yaml"},
		{AtomDefinition: AtomDefinition{ID: "/alpha"}, SourceFile: "b.yaml"},
	}
	if err := validateAtoms(atoms); err == nil {
		t.Fatalf("expected duplicate ID error")
	}

	atoms = []ProcessedAtom{
		{AtomDefinition: AtomDefinition{ID: "/alpha", DependsOn: []string{"/missing"}}, SourceFile: "a.yaml"},
	}
	if err := validateAtoms(atoms); err == nil {
		t.Fatalf("expected missing dependency error")
	}
}

func TestEncodeFloat32Slice(t *testing.T) {
	vec := []float32{1.5, -2.0}
	blob := encodeFloat32Slice(vec)
	if len(blob) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(blob))
	}
	got := math.Float32frombits(binary.LittleEndian.Uint32(blob[:4]))
	if got != vec[0] {
		t.Fatalf("expected %v, got %v", vec[0], got)
	}
}

func TestNullableStringAndEmbeddingText(t *testing.T) {
	if nullableString("") != nil {
		t.Fatalf("expected nil for empty string")
	}
	if val := nullableString("ok"); val != "ok" {
		t.Fatalf("expected \"ok\", got %v", val)
	}

	atom := ProcessedAtom{AtomDefinition: AtomDefinition{Description: "summary", Content: "body"}}
	if got := getTextForEmbedding(atom); got != "summary" {
		t.Fatalf("expected description to win, got %q", got)
	}

	atom = ProcessedAtom{AtomDefinition: AtomDefinition{Content: "body"}}
	if got := getTextForEmbedding(atom); got != "body" {
		t.Fatalf("expected content fallback, got %q", got)
	}
}

// The corpus DB must carry pin and capability-gating tags: a shipped DB that
// drops model/provider/requires_tool rows loads unpinned, tool-agnostic atoms.
func TestGenerateAndStoreAtoms_PersistsPinAndToolTags(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "corpus.db")
	db, err := createDatabase(dbPath)
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	defer db.Close()

	atoms := []ProcessedAtom{{AtomDefinition: AtomDefinition{
		ID: "pin/test", Category: "protocol", Content: "x",
		ShardTypes: []string{"/coder"}, Models: []string{"/claude_opus_4"},
		Providers: []string{"/anthropic"}, RequiresTools: []string{"read_file"},
	}, TokenCount: 1, ContentHash: "h", SourceFile: "f.yaml"}}
	if err := generateAndStoreAtoms(context.Background(), nil, db, atoms, true); err != nil {
		t.Fatalf("store atoms: %v", err)
	}

	rows, err := db.Query("SELECT dimension, tag FROM atom_context_tags WHERE atom_id = ?", "pin/test")
	if err != nil {
		t.Fatalf("query tags: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var dim, tag string
		if err := rows.Scan(&dim, &tag); err != nil {
			t.Fatalf("scan tag: %v", err)
		}
		got[dim] = tag
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	want := map[string]string{"shard": "/coder", "model": "/claude_opus_4", "provider": "/anthropic", "requires_tool": "read_file"}
	for dim, tag := range want {
		if got[dim] != tag {
			t.Errorf("dimension %q = %q, want %q (full row set %v)", dim, got[dim], tag, got)
		}
	}
}

func TestValidateAtomsRejectsMissingCategory(t *testing.T) {
	atoms := []ProcessedAtom{
		{AtomDefinition: AtomDefinition{ID: "/alpha", Category: "protocol"}, SourceFile: "a.yaml"},
		{AtomDefinition: AtomDefinition{ID: "/beta"}, SourceFile: "b.yaml"},
	}
	if err := validateAtoms(atoms); err == nil {
		t.Fatalf("expected missing category error for /beta")
	}
}

func TestParseYAMLFileSkipsAtomsWithoutContent(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "atoms.yaml")
	yamlData := []byte("- id: /empty\n  category: protocol\n- id: \"\"\n  category: protocol\n  content: \"no id\"\n- id: /ok\n  category: protocol\n  content: \"hello world\"\n")
	if err := os.WriteFile(yamlPath, yamlData, 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	atoms, err := parseYAMLFile(yamlPath, dir)
	if err != nil {
		t.Fatalf("parse YAML: %v", err)
	}
	if len(atoms) != 1 || atoms[0].ID != "/ok" {
		t.Fatalf("expected only /ok, got %+v", atoms)
	}
}

func TestReconcileEmbeddedSeedRejectsCustomInput(t *testing.T) {
	if _, err := reconcileEmbeddedSeed(context.Background(), t.TempDir(), filepath.Join(t.TempDir(), "out.db")); err == nil {
		t.Fatalf("expected custom -input conflict error")
	}
}

func TestCreateEmbeddingEngineRequiresModelEnv(t *testing.T) {
	t.Setenv("GENAI_EMBEDDING_MODEL", "")
	if _, err := createEmbeddingEngine("dummy-key"); err == nil {
		t.Fatalf("expected missing GENAI_EMBEDDING_MODEL error")
	}
}

func TestExtractAtomDefinitionsRejectsMissingDir(t *testing.T) {
	if _, err := extractAtomDefinitions(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatalf("expected walk error for missing directory")
	}
}

func TestEncodeFloat32SliceRoundTripsAllElements(t *testing.T) {
	vec := []float32{0, 1.5, -2.25, 3.75}
	blob := encodeFloat32Slice(vec)
	if len(blob) != len(vec)*4 {
		t.Fatalf("expected %d bytes, got %d", len(vec)*4, len(blob))
	}
	for i, want := range vec {
		got := math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
		if got != want {
			t.Fatalf("element %d: expected %v, got %v", i, want, got)
		}
	}
	if len(encodeFloat32Slice(nil)) != 0 {
		t.Fatalf("expected empty blob for nil vector")
	}
}

func TestNearestByEmbeddingSkipsRelatedAndRanksBySimilarity(t *testing.T) {
	mk := func(id, section string) *prompt.PromptAtom {
		a := prompt.NewPromptAtom(id, prompt.AtomCategory("protocol"), "content "+id)
		a.SectionOf = section
		return a
	}
	head := mk("head", "")
	sec := mk("head/section", "head")
	other := mk("other", "")
	// head and its section are related: they must never be reported as neighbours.
	atoms := []*prompt.PromptAtom{head, sec, other}
	vecs := [][]float32{{1, 0}, {1, 0}, {0, 1}}
	got := nearestByEmbedding(atoms, vecs)
	if _, ok := got[head.ID]; ok {
		t.Fatalf("related section must not appear as neighbour: %v", got)
	}
	// Orthogonal vectors are below the similarity threshold: no neighbours at all.
	if len(got) != 0 {
		t.Fatalf("expected no neighbours for orthogonal vectors, got %v", got)
	}
	// Near-identical vectors are neighbours, rounded to three decimals.
	atoms = []*prompt.PromptAtom{mk("a", ""), mk("b", ""), mk("c", "")}
	vecs = [][]float32{{1, 0}, {1, 0.001}, {0, 1}}
	got = nearestByEmbedding(atoms, vecs)
	sim, ok := got["a"]["b"]
	if !ok {
		t.Fatalf("expected a->b neighbour, got %v", got)
	}
	if sim < 0.999 || sim > 1.0 {
		t.Fatalf("expected similarity near 1.0, got %v", sim)
	}
	if _, ok := got["a"]["c"]; ok {
		t.Fatalf("dissimilar atom must not appear as neighbour: %v", got)
	}
}

func TestImportanceOfReturnsNilWhenUnscored(t *testing.T) {
	ledger := &prompt.ImportanceLedger{Atoms: map[string]prompt.ImportanceRecord{}}
	atom := ProcessedAtom{AtomDefinition: AtomDefinition{ID: "/alpha"}, ContentHash: "hash-a"}
	if got := importanceOf(ledger, atom); got != nil {
		t.Fatalf("expected nil for unscored atom, got %v", got)
	}
}
