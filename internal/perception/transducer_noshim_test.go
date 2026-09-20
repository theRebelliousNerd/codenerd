package perception

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestTransducerDoesNotReexportPiggybackTypes pins the NO SHIMS rule for the
// Piggyback Protocol types at the declaration site.
//
// The five protocol types live in codenerd/internal/articulation and every
// caller names them from there directly. transducer.go used to re-export
// them under its own package name for backward compatibility; those
// forwarding declarations were deleted so two spellings of one truth cannot
// coexist. Reintroducing any of them must turn this test red.
//
// An alias IS the canonical type, so no runtime assertion can tell the old
// spelling apart from the direct one: reflection reports the articulation
// type either way. The invariant is therefore a source invariant -- no
// forwarding declaration in transducer.go -- and this test asserts it at
// the source, which is the exact place the change sits.
func TestTransducerDoesNotReexportPiggybackTypes(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: cannot locate this test file")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "transducer.go"))
	if err != nil {
		t.Fatalf("read transducer.go: %v", err)
	}
	body := string(raw)

	for _, name := range []string{
		"PiggybackEnvelope",
		"ControlPacket",
		"IntentClassification",
		"MemoryOperation",
		"SelfCorrection",
	} {
		if strings.Contains(body, "type "+name+" =") {
			t.Fatalf("transducer.go re-exports %s (NO SHIMS, EVER): name it from articulation directly", name)
		}
	}

	// Guard against a vacuous pass on a gutted file: transducer.go must
	// still name the canonical package directly.
	if !strings.Contains(body, "articulation.") {
		t.Fatal("transducer.go no longer references the articulation package; expected direct use, found nothing")
	}
}
