package store

import (
	"path/filepath"
	"testing"
)

// The thirteen selector columns of a prompt atom were hydrated with bare
// json.Unmarshal calls. A malformed column produced an atom with no dimensions
// — which does not error, does not warn, and does not disappear: it stays in
// the pool and matches nothing, or, for a dimension where empty means "any",
// matches everything. Either way the JIT prompt is silently wrong.

func TestScanPromptAtoms_SelectorsRoundTrip(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "atoms.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	atom := &PromptAtom{
		AtomID:           "identity/coder/mission",
		Version:          1,
		Content:          "be precise",
		Category:         "identity",
		OperationalModes: []string{"/active", "/debugging"},
		IntentVerbs:      []string{"/fix"},
		ShardTypes:       []string{"/coder"},
		DependsOn:        []string{"identity/core"},
	}
	if err := s.StorePromptAtom(atom); err != nil {
		t.Fatalf("store atom: %v", err)
	}

	got, err := s.LoadPromptAtomsByCategory("identity")
	if err != nil {
		t.Fatalf("load atoms: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one atom, got %d", len(got))
	}
	if len(got[0].OperationalModes) != 2 || got[0].OperationalModes[0] != "/active" {
		t.Errorf("operational modes did not survive the round trip: %v", got[0].OperationalModes)
	}
	if len(got[0].IntentVerbs) != 1 || got[0].IntentVerbs[0] != "/fix" {
		t.Errorf("intent verbs did not survive the round trip: %v", got[0].IntentVerbs)
	}
	if len(got[0].DependsOn) != 1 {
		t.Errorf("depends_on did not survive the round trip: %v", got[0].DependsOn)
	}
}

func TestDecodeAtomSelector_CorruptColumnLeavesDimensionNil(t *testing.T) {
	dst := []string{"/stale"}
	decodeAtomSelector("identity/coder/mission", "intent_verbs", `["/fix", `, &dst)
	if dst != nil {
		t.Errorf("a column that does not parse must not leave a half-decoded selector: %v", dst)
	}
}

func TestDecodeAtomSelector_EmptyColumnIsNotCorruption(t *testing.T) {
	for _, raw := range []string{"", "  ", "null"} {
		var dst []string
		decodeAtomSelector("identity/coder/mission", "intent_verbs", raw, &dst)
		if dst != nil {
			t.Errorf("empty column %q should leave the dimension unset, got %v", raw, dst)
		}
	}
}
