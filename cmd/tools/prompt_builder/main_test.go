package main

import (
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
		ID: "pin/test", Category: "protocol", Priority: 50, Content: "x",
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
