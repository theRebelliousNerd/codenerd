package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRankSignaturesForTarget_TierOrder pins the three-tier relevance order.
//
// The fallback used to be directory order, so a target file that exports
// nothing of its own spent all eight signature slots on whichever sibling
// sorted first alphabetically. On internal/core/kernel.go — a 16-line package
// marker — that was eight symbols from action_validator.go followed by
// "… and 592 more": tokens spent, signal zero.
func TestRankSignaturesForTarget_TierOrder(t *testing.T) {
	sigs := []SymbolSignature{
		{Name: "ZebraBackground", File: "zzz.go", Exported: true},
		{Name: "AlphaBackground", File: "aaa.go", Exported: true},
		{Name: "Referenced", File: "dep.go", Exported: true},
		{Name: "DefinedHere", File: "target.go", Exported: true},
		{Name: "unexportedHere", File: "target.go", Exported: false},
	}

	ranked, pool := rankSignaturesForTarget(sigs, "target.go", []string{"Referenced"}, nil)
	if pool != 4 {
		t.Fatalf("pool = %d, want 4 exported symbols", pool)
	}
	want := []string{"DefinedHere", "Referenced", "AlphaBackground", "ZebraBackground"}
	for i, name := range want {
		if ranked[i].Name != name {
			t.Fatalf("rank %d = %q, want %q (full order %v)", i, ranked[i].Name, name, names(ranked))
		}
	}
}

// TestRankSignaturesForTarget_CentralityBreaksTies is the fix for the case the
// tiers cannot help with: a file that defines nothing and references nothing.
// The most useful eight symbols there are the ones the rest of the package
// leans on, not the ones whose filename sorts first.
func TestRankSignaturesForTarget_CentralityBreaksTies(t *testing.T) {
	sigs := []SymbolSignature{
		{Name: "AlphaRarelyUsed", File: "aaa.go", Exported: true},
		{Name: "ZebraLoadBearing", File: "zzz.go", Exported: true},
	}
	centrality := map[string]int{"ZebraLoadBearing": 12, "AlphaRarelyUsed": 1}

	ranked, _ := rankSignaturesForTarget(sigs, "marker.go", nil, centrality)
	if ranked[0].Name != "ZebraLoadBearing" {
		t.Fatalf("centrality did not win the tie: %v", names(ranked))
	}

	// Without centrality the order falls back to name, deterministically.
	plain, _ := rankSignaturesForTarget(sigs, "marker.go", nil, nil)
	if plain[0].Name != "AlphaRarelyUsed" {
		t.Fatalf("tie-break without centrality is not by file/name: %v", names(plain))
	}
}

// TestRankSignaturesForTarget_Deterministic pins byte-stability. A section that
// reshuffles between turns for no reason throws away the provider's prompt
// cache on every turn.
func TestRankSignaturesForTarget_Deterministic(t *testing.T) {
	sigs := []SymbolSignature{
		{Name: "B", File: "b.go", Exported: true},
		{Name: "A", File: "a.go", Exported: true},
		{Name: "C", File: "c.go", Exported: true},
	}
	first, _ := rankSignaturesForTarget(append([]SymbolSignature(nil), sigs...), "t.go", nil, nil)
	for i := 0; i < 5; i++ {
		again, _ := rankSignaturesForTarget(append([]SymbolSignature(nil), sigs...), "t.go", nil, nil)
		for j := range first {
			if first[j].Name != again[j].Name {
				t.Fatalf("ranking is not deterministic: %v vs %v", names(first), names(again))
			}
		}
	}
}

// TestRankSignaturesForTarget_Empty covers the degenerate inputs the prompt path
// hits for a file whose package has no exports at all.
func TestRankSignaturesForTarget_Empty(t *testing.T) {
	if ranked, pool := rankSignaturesForTarget(nil, "t.go", nil, nil); ranked != nil || pool != 0 {
		t.Fatalf("nil input produced %v/%d", ranked, pool)
	}
	unexported := []SymbolSignature{{Name: "priv", File: "a.go"}}
	if ranked, pool := rankSignaturesForTarget(unexported, "t.go", nil, nil); ranked != nil || pool != 0 {
		t.Fatalf("all-unexported input produced %v/%d", ranked, pool)
	}
}

// TestRankTypesForTarget_KeepsUnexported checks the deliberate difference from
// signatures: inside a package the model edits unexported types too.
func TestRankTypesForTarget_KeepsUnexported(t *testing.T) {
	types := []TypeDefinition{
		{Name: "otherPrivate", File: "other.go"},
		{Name: "localPrivate", File: "target.go"},
		{Name: "OtherPublic", File: "other.go", Exported: true},
	}
	ranked, pool := rankTypesForTarget(types, "target.go", nil, nil)
	if pool != 3 {
		t.Fatalf("pool = %d, want 3 (unexported types are kept)", pool)
	}
	if ranked[0].Name != "localPrivate" {
		t.Fatalf("file-local type did not rank first: %+v", ranked)
	}
	if ranked[1].Name != "OtherPublic" {
		t.Fatalf("exported did not outrank unexported within a tier: %+v", ranked)
	}
}

// TestPromptSection_RanksRealPackage is the end-to-end check on a real package:
// the target's own symbols must come first, and a sibling it does not touch
// must not displace them.
func TestPromptSection_RanksRealPackage(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		// Sorts first alphabetically, and is irrelevant to the target.
		"aaa_unrelated.go": "package p\n\n// Unrelated1 is noise.\nfunc Unrelated1() {}\n\n// Unrelated2 is noise.\nfunc Unrelated2() {}\n",
		"helper.go":        "package p\n\n// Helper is what the target calls.\nfunc Helper() int { return 1 }\n",
		"target.go":        "package p\n\n// TargetOne is defined here.\nfunc TargetOne() int { return Helper() }\n",
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	h := NewHolographicProvider(nil, dir)
	target := filepath.Join(dir, "target.go")

	hc, err := h.GetContext(target)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	// The target calls Helper, which lives in a sibling — that is the reference
	// edge the ranking is built on.
	if !contains(hc.ReferencedSymbols, "Helper") {
		t.Fatalf("ReferencedSymbols missing Helper: %v", hc.ReferencedSymbols)
	}
	if contains(hc.ReferencedSymbols, "TargetOne") {
		t.Errorf("a file's own definition must not be listed as a reference: %v", hc.ReferencedSymbols)
	}

	section := h.PromptSection(context.Background(), target)
	iTarget := strings.Index(section, "TargetOne")
	iHelper := strings.Index(section, "Helper")
	iNoise := strings.Index(section, "Unrelated1")
	if iTarget < 0 || iHelper < 0 {
		t.Fatalf("section is missing the relevant symbols:\n%s", section)
	}
	if iTarget > iHelper {
		t.Errorf("the file's own symbol must rank above the one it calls:\n%s", section)
	}
	if iNoise >= 0 && iNoise < iHelper {
		t.Errorf("an unrelated alphabetically-first symbol outranked a referenced one:\n%s", section)
	}
}

// TestPromptSection_TruncationNamesItsPool pins the wording. "and 593 more"
// beside eight symbols from the target's own file reads like the file has 601
// exports.
func TestPromptSection_TruncationNamesItsPool(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	sb.WriteString("package p\n")
	for i := 0; i < 40; i++ {
		sb.WriteString("\nfunc Fn")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString(string(rune('a' + i/26)))
		sb.WriteString("() {}\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "big.go"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewHolographicProvider(nil, dir)
	section := h.PromptSection(context.Background(), filepath.Join(dir, "big.go"))
	if !strings.Contains(section, "more exported in package `p`") {
		t.Fatalf("truncation line does not name its pool:\n%s", section)
	}
}

func names(sigs []SymbolSignature) []string {
	out := make([]string, len(sigs))
	for i, s := range sigs {
		out[i] = s.Name
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
