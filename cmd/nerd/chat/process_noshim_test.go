package chat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestProcessInputRefersToArticulationDirectly pins the call-site half of the
// NO SHIMS change inside Model.processInput.
//
// processInput builds the turn's control packet and memory-operation list
// from the articulation output. It used to spell those types through the
// old perception re-exports; every use now names the articulation
// package's types directly. Reverting processInput to the perception
// spelling must turn this test red.
//
// The two spellings are type aliases, hence identical types with identical
// behaviour -- the compiler, and a runtime assertion, cannot tell them
// apart. What the change guarantees is single-truth spelling at the call
// site, so this test asserts it at the source, scoped to the changed
// function.
func TestProcessInputRefersToArticulationDirectly(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: cannot locate this test file")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "process.go"))
	if err != nil {
		t.Fatalf("read process.go: %v", err)
	}

	idx := strings.Index(string(raw), ") processInput(")
	if idx < 0 {
		t.Fatal("process.go: Model.processInput not found; cannot scope to the changed function")
	}
	fn := string(raw)[idx:]

	for _, name := range []string{
		"PiggybackEnvelope",
		"ControlPacket",
		"IntentClassification",
		"MemoryOperation",
		"SelfCorrection",
	} {
		if strings.Contains(fn, "perception."+name) {
			t.Fatalf("processInput reaches %s through perception; use articulation.%s directly", name, name)
		}
	}

	// Guard against a vacuous pass: the canonical spellings the change
	// introduced must actually be present in the function body.
	if !strings.Contains(fn, "articulation.ControlPacket") {
		t.Fatal("processInput does not build articulation.ControlPacket; expected direct canonical use")
	}
	if !strings.Contains(fn, "articulation.MemoryOperation") {
		t.Fatal("processInput does not use articulation.MemoryOperation; expected direct canonical use")
	}
}
