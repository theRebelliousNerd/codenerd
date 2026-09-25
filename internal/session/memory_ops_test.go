package session

import (
	"testing"

	"codenerd/internal/types"
)

// memoryPersister is the session's persister when it is the knowledge store,
// as production's LocalStore is: it also takes a promoted preference and a
// stored vector.
type memoryPersister struct {
	facts   []string
	vectors []string
}

func (*memoryPersister) StoreSessionTurn(string, int, string, string, string, string) error {
	return nil
}
func (*memoryPersister) StoreCompressedState(string, int, string, float64) error { return nil }
func (p *memoryPersister) StoreFact(predicate string, args []any, _ string, _ int) error {
	p.facts = append(p.facts, predicate)
	return nil
}
func (p *memoryPersister) StoreVector(content string, _ map[string]any) error {
	p.vectors = append(p.vectors, content)
	return nil
}

// The envelope's memory operations land where the chat compressor lands them.
// On this path -- nerd run, delegated tasks, campaigns -- they used to be
// asserted as memory_operation(Op, Key, Value), which no .mg file declares, so
// the note the model was told would stay in session context was stored where
// no rule or query could read it, and a promotion never reached the store.
func TestPiggybackMemoryOperationsLandInKernelAndStore(t *testing.T) {
	kernel := realKernel(t)
	persister := &memoryPersister{}
	executor := &Executor{kernel: kernel, config: DefaultExecutorConfig()}
	executor.SetSessionPersister(persister)

	envelope := `{"control_packet":{"memory_operations":[` +
		`{"op":"note","key":"current_focus","value":"the router's retry path"},` +
		`{"op":"promote_to_long_term","key":"user_preference/indent","value":"tabs"},` +
		`{"op":"store_vector","key":"snippet/retry","value":"backoff doubles each attempt"}` +
		`]},"surface_response":"noted"}`
	if got := executor.processPiggybackControlPacket(envelope); got != "noted" {
		t.Fatalf("surface = %q", got)
	}

	notes, err := kernel.Query("session_note")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || types.ExtractString(notes[0].Args[1]) != "the router's retry path" {
		t.Fatalf("the note is not in the kernel as session_note: %v", notes)
	}
	if len(persister.facts) != 1 || persister.facts[0] != "user_preference/indent" {
		t.Fatalf("the promotion did not reach the store: %v", persister.facts)
	}
	if len(persister.vectors) != 1 {
		t.Fatalf("the vector did not reach the store: %v", persister.vectors)
	}
	if rows, _ := kernel.Query("memory_operation"); len(rows) != 0 {
		t.Fatalf("memory operations are still asserted under the undeclared memory_operation: %v", rows)
	}
}
