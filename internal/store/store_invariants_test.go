package store

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

// Every selector and composition field of a PromptAtom survives a store and a
// load. It walks the struct by reflection, so a selector dimension added
// later without a column (or without a scan) fails here instead of shipping
// atoms that match nothing -- or, where empty means "any", everything.
func TestPromptAtom_EverySliceFieldRoundTrips(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "atoms.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	atom := &PromptAtom{AtomID: "probe/every-selector", Version: 1, Content: "probe", Category: "protocol",
		Priority: 42, IsMandatory: true, IsExclusive: "probe-group"}
	v := reflect.ValueOf(atom).Elem()
	var sliceFields []string
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if f.Type == reflect.TypeOf([]string(nil)) {
			v.Field(i).Set(reflect.ValueOf([]string{"/" + f.Name + "_a", "/" + f.Name + "_b"}))
			sliceFields = append(sliceFields, f.Name)
		}
	}
	if len(sliceFields) < 13 {
		t.Fatalf("found %d []string fields on PromptAtom, expected the 11 selectors plus depends_on/conflicts_with", len(sliceFields))
	}
	if err := s.StorePromptAtom(atom); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPromptAtom(atom.AtomID)
	if err != nil || got == nil {
		t.Fatalf("GetPromptAtom: %v (%v)", err, got)
	}
	gv := reflect.ValueOf(got).Elem()
	for _, name := range sliceFields {
		want := v.FieldByName(name).Interface()
		if have := gv.FieldByName(name).Interface(); !reflect.DeepEqual(have, want) {
			t.Errorf("%s: stored %v, loaded %v", name, want, have)
		}
	}
	if got.Priority != 42 || !got.IsMandatory || got.IsExclusive != "probe-group" {
		t.Errorf("composition fields lost: priority=%d mandatory=%v exclusive=%q", got.Priority, got.IsMandatory, got.IsExclusive)
	}
}

// A fresh tools.db has statistics: zeros. SUM over no rows is NULL, and the
// stats query scanned it into an int, so every stats read of an empty
// journal failed -- /cleanup-tools answered a new workspace with an error.
func TestToolStore_GetStatsOnAnEmptyJournal(t *testing.T) {
	ts, err := NewToolStore(filepath.Join(t.TempDir(), "tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	stats, err := ts.GetStats()
	if err != nil {
		t.Fatalf("GetStats on an empty journal: %v", err)
	}
	if stats.TotalExecutions != 0 || stats.SuccessCount != 0 || stats.FailureCount != 0 {
		t.Fatalf("empty journal stats = %+v", stats)
	}
}

// A reader of the world cache never sees one scan's fingerprint with another
// scan's facts, while writers replace the file's facts concurrently. The
// incremental scanner trusts that pair to decide whether to reparse.
func TestWorldCache_FingerprintAndFactsNeverTear(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "world.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	const path = "internal/probe/probe.go"
	versions := map[string][]WorldFactInput{
		"fp-a": {{Predicate: "symbol_graph", Args: []any{"a1"}}},
		"fp-b": {{Predicate: "symbol_graph", Args: []any{"b1"}}, {Predicate: "symbol_graph", Args: []any{"b2"}}},
	}
	if err := s.ReplaceWorldFactsForFile(path, "fast", "fp-a", versions["fp-a"]); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				fp := "fp-a"
				if (i+w)%2 == 1 {
					fp = "fp-b"
				}
				if err := s.UpsertWorldFile(WorldFileMeta{Path: path, Lang: "go", Fingerprint: fp}); err != nil {
					errs <- err
					return
				}
				if err := s.ReplaceWorldFactsForFile(path, "fast", fp, versions[fp]); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				facts, fp, err := s.LoadWorldFactsForFile(path, "fast")
				if err != nil {
					errs <- err
					return
				}
				want, ok := versions[fp]
				if !ok {
					errs <- fmt.Errorf("read an unknown fingerprint %q", fp)
					return
				}
				if len(facts) != len(want) {
					errs <- fmt.Errorf("fingerprint %s read with %d facts, want %d: a torn replace", fp, len(facts), len(want))
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
