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
