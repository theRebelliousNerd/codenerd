package campaign

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// A specialist without a knowledge DB has no knowledge, and asking must not
// create one: every door now looks up the specialist of every task.
func TestLocalSpecialistKnowledgeProvider_MissingDBCreatesNothing(t *testing.T) {
	workspace := t.TempDir()
	provider := NewLocalSpecialistKnowledgeProvider(workspace)

	atoms, err := provider.GetRelevantKnowledge(context.Background(), "coder", "fix the parser", 5)
	if err != nil {
		t.Fatalf("GetRelevantKnowledge: %v", err)
	}
	if len(atoms) != 0 {
		t.Fatalf("atoms = %+v, want none", atoms)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".nerd", "agents")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("looking up a missing specialist created .nerd/agents (stat err = %v)", err)
	}
}
