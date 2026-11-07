package core

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

const specPairingBenchPolicy = `
Decl spec_section(Ref, Path, Heading, Fingerprint) bound [/string, /string, /string, /string].
Decl spec_element(Ref, Dir, File, Kind, Exported, Fingerprint) bound [/string, /string, /string, /name, /name, /string].
Decl spec_link(SpecRef, CodeRef, Relation, Source) bound [/string, /string, /name, /name].
Decl spec_confirmed(SpecRef, CodeRef, SpecFingerprint, CodeFingerprint, ConfirmedAt) bound [/string, /string, /string, /string, /number].
Decl spec_drift(SpecRef, CodeRef) bound [/string, /string].
Decl spec_stale_code(SpecRef, CodeRef) bound [/string, /string].
Decl bench_filler(A) bound [/string].

spec_drift(SpecRef, CodeRef) :-
	spec_link(SpecRef, CodeRef, _, _),
	spec_confirmed(SpecRef, CodeRef, SpecPrint, CodePrint, _),
	spec_section(SpecRef, _, _, SpecPrint),
	spec_element(CodeRef, _, _, _, _, NowPrint),
	NowPrint != CodePrint.

spec_stale_code(SpecRef, CodeRef) :-
	spec_link(SpecRef, CodeRef, _, _),
	spec_confirmed(SpecRef, CodeRef, SpecPrint, CodePrint, _),
	spec_element(CodeRef, _, _, _, _, CodePrint),
	spec_section(SpecRef, _, _, NowPrint),
	NowPrint != SpecPrint.
`

func specGenFacts(gen, nLink, nElem, nSec int) (links, elements, sections, confirmed []Fact) {
	// Fingerprint generations: gen 0 uses s0/c0 base, gen 1 uses s2/c2 so every
	// row's canonical key changes (full replacement) while the drift pattern stays.
	sBase := fmt.Sprintf("s%d", gen*2)
	cBase := fmt.Sprintf("c%d", gen*2)
	sAlt := fmt.Sprintf("s%d", gen*2+1)
	cAlt := fmt.Sprintf("c%d", gen*2+1)
	for i := 0; i < nSec; i++ {
		ref := fmt.Sprintf("Docs/spec%05d.md:Sec > H%d", i, i)
		fp := sBase + fmt.Sprintf("-%d", i)
		if i%2 == 1 {
			fp = sAlt + fmt.Sprintf("-%d", i)
		}
		if gen == 1 {
			// Move both generations forward so gen1 rows differ canonically.
			fp = fmt.Sprintf("s%d-%d", gen*2+1, i)
			if i%2 == 1 {
				fp = fmt.Sprintf("s%d-%d", gen*2+2, i)
			}
		}
		sections = append(sections, Fact{Predicate: "spec_section", Args: []any{ref, fmt.Sprintf("Docs/spec%05d.md", i), fmt.Sprintf("H%d", i), fp}})
	}
	for i := 0; i < nElem; i++ {
		ref := fmt.Sprintf("internal/pkg.Elem%05d", i)
		fp := cAlt + fmt.Sprintf("-%d", i)
		if i%2 == 1 {
			fp = cBase + fmt.Sprintf("-%d", i)
		}
		if gen == 1 {
			fp = fmt.Sprintf("c%d-%d", gen*2+2, i)
			if i%2 == 1 {
				fp = fmt.Sprintf("c%d-%d", gen*2+1, i)
			}
		}
		elements = append(elements, Fact{Predicate: "spec_element", Args: []any{ref, "internal/pkg", fmt.Sprintf("elem%05d.go", i), MangleAtom("/function"), MangleAtom("/true"), fp}})
	}
	src := MangleAtom("/doc_citation")
	if gen == 1 {
		src = MangleAtom("/code_comment")
	}
	for i := 0; i < nLink; i++ {
		specRef := fmt.Sprintf("Docs/spec%05d.md:Sec > H%d", i%nSec, i%nSec)
		codeRef := fmt.Sprintf("internal/pkg.Elem%05d", i%nElem)
		// Confirmed prints are the gen0 base pair for both generations so the
		// drift/stale pattern is stable; ConfirmedAt moves to force replacement.
		links = append(links, Fact{Predicate: "spec_link", Args: []any{specRef, codeRef, MangleAtom("/cites"), src}})
		specPrint := sBase + fmt.Sprintf("-%d", (i%nSec))
		codePrint := cBase + fmt.Sprintf("-%d", (i%nElem))
		confirmed = append(confirmed, Fact{Predicate: "spec_confirmed", Args: []any{specRef, codeRef, specPrint, codePrint, int64(1000 + gen)}})
	}
	return links, elements, sections, confirmed
}

func assertFactsTB(t testing.TB, k *RealKernel, facts []Fact) {
	t.Helper()
	for _, f := range facts {
		if err := k.AssertWithoutEval(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
}

func factSetKey(f Fact) string {
	s := f.Predicate
	for _, a := range f.Args {
		s += "\x00" + fmt.Sprintf("%v", a)
	}
	return s
}

func TestSpecPairingDRedMatchesFullFixpoint(t *testing.T) {
	const nLink, nElem, nSec = 400, 200, 100
	build := func(gen int) *RealKernel {
		k := newRealKernelBase()
		k.AppendPolicy(specPairingBenchPolicy)
		links, elements, sections, confirmed := specGenFacts(gen, nLink, nElem, nSec)
		assertFactsTB(t, k, sections)
		assertFactsTB(t, k, elements)
		assertFactsTB(t, k, links)
		assertFactsTB(t, k, confirmed)
		if err := k.Evaluate(); err != nil {
			t.Fatalf("gen %d evaluate: %v", gen, err)
		}
		return k
	}
	// Incremental path: boot gen0, then replace every spec fact with gen1.
	k := build(0)
	oldLinks, oldElements, oldSections, oldConfirmed := specGenFacts(0, nLink, nElem, nSec)
	_, _, _, _ = oldLinks, oldElements, oldSections, oldConfirmed
	newLinks, newElements, newSections, newConfirmed := specGenFacts(1, nLink, nElem, nSec)
	for _, f := range oldSections {
		if err := k.RetractExactFact(f); err != nil {
			t.Fatalf("retract section: %v", err)
		}
	}
	for _, f := range oldElements {
		if err := k.RetractExactFact(f); err != nil {
			t.Fatalf("retract element: %v", err)
		}
	}
	for _, f := range oldLinks {
		if err := k.RetractExactFact(f); err != nil {
			t.Fatalf("retract link: %v", err)
		}
	}
	for _, f := range oldConfirmed {
		if err := k.RetractExactFact(f); err != nil {
			t.Fatalf("retract confirmed: %v", err)
		}
	}
	assertFactsTB(t, k, newSections)
	assertFactsTB(t, k, newElements)
	assertFactsTB(t, k, newLinks)
	assertFactsTB(t, k, newConfirmed)
	if err := k.EvaluatePending(); err != nil {
		t.Fatalf("incremental re-eval: %v", err)
	}
	// Full fixpoint path: fresh kernel holding only gen1.
	want := build(1)
	for _, pred := range []string{"spec_drift", "spec_stale_code"} {
		got, err := k.Query(pred)
		if err != nil {
			t.Fatalf("query %s incremental: %v", pred, err)
		}
		exp, err := want.Query(pred)
		if err != nil {
			t.Fatalf("query %s full: %v", pred, err)
		}
		gotKeys := make([]string, 0, len(got))
		for _, f := range got {
			gotKeys = append(gotKeys, factSetKey(f))
		}
		expKeys := make([]string, 0, len(exp))
		for _, f := range exp {
			expKeys = append(expKeys, factSetKey(f))
		}
		sort.Strings(gotKeys)
		sort.Strings(expKeys)
		if len(gotKeys) != len(expKeys) {
			t.Fatalf("%s count mismatch: dred=%d full=%d", pred, len(gotKeys), len(expKeys))
		}
		for i := range gotKeys {
			if gotKeys[i] != expKeys[i] {
				t.Fatalf("%s row %d mismatch: dred=%q full=%q", pred, i, gotKeys[i], expKeys[i])
			}
		}
	}
	t.Logf("incremental mode=%s profile=%s", k.LastEvaluation().Mode, k.LastEvaluation().Profile)
}

func BenchmarkSpecPairingDRedFullReplacement(b *testing.B) {
	const nLink, nElem, nSec = 26000, 15000, 6800
	k := newRealKernelBase()
	k.AppendPolicy(specPairingBenchPolicy)
	links0, elements0, sections0, confirmed0 := specGenFacts(0, nLink, nElem, nSec)
	assertFactsTB(b, k, sections0)
	assertFactsTB(b, k, elements0)
	assertFactsTB(b, k, links0)
	assertFactsTB(b, k, confirmed0)
	// Background EDB so the global change ratio stays below the full-fixpoint
	// cutover, forcing the DRed path like production (73k spec rows over 694k).
	for i := 0; i < 620000; i++ {
		_ = k.AssertWithoutEval(Fact{Predicate: "bench_filler", Args: []any{fmt.Sprintf("fill-%07d", i)}})
	}
	if err := k.Evaluate(); err != nil {
		b.Fatal(err)
	}
	b.Logf("boot mode=%s profile=%s", k.LastEvaluation().Mode, k.LastEvaluation().Profile)
	links1, elements1, sections1, confirmed1 := specGenFacts(1, nLink, nElem, nSec)
	b.ResetTimer()
	var total time.Duration
	for i := 0; i < b.N; i++ {
		// Alternate generations so every pass replaces every spec fact.
		var oldS, oldE, oldL, oldC, newS, newE, newL, newC []Fact
		if i%2 == 0 {
			oldS, oldE, oldL, oldC = sections0, elements0, links0, confirmed0
			newS, newE, newL, newC = sections1, elements1, links1, confirmed1
		} else {
			oldS, oldE, oldL, oldC = sections1, elements1, links1, confirmed1
			newS, newE, newL, newC = sections0, elements0, links0, confirmed0
		}
		for _, f := range oldS {
			_ = k.RetractExactFact(f)
		}
		for _, f := range oldE {
			_ = k.RetractExactFact(f)
		}
		for _, f := range oldL {
			_ = k.RetractExactFact(f)
		}
		for _, f := range oldC {
			_ = k.RetractExactFact(f)
		}
		for _, f := range newS {
			_ = k.AssertWithoutEval(f)
		}
		for _, f := range newE {
			_ = k.AssertWithoutEval(f)
		}
		for _, f := range newL {
			_ = k.AssertWithoutEval(f)
		}
		for _, f := range newC {
			_ = k.AssertWithoutEval(f)
		}
		started := time.Now()
		if err := k.EvaluatePending(); err != nil {
			b.Fatal(err)
		}
		total += time.Since(started)
		ev := k.LastEvaluation()
		b.Logf("iter %d mode=%s changed=%d inserted=%d overdeleted=%d rederived=%d variants=%d profile=%s", i, ev.Mode, ev.ChangedRows, ev.DeltaFacts, ev.OverDeleted, ev.Rederived, ev.DeltaVariants, ev.Profile)
		if ev.Mode != "dred" {
			b.Logf("note: re-eval ran as %q fallback %q", ev.Mode, ev.ConeFallback)
		}
	}
	b.ReportMetric(float64(total.Microseconds())/float64(b.N), "eval-us/op")
}
