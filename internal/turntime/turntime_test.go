package turntime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock is a clock the test advances by hand.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) tick(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// TestFakeTurnLedger runs a turn with known durations per phase and requires
// the ledger to say exactly those, with verification inside a repair round
// charged to verification and the round charged its model and tool time only.
func TestFakeTurnLedger(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	l := NewWithClock(clk.now)

	clk.tick(2 * time.Second) // other: compile, memory
	end := l.Model()
	clk.tick(30 * time.Second)
	end()
	end = l.Tool()
	clk.tick(5 * time.Second)
	end()
	end = l.Verify(KindBuild)
	clk.tick(11 * time.Second)
	end()
	end = l.Verify(KindTests)
	clk.tick(40 * time.Second)
	end()

	// A repair round: model 20s, tool 4s, then its own recheck run inside the
	// round's window (9s), then 3s more model.
	round := l.Repair(KindTests)
	end = l.Model()
	clk.tick(20 * time.Second)
	end()
	end = l.Tool()
	clk.tick(4 * time.Second)
	end()
	end = l.Verify(KindTests)
	clk.tick(9 * time.Second)
	end()
	end = l.Model()
	clk.tick(3 * time.Second)
	end()
	round()

	end = l.Verify(KindImporter)
	clk.tick(7 * time.Second)
	end()
	end = l.Verify(KindCoverage)
	clk.tick(6 * time.Second)
	end()
	end = l.Verify(KindPin)
	clk.tick(8 * time.Second)
	end()
	end = l.Verify(KindFinal)
	clk.tick(12 * time.Second)
	end()
	clk.tick(1 * time.Second) // other
	l.Close()
	clk.tick(time.Hour) // after close: nothing

	s := l.Snapshot()
	check := func(name string, got, wantSecs time.Duration) {
		t.Helper()
		if got != wantSecs*time.Second {
			t.Errorf("%s = %s, want %ds", name, got, wantSecs)
		}
	}
	check("model", s.Model, 30+20+3)
	check("tool", s.Tool, 5+4)
	check("other", s.Other, 3)
	check("verify total", s.VerifyTime(), 11+40+9+7+6+8+12)
	check("total", s.Total, 2+30+5+11+40+36+7+6+8+12+1)
	if sum := s.Model + s.Tool + s.Other + s.VerifyTime(); sum != s.Total {
		t.Errorf("phases sum to %s, total %s: a phase was double counted or dropped", sum, s.Total)
	}
	for kind, secs := range map[string]time.Duration{
		KindBuild: 11, KindTests: 49, KindImporter: 7, KindCoverage: 6, KindPin: 8, KindFinal: 12,
	} {
		var got Kind
		for _, k := range s.Verify {
			if k.Kind == kind {
				got = k
			}
		}
		check("verify "+kind, got.Time, secs)
	}
	if s.VerifyRuns() != 7 {
		t.Errorf("verify runs = %d, want 7", s.VerifyRuns())
	}
	if len(s.Repair) != 1 || s.Repair[0].Kind != KindTests || s.Repair[0].Count != 1 {
		t.Fatalf("repair = %+v, want one tests round", s.Repair)
	}
	check("repair time (model+tool inside the round, verification excluded)", s.Repair[0].Time, 20+4+3)
	if s.ModelCalls != 3 || s.ToolCalls != 2 {
		t.Errorf("calls = model %d tool %d, want 3 and 2", s.ModelCalls, s.ToolCalls)
	}
}

// TestOverlappingCallsAreChargedOnce: concurrent tool calls overlap in wall
// time, and the ledger measures the turn's clock.
func TestOverlappingCallsAreChargedOnce(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := NewWithClock(clk.now)
	a := l.Tool()
	clk.tick(10 * time.Second)
	b := l.Tool()
	clk.tick(10 * time.Second)
	a()
	clk.tick(10 * time.Second)
	b()
	s := l.Snapshot()
	if s.Tool != 30*time.Second || s.Total != 30*time.Second || s.ToolCalls != 2 {
		t.Fatalf("tool = %s total = %s calls = %d, want 30s, 30s, 2", s.Tool, s.Total, s.ToolCalls)
	}
}

// TestConcurrentSpansAreRaceFree hammers every span kind from many goroutines
// on the real clock (run with -race) and requires the phases to sum to the
// total.
func TestConcurrentSpansAreRaceFree(t *testing.T) {
	l := New()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				var end func()
				switch (i + j) % 4 {
				case 0:
					end = l.Tool()
				case 1:
					end = l.Model()
				case 2:
					end = l.Verify(KindTests)
				default:
					end = l.Repair(KindBuild)
				}
				_ = l.Snapshot()
				end()
				end() // a second end is a no-op
			}
		}(i)
	}
	wg.Wait()
	s := l.Snapshot()
	if sum := s.Model + s.Tool + s.Other + s.VerifyTime(); sum != s.Total {
		t.Fatalf("phases sum to %s, total %s", sum, s.Total)
	}
	if s.ToolCalls != 32*50/4 || s.ModelCalls != 32*50/4 || s.VerifyRuns() != 32*50/4 || s.RepairRounds() != 32*50/4 {
		t.Fatalf("lost calls: %+v", s)
	}
}

func TestNilLedgerRecordsNothing(t *testing.T) {
	var l *Ledger
	l.Model()()
	l.Verify("x")()
	l.Close()
	if l.Snapshot().Recorded() || From(context.Background()) != nil {
		t.Fatal("a nil ledger must record nothing")
	}
}

func TestContextCarriesLedgerAndKind(t *testing.T) {
	l := New()
	ctx := WithVerifyKind(WithLedger(context.Background(), l), KindImporter)
	if From(ctx) != l || VerifyKindFrom(ctx) != KindImporter {
		t.Fatal("ledger and kind must ride the context")
	}
}

func TestLineAndLogFields(t *testing.T) {
	s := Split{
		Total: 12*time.Minute + 3*time.Second, Model: 8 * time.Minute, Tool: time.Minute, Other: 11 * time.Second,
		ModelCalls: 41, ToolCalls: 96,
		Verify: []Kind{{KindBuild, 2, 20 * time.Second}, {KindTests, 3, 2*time.Minute + 32*time.Second}},
		Repair: []Kind{{KindTests, 2, 90 * time.Second}},
	}
	line := s.Line()
	for _, want := range []string{"Time 12m3s", "model 8m0s (41 calls)", "tools 1m0s (96)", "verify 2m52s (build 20s, tests 2m32s)", "repair 2 rounds (tests 2; 1m30s)", "other 11s"} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "\n") {
		t.Errorf("the summary is one line: %q", line)
	}
	fields := s.LogFields()
	for _, want := range []string{"secs_total=723.0s", "secs_model=480.0s", "secs_tool=60.0s", "secs_verify=172.0s", "secs_other=11.0s", "repair_rounds=2", "verify_build=20.0s/2", "repair_tests=90.0s/2"} {
		if !strings.Contains(fields, want) {
			t.Errorf("fields %q lack %q", fields, want)
		}
	}
	if (Split{}).Line() != "" {
		t.Error("an unmeasured split renders nothing")
	}
}

func TestEmptyKindDefaultsToOther(t *testing.T) {
	clk := &fakeClock{t: time.Unix(2000, 0)}
	l := NewWithClock(clk.now)
	endV := l.Verify("")
	clk.tick(5 * time.Second)
	endV()
	round := l.Repair("")
	endM := l.Model()
	clk.tick(3 * time.Second)
	endM()
	round()
	s := l.Snapshot()
	var gotV, gotR *Kind
	for i := range s.Verify {
		if s.Verify[i].Kind == KindOther {
			gotV = &s.Verify[i]
		}
	}
	for i := range s.Repair {
		if s.Repair[i].Kind == KindOther {
			gotR = &s.Repair[i]
		}
	}
	if gotV == nil || gotV.Time != 5*time.Second || gotV.Count != 1 {
		t.Fatalf("verify other = %+v, want 5s/1", gotV)
	}
	if gotR == nil || gotR.Time != 3*time.Second || gotR.Count != 1 {
		t.Fatalf("repair other = %+v, want 3s/1", gotR)
	}
}

func TestVerifyOutranksModelAndTool(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3000, 0)}
	l := NewWithClock(clk.now)
	endM := l.Model()
	endT := l.Tool()
	clk.tick(4 * time.Second)
	endV := l.Verify(KindBuild)
	clk.tick(6 * time.Second)
	endV()
	clk.tick(2 * time.Second)
	endM()
	endT()
	s := l.Snapshot()
	if s.Model != 6*time.Second {
		t.Errorf("model = %s, want 6s (4s with tool open + 2s alone)", s.Model)
	}
	if s.Tool != 0 {
		t.Errorf("tool = %s, want 0 (model outranks tool throughout)", s.Tool)
	}
	if s.VerifyTime() != 6*time.Second {
		t.Errorf("verify = %s, want 6s", s.VerifyTime())
	}
	if s.Total != 12*time.Second {
		t.Errorf("total = %s, want 12s", s.Total)
	}
	if sum := s.Model + s.Tool + s.Other + s.VerifyTime(); sum != s.Total {
		t.Errorf("phases sum to %s, total %s", sum, s.Total)
	}
}

func TestModelOutranksToolAlone(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3100, 0)}
	l := NewWithClock(clk.now)
	endM := l.Model()
	endT := l.Tool()
	clk.tick(10 * time.Second)
	endM()
	clk.tick(5 * time.Second)
	endT()
	s := l.Snapshot()
	if s.Model != 10*time.Second || s.Tool != 5*time.Second {
		t.Fatalf("model = %s tool = %s, want 10s and 5s", s.Model, s.Tool)
	}
}

func TestOverlappingVerifyKindsChargeAlphabeticallyFirst(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3200, 0)}
	l := NewWithClock(clk.now)
	endVet := l.Verify(KindVet)
	endBuild := l.Verify(KindBuild)
	clk.tick(10 * time.Second)
	endVet()
	clk.tick(5 * time.Second)
	endBuild()
	s := l.Snapshot()
	byKind := map[string]Kind{}
	for _, k := range s.Verify {
		byKind[k.Kind] = k
	}
	if byKind[KindBuild].Time != 15*time.Second || byKind[KindBuild].Count != 1 {
		t.Errorf("build = %+v, want 15s/1", byKind[KindBuild])
	}
	if byKind[KindVet].Time != 0 || byKind[KindVet].Count != 1 {
		t.Errorf("vet = %+v, want 0s/1 (overlap charged to build)", byKind[KindVet])
	}
}

func TestOverlappingRepairKindsChargeAlphabeticallyFirst(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3201, 0)}
	l := NewWithClock(clk.now)
	endVet := l.Repair(KindVet)
	endBuild := l.Repair(KindBuild)
	endM := l.Model()
	clk.tick(10 * time.Second)
	endVet()
	clk.tick(5 * time.Second)
	endBuild()
	endM()
	s := l.Snapshot()
	byKind := map[string]Kind{}
	for _, k := range s.Repair {
		byKind[k.Kind] = k
	}
	if byKind[KindBuild].Time != 15*time.Second || byKind[KindBuild].Count != 1 {
		t.Errorf("build repair = %+v, want 15s/1", byKind[KindBuild])
	}
	if byKind[KindVet].Time != 0 || byKind[KindVet].Count != 1 {
		t.Errorf("vet repair = %+v, want 0s/1 (overlap charged to build)", byKind[KindVet])
	}
	if s.Model != 15*time.Second {
		t.Errorf("model = %s, want 15s", s.Model)
	}
	if s.RepairRounds() != 2 || s.RepairTime() != 15*time.Second {
		t.Errorf("repair rounds = %d time = %s, want 2 and 15s", s.RepairRounds(), s.RepairTime())
	}
}

func TestNewWithClockNilUsesWallClock(t *testing.T) {
	l := NewWithClock(nil)
	if l == nil {
		t.Fatal("NewWithClock(nil) must fall back to the wall clock, not return nil")
	}
	end := l.Model()
	end()
	if s := l.Snapshot(); s.ModelCalls != 1 {
		t.Fatalf("model calls = %d, want 1", s.ModelCalls)
	}
	endV := l.Verify(KindBuild)
	endV()
	if got := l.Snapshot().VerifyRuns(); got != 1 {
		t.Fatalf("verify runs = %d, want 1", got)
	}
}

func TestBackwardClockChargesNothing(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3301, 0)}
	l := NewWithClock(clk.now)
	end := l.Model()
	clk.tick(-5 * time.Second)
	end()
	s := l.Snapshot()
	if s.Model != 0 {
		t.Errorf("model = %s, want 0 (backward clock charges nothing)", s.Model)
	}
	if s.Total != 0 {
		t.Errorf("total = %s, want 0 (ledger clock must not rewind)", s.Total)
	}
	if s.ModelCalls != 1 {
		t.Errorf("model calls = %d, want 1 (the call still counts)", s.ModelCalls)
	}
	if s.Recorded() {
		t.Error("unmeasured split must not report recorded")
	}
}

func TestOverlappingModelChargedOnce(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3300, 0)}
	l := NewWithClock(clk.now)
	a := l.Model()
	clk.tick(10 * time.Second)
	b := l.Model()
	clk.tick(10 * time.Second)
	a()
	clk.tick(10 * time.Second)
	b()
	s := l.Snapshot()
	if s.Model != 30*time.Second || s.Total != 30*time.Second || s.ModelCalls != 2 {
		t.Fatalf("model = %s total = %s calls = %d, want 30s, 30s, 2", s.Model, s.Total, s.ModelCalls)
	}
}

func TestDoubleEndIsNoOp(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3400, 0)}
	l := NewWithClock(clk.now)
	end := l.Model()
	clk.tick(5 * time.Second)
	end()
	clk.tick(5 * time.Second)
	end()
	s := l.Snapshot()
	if s.Model != 5*time.Second || s.Other != 5*time.Second || s.ModelCalls != 1 {
		t.Fatalf("model = %s other = %s calls = %d, want 5s, 5s, 1", s.Model, s.Other, s.ModelCalls)
	}
}

func TestCloseFreezesLedger(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3500, 0)}
	l := NewWithClock(clk.now)
	end := l.Model()
	clk.tick(5 * time.Second)
	end()
	before := l.Snapshot()
	l.Close()
	l.Close()
	clk.tick(time.Hour)
	afterTick := l.Snapshot()
	if !splitEqual(afterTick, before) {
		t.Fatalf("snapshot after close+tick = %+v, want %+v", afterTick, before)
	}
	endM := l.Model()
	clk.tick(10 * time.Second)
	endM()
	endV := l.Verify(KindBuild)
	clk.tick(10 * time.Second)
	endV()
	round := l.Repair(KindTests)
	endMM := l.Model()
	clk.tick(10 * time.Second)
	endMM()
	round()
	afterSpans := l.Snapshot()
	if !splitEqual(afterSpans, before) {
		t.Fatalf("spans after close must record nothing: got %+v want %+v", afterSpans, before)
	}
}

func TestSnapshotSortsKinds(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3600, 0)}
	l := NewWithClock(clk.now)
	for _, kind := range []string{KindTests, KindBuild, KindVet} {
		end := l.Verify(kind)
		clk.tick(time.Second)
		end()
	}
	for _, kind := range []string{KindTests, KindBuild} {
		round := l.Repair(kind)
		endM := l.Model()
		clk.tick(time.Second)
		endM()
		round()
	}
	s := l.Snapshot()
	if len(s.Verify) != 3 || s.Verify[0].Kind != KindBuild || s.Verify[1].Kind != KindTests || s.Verify[2].Kind != KindVet {
		t.Fatalf("verify order = %+v, want build, tests, vet", s.Verify)
	}
	if len(s.Repair) != 2 || s.Repair[0].Kind != KindBuild || s.Repair[1].Kind != KindTests {
		t.Fatalf("repair order = %+v, want build, tests", s.Repair)
	}
}

func TestRepairIgnoresOtherTime(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3700, 0)}
	l := NewWithClock(clk.now)
	round := l.Repair(KindTests)
	clk.tick(4 * time.Second)
	endM := l.Model()
	clk.tick(6 * time.Second)
	endM()
	round()
	s := l.Snapshot()
	if s.Other != 4*time.Second || s.Model != 6*time.Second {
		t.Fatalf("other = %s model = %s, want 4s and 6s", s.Other, s.Model)
	}
	if len(s.Repair) != 1 || s.Repair[0].Time != 6*time.Second {
		t.Fatalf("repair = %+v, want 6s (other excluded)", s.Repair)
	}
}

func TestRepairExcludesVerificationInsideRound(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3750, 0)}
	l := NewWithClock(clk.now)
	round := l.Repair(KindTests)
	endM := l.Model()
	clk.tick(5 * time.Second)
	endM()
	endV := l.Verify(KindTests)
	clk.tick(7 * time.Second)
	endV()
	round()
	s := l.Snapshot()
	if s.Model != 5*time.Second || s.VerifyTime() != 7*time.Second {
		t.Fatalf("model = %s verify = %s, want 5s and 7s", s.Model, s.VerifyTime())
	}
	if len(s.Repair) != 1 || s.Repair[0].Time != 5*time.Second {
		t.Fatalf("repair = %+v, want 5s (verification excluded)", s.Repair)
	}
}

func TestLineSingularAndEmptySections(t *testing.T) {
	single := Split{Total: 5 * time.Second, Model: 2 * time.Second, Tool: time.Second, Other: 2 * time.Second, ModelCalls: 1, ToolCalls: 1}
	line := single.Line()
	if !strings.Contains(line, "1 call)") {
		t.Errorf("singular line %q lacks %q", line, "1 call)")
	}
	if strings.Contains(line, "1 calls") {
		t.Errorf("singular line %q must not say calls", line)
	}
	if strings.Contains(line, "verify") || strings.Contains(line, "repair") {
		t.Errorf("line without verify/repair must omit them: %q", line)
	}
	oneRound := Split{Total: 20 * time.Second, Model: 10 * time.Second, Other: 10 * time.Second, ModelCalls: 1, Repair: []Kind{{KindTests, 1, 10 * time.Second}}}
	if line := oneRound.Line(); !strings.Contains(line, "1 round (") {
		t.Errorf("singular repair line %q lacks %q", line, "1 round (")
	}
	empty := Split{Total: time.Second, Model: time.Second, Other: 0, ModelCalls: 1}
	fields := empty.LogFields()
	if !strings.Contains(fields, "secs_total=1.0s") {
		t.Errorf("fields %q lack total", fields)
	}
	if strings.Contains(fields, "/") {
		t.Errorf("fields without kinds must omit per-kind time/count pairs: %q", fields)
	}
}

func TestShortAndSecsResolution(t *testing.T) {
	if got := short(500 * time.Millisecond); got != "0.5s" {
		t.Errorf("short 0.5s = %q", got)
	}
	if got := short(9*time.Second + 950*time.Millisecond); got != "9.9s" {
		t.Errorf("short just under ten = %q", got)
	}
	if got := short(10 * time.Second); got != "10s" {
		t.Errorf("short 10s boundary = %q, want 10s", got)
	}
	if got := short(12*time.Second + 300*time.Millisecond); got != "12s" {
		t.Errorf("short above ten rounds = %q, want 12s", got)
	}
	if got := short(8 * time.Minute); got != "8m0s" {
		t.Errorf("short 8m = %q", got)
	}
	if got := secs(0); got != "0.0s" {
		t.Errorf("secs 0 = %q", got)
	}
	if got := secs(1500 * time.Millisecond); got != "1.5s" {
		t.Errorf("secs 1.5s = %q", got)
	}
	if plural(1) != "" || plural(2) != "s" || plural(0) != "s" {
		t.Errorf("plural forms wrong")
	}
}

func TestContextEdgeCases(t *testing.T) {
	if From(nil) != nil {
		t.Error("From(nil) must be nil")
	}
	if From(context.Background()) != nil {
		t.Error("background without ledger must yield nil")
	}
	if VerifyKindFrom(context.Background()) != "" {
		t.Error("background without kind must yield empty")
	}
	if VerifyKindFrom(nil) != "" {
		t.Error("VerifyKindFrom(nil) must yield empty, not panic")
	}
	l := New()
	ctx := WithLedger(context.Background(), l)
	if From(ctx) != l {
		t.Error("ledger must ride the context")
	}
	if VerifyKindFrom(WithVerifyKind(context.Background(), KindVet)) != KindVet {
		t.Error("kind must round-trip")
	}
}

func TestNilLedgerEmptyKindSafe(t *testing.T) {
	var l *Ledger
	l.Verify("")()
	l.Repair("")()
	l.Tool()()
	l.Model()()
	l.Close()
	if l.Snapshot().Recorded() {
		t.Fatal("nil ledger must record nothing even for empty kinds")
	}
}

func splitEqual(a, b Split) bool {
	if a.Total != b.Total || a.Model != b.Model || a.Tool != b.Tool || a.Other != b.Other || a.ModelCalls != b.ModelCalls || a.ToolCalls != b.ToolCalls {
		return false
	}
	if len(a.Verify) != len(b.Verify) || len(a.Repair) != len(b.Repair) {
		return false
	}
	for i := range a.Verify {
		if a.Verify[i] != b.Verify[i] {
			return false
		}
	}
	for i := range a.Repair {
		if a.Repair[i] != b.Repair[i] {
			return false
		}
	}
	return true
}
