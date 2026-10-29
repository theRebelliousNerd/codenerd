package core

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"
)

const fingerprintPolicy = `
Decl fp_row(X) bound [/string].
Decl fp_seen(X) bound [/string].
fp_seen(X) :- fp_row(X).
`

func fingerprintKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := newRealKernelBase()
	k.AppendPolicy(fingerprintPolicy)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	return k
}

func mustFingerprint(t *testing.T, k *RealKernel) string {
	t.Helper()
	fp, err := k.ContentFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func queryAllDump(t *testing.T, k *RealKernel) string {
	t.Helper()
	all, err := k.QueryAll()
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, facts := range all {
		for _, f := range facts {
			lines = append(lines, f.String())
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// Equal content, by any route, has one fingerprint; any different EDB or
// program has another. Kernels whose fingerprints agree answer QueryAll
// alike, over a random walk of asserts and retracts on two kernels.
func TestContentFingerprintNamesTheKernelsContent(t *testing.T) {
	a, b := fingerprintKernel(t), fingerprintKernel(t)
	if mustFingerprint(t, a) != mustFingerprint(t, b) {
		t.Fatal("two empty kernels of one program differ")
	}
	rng := rand.New(rand.NewSource(20261029))
	agreed, differed := 0, 0
	for step := 0; step < 400; step++ {
		k := []*RealKernel{a, b}[rng.Intn(2)]
		f := Fact{Predicate: "fp_row", Args: []any{fmt.Sprintf("r%d", rng.Intn(6))}}
		if rng.Intn(2) == 0 {
			_ = k.Assert(f)
		} else {
			_ = k.RetractExactFact(f)
		}
		same := mustFingerprint(t, a) == mustFingerprint(t, b)
		if got := queryAllDump(t, a) == queryAllDump(t, b); got != same {
			t.Fatalf("step %d: fingerprints equal=%v but QueryAll equal=%v", step, same, got)
		}
		if same {
			agreed++
		} else {
			differed++
		}
	}
	if agreed == 0 || differed == 0 {
		t.Fatalf("the walk never exercised both outcomes (agreed %d, differed %d)", agreed, differed)
	}

	clone := a.Clone()
	if mustFingerprint(t, clone) != mustFingerprint(t, a) {
		t.Fatal("a clone's fingerprint differs from its parent's")
	}
	before := mustFingerprint(t, a)
	a.AppendPolicy("Decl fp_other(X) bound [/string].\n")
	if mustFingerprint(t, a) == before {
		t.Fatal("a program change left the fingerprint unchanged")
	}
}

// A kernel whose rules read an external predicate cannot be named by its EDB:
// its fingerprint is unique to the state and never matches another kernel's.
func TestContentFingerprintOfAVolatileProgramIsUnique(t *testing.T) {
	a, b := fingerprintKernel(t), fingerprintKernel(t)
	for _, k := range []*RealKernel{a, b} {
		k.mu.Lock()
		k.cone.volatile = []string{"fp_seen"}
		k.mu.Unlock()
	}
	defer func() {
		// The cone index is shared through the compiled-program cache.
		a.mu.Lock()
		a.cone.volatile = nil
		a.mu.Unlock()
	}()
	if mustFingerprint(t, a) == mustFingerprint(t, b) {
		t.Fatal("two kernels with external inputs share a content fingerprint")
	}
}
