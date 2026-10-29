package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAtomDefinitions(t *testing.T) {
	dir := t.TempDir()
	yaml := `- id: test_atom
  category: identity
  priority: 50
  is_mandatory: true
  content: "You are a tester."
`
	if err := os.WriteFile(filepath.Join(dir, "atoms.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-YAML file must be ignored by the directory walk.
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0o644)

	atoms, err := extractAtomDefinitions(dir)
	if err != nil {
		t.Fatalf("extractAtomDefinitions: %v", err)
	}
	if len(atoms) != 1 || atoms[0].ID != "test_atom" {
		t.Fatalf("expected 1 atom 'test_atom', got %+v", atoms)
	}
	if atoms[0].Category != "identity" || !atoms[0].IsMandatory {
		t.Errorf("atom fields not parsed: %+v", atoms[0])
	}
}
