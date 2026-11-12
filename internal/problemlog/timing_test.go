package problemlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/types"
)

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

// The watcher aggregates the audit log's perf events as the log grows: only
// complete lines, only timing events, one aggregate per system and operation.
func TestTimingWatcher_AggregatesIncrementally(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run_audit.log")
	appendFile(t, path, "# Audit log started\n"+
		`{"ts":1,"event":"perf_metric","cat":"kernel","action":"Evaluate","dur_ms":40}`+"\n"+
		`{"ts":2,"event":"kernel_query","cat":"kernel","action":"Query","dur_ms":999}`+"\n"+
		`{"ts":3,"event":"perf_slow","cat":"kernel","action":"Evaluate","dur_ms":160}`+"\n"+
		`{"ts":4,"event":"perf_metric","cat":"store","action":"Get","dur_ms":5}`+"\n"+
		`{"ts":5,"event":"perf_metric","cat":"store","act`) // torn: a live writer's
	w := NewTimingWatcher()
	if err := w.Read(path); err != nil {
		t.Fatal(err)
	}
	if !w.Changed() || w.Changed() {
		t.Fatal("Changed must report a read that moved the aggregate, once")
	}
	got := w.Timings()
	if len(got) != 2 {
		t.Fatalf("timings = %+v", got)
	}
	ev := got[0]
	if ev.Target() != "timing:kernel.Evaluate" || ev.Count != 2 || ev.TotalMs != 200 || ev.MaxMs != 160 || ev.MeanMs() != 100 {
		t.Fatalf("costliest = %+v", ev)
	}
	// The torn line completes; nothing is counted twice.
	appendFile(t, path, `ion":"Get","dur_ms":7}`+"\n")
	if err := w.Read(path); err != nil {
		t.Fatal(err)
	}
	if g, _ := w.Lookup("timing:store.Get"); g.Count != 2 || g.TotalMs != 12 {
		t.Fatalf("store.Get = %+v", g)
	}
	w.Changed()
	if err := w.Read(path); err != nil {
		t.Fatal(err)
	}
	if w.Changed() {
		t.Fatal("a read of nothing new changed the aggregate")
	}

	// A new run's log starts over: timings belong to the process that took them.
	next := filepath.Join(t.TempDir(), "next_audit.log")
	appendFile(t, next, `{"event":"perf_metric","cat":"world","action":"Scan","dur_ms":3}`+"\n")
	if err := w.Read(next); err != nil {
		t.Fatal(err)
	}
	if got := w.Timings(); len(got) != 1 || got[0].System != "world" {
		t.Fatalf("after a new log: %+v", got)
	}
	// A log that does not exist yet is no error.
	if err := w.Read(filepath.Join(t.TempDir(), "missing.log")); err != nil {
		t.Fatal(err)
	}
}

func TestTimingFacts_MatchTheDecl(t *testing.T) {
	facts := TimingFacts([]Timing{{System: "kernel", Operation: "Evaluate", Count: 3, TotalMs: 30, MaxMs: 20}})
	want := []any{types.MangleString("timing:kernel.Evaluate"), types.MangleString("kernel"), types.MangleString("Evaluate"), int64(3), int64(30), int64(10)}
	if len(facts) != 1 || facts[0].Predicate != TimingPredicate || len(facts[0].Args) != len(want) {
		t.Fatalf("facts = %v", facts)
	}
	for i, a := range want {
		if facts[0].Args[i] != a {
			t.Fatalf("arg %d = %#v, want %#v", i, facts[0].Args[i], a)
		}
	}
	k := &fakeKernel{}
	if err := ReplaceTimings(k, nil); err != nil || len(k.retracted) != 2 || len(k.asserted) != 0 {
		t.Fatalf("replace with none: %v %v %v", err, k.retracted, k.asserted)
	}
}

// Timings after the boot settled are kept apart from the boot's: the optimize
// lane ranks the steady state (self_timing_settled). A boundary set after
// lines were read re-reads them on the right side of it.
func TestTimingWatcher_SettledTimingsExcludeTheBoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run_audit.log")
	appendFile(t, path, `{"ts":1000,"event":"perf_metric","cat":"kernel","action":"Evaluate","dur_ms":900}`+"\n"+
		`{"ts":2000,"event":"perf_metric","cat":"kernel","action":"Evaluate","dur_ms":800}`+"\n"+
		`{"ts":9000,"event":"perf_metric","cat":"kernel","action":"Evaluate","dur_ms":10}`+"\n")
	w := NewTimingWatcher()
	if err := w.Read(path); err != nil {
		t.Fatal(err)
	}
	if ev, _ := w.Lookup("timing:kernel.Evaluate"); ev.Count != 3 || ev.SettledCount != 0 {
		t.Fatalf("before the boot settled: %+v, want no settled timings", ev)
	}
	if facts := TimingFacts(w.Timings()); len(facts) != 1 {
		t.Fatalf("facts before the boot settled = %v, want only self_timing", facts)
	}
	w.SetSettled(time.UnixMilli(5000))
	if err := w.Read(path); err != nil {
		t.Fatal(err)
	}
	ev, _ := w.Lookup("timing:kernel.Evaluate")
	if ev.Count != 3 || ev.TotalMs != 1710 || ev.SettledCount != 1 || ev.SettledTotalMs != 10 || ev.SettledMeanMs() != 10 {
		t.Fatalf("after the boot settled: %+v", ev)
	}
	facts := TimingFacts(w.Timings())
	want := []any{types.MangleString("timing:kernel.Evaluate"), int64(1), int64(10), int64(10)}
	if len(facts) != 2 || facts[1].Predicate != SettledTimingPredicate || len(facts[1].Args) != len(want) {
		t.Fatalf("facts = %v", facts)
	}
	for i, a := range want {
		if facts[1].Args[i] != a {
			t.Fatalf("settled arg %d = %#v, want %#v", i, facts[1].Args[i], a)
		}
	}
}
