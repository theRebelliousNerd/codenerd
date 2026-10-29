package store

import (
	"path/filepath"
	"testing"
)

// The chat remember path stored (predicate "preference", type "learned") while
// hydration loads type "preference": a remembered preference sat in the store
// and never reached a turn. Opening the store adopts exactly those rows. A
// "learned" row of any other predicate is a model-sent kernel update, not a
// user instruction, and keeps its type.
func TestNewLocalStore_AdoptsOnlyRememberedPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.db")
	db, err := NewLocalStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.StoreFact("preference", []any{"use tabs"}, "learned", 10); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreFact("project_uses", []any{"sqlite"}, "learned", 5); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreFact("preference", []any{"use go 1.24"}, "preference", 10); err != nil {
		t.Fatal(err)
	}
	if prefs, _ := db.UserPreferences(); len(prefs) != 1 {
		t.Fatalf("before reopening, hydration sees %d preferences, want only the Piggyback-promoted one", len(prefs))
	}
	db.Close()

	db, err = NewLocalStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	prefs, err := db.UserPreferences()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range prefs {
		got[p.Args[0].(string)] = true
	}
	if len(prefs) != 2 || !got["use tabs"] || !got["use go 1.24"] {
		t.Fatalf("remembered preferences after reopen = %v, want both preferences", got)
	}
	learned, _ := db.LoadAllFacts("learned")
	if len(learned) != 1 || learned[0].Predicate != "project_uses" {
		t.Fatalf("an unrelated learned row was retyped: %v", learned)
	}
}

func TestDeleteStoredFact_RemovesOneRowByIdentity(t *testing.T) {
	db, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, v := range []string{"use tabs", "use spaces"} {
		if err := db.StoreFact("preference", []any{v}, UserPreferenceFactType, 10); err != nil {
			t.Fatal(err)
		}
	}
	prefs, _ := db.UserPreferences()
	n, err := db.DeleteStoredFact(prefs[0].ID)
	if err != nil || n != 1 {
		t.Fatalf("DeleteStoredFact = %d, %v", n, err)
	}
	if n, _ := db.DeleteStoredFact(prefs[0].ID); n != 0 {
		t.Fatalf("a second delete of the same row removed %d rows, want 0 (already absent)", n)
	}
	if left, _ := db.UserPreferences(); len(left) != 1 {
		t.Fatalf("%d preferences left, want the other one", len(left))
	}
}
