package factsnap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

func TestLegacyJSONDirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "facts.json")
	want := []types.Fact{
		{Predicate: "p1", Args: []any{"a", "b"}},
		{Predicate: "p2", Args: []any{"c"}},
	}
	data, _ := json.Marshal(want)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LegacyJSON(path)
	if err != nil {
		t.Fatalf("LegacyJSON: %v", err)
	}
	if len(got) != 2 || got[0].Predicate != "p1" || got[1].Predicate != "p2" {
		t.Errorf("LegacyJSON round-trip mismatch: %+v", got)
	}

	// Missing file and malformed JSON both return errors (not panics).
	if _, err := LegacyJSON(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("missing file should error")
	}
	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(bad, []byte("{not json"), 0o644)
	if _, err := LegacyJSON(bad); err == nil {
		t.Error("malformed JSON should error")
	}
}
