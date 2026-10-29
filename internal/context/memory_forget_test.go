package context

import (
	stdctx "context"
	"encoding/json"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

type forgetRig struct {
	kernel *core.RealKernel
	db     *store.LocalStore
}

func newForgetRig(t *testing.T) forgetRig {
	t.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return forgetRig{kernel: kernel, db: db}
}

// remember lands a person's "remember" the way the chat does.
func (r forgetRig) remember(t *testing.T, sentence string) {
	t.Helper()
	d := ApplyMemoryOperation(r.kernel, r.db, articulation.MemoryOperation{Op: "promote_to_long_term", Key: "preference", Value: sentence})
	if !d.Took() {
		t.Fatalf("remember %q: %s", sentence, d.Notice())
	}
}

func (r forgetRig) forget(words string) articulation.ControlDisposition {
	return ApplyMemoryOperation(r.kernel, r.db, articulation.MemoryOperation{Op: "forget", Key: words, Domain: articulation.MemoryDomainUserMemory})
}

// hydrated is what a fresh turn's hydration shows the kernel.
func (r forgetRig) hydrated(t *testing.T) []string {
	t.Helper()
	vs := core.NewVirtualStoreWithConfig(nil, core.DefaultVirtualStoreConfig())
	vs.SetLocalDB(r.db)
	fresh, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	vs.SetKernel(fresh)
	if _, err := vs.HydrateLearnings(stdctx.Background()); err != nil {
		t.Fatal(err)
	}
	return learnedValues(t, fresh)
}

func learnedValues(t *testing.T, k types.Kernel) []string {
	t.Helper()
	facts, err := k.Query("learned_preference")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range facts {
		out = append(out, f.Args[1].(string))
	}
	return out
}

// A remembered sentence reaches the next turn's hydration, and forgetting it
// removes the one record and its live copy: not every preference, and not the
// session note a forget used to drop in its place.
func TestForgetUserMemory_RemovesTheOneRecordAndItsLiveCopy(t *testing.T) {
	r := newForgetRig(t)
	r.remember(t, "I prefer tabs over spaces")
	r.remember(t, "always run the race detector")
	if d := ApplyMemoryOperation(r.kernel, r.db, articulation.MemoryOperation{Op: "note", Key: "tabs", Value: "scratch"}); !d.Took() {
		t.Fatal(d.Notice())
	}
	// Live copies, as the boot hydration would have asserted them.
	vs := core.NewVirtualStoreWithConfig(nil, core.DefaultVirtualStoreConfig())
	vs.SetLocalDB(r.db)
	vs.SetKernel(r.kernel)
	if _, err := vs.HydrateLearnings(stdctx.Background()); err != nil {
		t.Fatal(err)
	}
	if got := learnedValues(t, r.kernel); len(got) != 2 {
		t.Fatalf("remembered preferences hydrated as %v, want both", got)
	}

	d := r.forget("prefer tabs")
	if !d.Took() || !strings.Contains(d.Receipt, "removed the saved long-term preference") {
		t.Fatalf("forget = %+v", d)
	}
	if got := learnedValues(t, r.kernel); len(got) != 1 || got[0] != "always run the race detector" {
		t.Errorf("live copies after forget = %v, want only the other preference", got)
	}
	if got := r.hydrated(t); len(got) != 1 || got[0] != "always run the race detector" {
		t.Errorf("a fresh hydration shows %v, want only the other preference", got)
	}
	if notes, _ := r.kernel.Query("session_note"); len(notes) != 1 {
		t.Errorf("a user-memory forget touched the session note: %v", notes)
	}

	// Already absent in the right domain is a stated no-op, not a success claim.
	d = r.forget("prefer tabs")
	if d.Stage != articulation.StageUnresolved || !strings.Contains(d.Reason, "nothing was removed") {
		t.Errorf("forgetting it again = %+v", d)
	}
}

func TestForgetUserMemory_NeverDeletesOnAGuess(t *testing.T) {
	r := newForgetRig(t)
	r.remember(t, "use tabs in Go files")
	r.remember(t, "use tabs in Makefiles")

	d := r.forget("use tabs")
	if d.Stage != articulation.StageUnresolved || !strings.Contains(d.Reason, "2 saved preferences match") {
		t.Fatalf("ambiguous forget = %+v", d)
	}
	if got := r.hydrated(t); len(got) != 2 {
		t.Fatalf("an ambiguous forget changed memory: %v", got)
	}
	// Exact words resolve even though both contain them.
	if d := r.forget("Use tabs in Makefiles."); !d.Took() {
		t.Fatalf("exact forget = %+v", d)
	}
	if got := r.hydrated(t); len(got) != 1 || got[0] != "use tabs in Go files" {
		t.Fatalf("after the exact forget: %v", got)
	}
	// Words too short to select a record select none.
	if d := r.forget("us"); d.Stage != articulation.StageUnresolved {
		t.Fatalf("short forget = %+v", d)
	}
}

// The generic key every remembered sentence shares never selects by itself.
func TestForgetUserMemory_TheGenericKeyIsNotAnIdentity(t *testing.T) {
	r := newForgetRig(t)
	r.remember(t, "use tabs")
	r.remember(t, "use go 1.24")
	if d := r.forget("preference"); d.Took() || len(r.hydrated(t)) != 2 {
		t.Fatalf("forgetting by the shared key removed something: %+v", d)
	}
	// A distinctive key a Piggyback promotion chose does identify its record.
	if d := ApplyMemoryOperation(r.kernel, r.db, articulation.MemoryOperation{Op: "promote_to_long_term", Key: "indent_style", Value: "tabs"}); !d.Took() {
		t.Fatal(d.Notice())
	}
	if d := r.forget("indent style"); !d.Took() {
		t.Fatalf("forget by distinctive key = %+v", d)
	}
}

// A model's control packet cannot address durable memory: its forget has no
// domain, drops a session note, and says durable memory was not touched.
func TestForget_ModelPacketIsSessionOnlyAndSaysSo(t *testing.T) {
	r := newForgetRig(t)
	r.remember(t, "use tabs")
	ApplyMemoryOperation(r.kernel, r.db, articulation.MemoryOperation{Op: "note", Key: "use tabs", Value: "x"})

	d := ApplyMemoryOperation(r.kernel, r.db, articulation.MemoryOperation{Op: "forget", Key: "use tabs"})
	if !d.Took() || !strings.Contains(d.Receipt, "dropped the session note") || !strings.Contains(d.Receipt, "long-term memory was not touched") {
		t.Fatalf("session forget = %+v", d)
	}
	if got := r.hydrated(t); len(got) != 1 {
		t.Fatalf("a packet forget reached durable memory: %v", got)
	}
	d = ApplyMemoryOperation(r.kernel, r.db, articulation.MemoryOperation{Op: "forget", Key: "use tabs"})
	if !strings.Contains(d.Receipt, "there was no session note") {
		t.Fatalf("second session forget = %+v", d)
	}
	// The wire form never carries the domain: JSON cannot set it.
	var op articulation.MemoryOperation
	if err := jsonUnmarshal(`{"op":"forget","key":"x","domain":"user_memory"}`, &op); err != nil || op.Domain != articulation.MemoryDomainSession {
		t.Fatalf("a model envelope set the domain: %+v %v", op, err)
	}
}

func TestForgetUserMemory_UnsupportedWithoutAStoreThatCanRemove(t *testing.T) {
	k, _ := core.NewRealKernelWithWorkspace(t.TempDir())
	d := ApplyMemoryOperation(k, nil, articulation.MemoryOperation{Op: "forget", Key: "tabs please", Domain: articulation.MemoryDomainUserMemory})
	if d.Stage != articulation.StageUnsupported {
		t.Fatalf("forget with no store = %+v", d)
	}
}

type noExactKernel struct{ MemoryKernel }

func TestForgetUserMemory_ALiveCopyThatCannotBeRetiredIsPartial(t *testing.T) {
	r := newForgetRig(t)
	r.remember(t, "use tabs")
	d := ApplyMemoryOperation(noExactKernel{r.kernel}, r.db, articulation.MemoryOperation{Op: "forget", Key: "use tabs", Domain: articulation.MemoryDomainUserMemory})
	if d.Stage != articulation.StagePartiallyApplied || !strings.Contains(d.Reason, "live copy") {
		t.Fatalf("forget = %+v", d)
	}
	if got := r.hydrated(t); len(got) != 0 {
		t.Fatalf("the saved record survived: %v", got)
	}
}

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
