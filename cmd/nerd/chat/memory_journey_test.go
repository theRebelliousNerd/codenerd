package chat

import (
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

// A person says "remember that I prefer tabs", later "forget that": the first
// is stored as the type hydration loads (so it is recallable), the second
// removes that record, and each turn tells the person what happened.
func TestRememberThenForget_TheSavedPreferenceIsRecallableAndRemovable(t *testing.T) {
	t.Parallel()
	db, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel()
	m.kernel = kernel
	m.localDB = db

	var warnings []string
	m.processMemoryOperations(perception.Intent{MemoryOperations: []articulation.MemoryOperation{
		{Op: "promote_to_long_term", Key: "preference", Value: "I prefer tabs"},
	}}, &warnings)
	if len(warnings) != 0 {
		t.Fatalf("remember warned: %v", warnings)
	}
	prefs, err := db.UserPreferences()
	if err != nil || len(prefs) != 1 {
		t.Fatalf("hydration's view of the remembered preference = %v, %v; want it recallable", prefs, err)
	}

	m.processMemoryOperations(perception.Intent{MemoryOperations: []articulation.MemoryOperation{
		{Op: "forget", Key: "prefer tabs", Domain: articulation.MemoryDomainUserMemory},
	}}, &warnings)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "removed the saved long-term preference") {
		t.Fatalf("forget acknowledgement = %v", warnings)
	}
	if prefs, _ := db.UserPreferences(); len(prefs) != 0 {
		t.Fatalf("forget left %v", prefs)
	}

	warnings = nil
	m.processMemoryOperations(perception.Intent{MemoryOperations: []articulation.MemoryOperation{
		{Op: "forget", Key: "prefer tabs", Domain: articulation.MemoryDomainUserMemory},
	}}, &warnings)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "nothing was removed") {
		t.Fatalf("forgetting an absent preference must say so, got %v", warnings)
	}
}
