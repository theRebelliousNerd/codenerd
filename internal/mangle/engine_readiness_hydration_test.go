package mangle

import (
	"errors"
	"testing"
)

func TestEngineReadiness_HydrationBoundary(t *testing.T) {
	for _, mode := range []string{"invalid_prefix", "invalid_first", "load_error", "success", "manual_success"} {
		t.Run(mode, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.AutoEval = true
			e := readinessEngine(t, cfg)
			prefix := Fact{Predicate: "excluded", Args: []any{"work.go"}}
			bad := Fact{Predicate: "unknown"}
			p := &mockPersistence{facts: []Fact{prefix, bad}}
			switch mode {
			case "invalid_first":
				p.facts = []Fact{bad}
			case "load_error":
				p.err = errors.New("fixture load failed")
			case "success", "manual_success":
				p.facts = []Fact{prefix}
				if mode == "manual_success" {
					e.ToggleAutoEval(false)
				}
			}
			e.persistence = p
			err := e.WarmFromPersistence(t.Context())
			switch mode {
			case "invalid_prefix":
				if err == nil {
					t.Fatal("hydration failure fixture succeeded")
				}
				if rows, readErr := e.GetFacts("excluded"); readErr != nil || len(rows) != 1 {
					t.Fatalf("hydration lost accepted prefix: %v / %v", rows, readErr)
				}
				assertDerivedUnavailable(t, e, "eligible", err)
				if err := e.AddFact("tag", "recover"); err != nil {
					t.Fatal(err)
				}
				if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 0 {
					t.Fatalf("hydration did not restore auto-evaluation and recover: %v / %v", rows, err)
				}
			case "invalid_first", "load_error":
				if err == nil {
					t.Fatal("hydration rejection fixture succeeded")
				}
				if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 1 {
					t.Fatalf("rejected hydration poisoned unchanged state: %v / %v", rows, err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				if mode == "manual_success" {
					want = 1
				}
				if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != want {
					t.Fatalf("successful hydration changed evaluation contract: %v / %v", rows, err)
				}
			}
		})
	}
}
