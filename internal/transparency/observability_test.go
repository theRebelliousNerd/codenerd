package transparency

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
)

// --- Drop counters -----------------------------------------------------

func TestGlassBoxBusStats_WhenSubscriberFull_ShouldCountDrops(t *testing.T) {
	t.Parallel()
	bus := NewGlassBoxEventBus()
	bus.Enable()
	ch := bus.Subscribe()
	defer bus.Close()

	// Subscriber buffer is 512; nobody drains, so overflow is guaranteed.
	for i := 0; i < 600; i++ {
		bus.EmitImmediate(GlassBoxEvent{Category: CategoryKernel, Summary: "flood"})
	}

	stats := bus.Stats()
	if stats.Dropped == 0 {
		t.Fatal("expected drops to be counted once the subscriber channel filled")
	}
	if stats.Delivered != uint64(cap(ch)) {
		t.Errorf("expected %d delivered, got %d", cap(ch), stats.Delivered)
	}
	if stats.Delivered+stats.Dropped != 600 {
		t.Errorf("delivered+dropped=%d, want 600", stats.Delivered+stats.Dropped)
	}
}

func TestToolEventBus_WhenChannelFull_ShouldCountDrops(t *testing.T) {
	t.Parallel()
	bus := NewToolEventBus()

	for i := 0; i < 60; i++ {
		bus.Emit(ToolEvent{ToolName: "exec_cmd", Success: true})
	}

	stats := bus.Stats()
	if stats.Emitted != 60 {
		t.Errorf("expected 60 emitted, got %d", stats.Emitted)
	}
	if stats.Dropped != 10 {
		t.Errorf("expected 10 drops past the 50-slot buffer, got %d", stats.Dropped)
	}
	if stats.Delivered != 50 {
		t.Errorf("expected 50 delivered, got %d", stats.Delivered)
	}
}

// --- Invariant: tool events are independent of Glass Box ---------------

func TestToolEventBus_WhenGlassBoxDisabled_ShouldStillDeliver(t *testing.T) {
	t.Parallel()
	glass := NewGlassBoxEventBus() // never enabled
	tools := NewToolEventBus()
	sub := glass.Subscribe()
	defer glass.Close()

	glass.Emit(GlassBoxEvent{Category: CategoryRouting, Summary: "should not appear"})
	glass.EmitImmediate(GlassBoxEvent{Category: CategoryRouting, Summary: "should not appear"})
	glass.Flush()
	tools.Emit(ToolEvent{ToolName: "read_file", Result: "ok", Success: true})

	select {
	case evt := <-sub:
		t.Fatalf("disabled Glass Box bus delivered an event: %+v", evt)
	default:
	}

	select {
	case evt := <-tools.Subscribe():
		if evt.ToolName != "read_file" {
			t.Fatalf("unexpected tool event: %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("tool events must flow regardless of Glass Box state")
	}
}

// --- Concurrency ------------------------------------------------------

func TestGlassBoxEventBus_WhenConcurrentEmitAndDrain_ShouldNotRace(t *testing.T) {
	t.Parallel()
	bus := NewGlassBoxEventBus()
	bus.batchWindow = time.Millisecond
	bus.Enable()

	const (
		producers = 8
		perProd   = 200
		consumers = 4
	)

	subs := make([]<-chan GlassBoxEvent, consumers)
	for i := range subs {
		subs[i] = bus.Subscribe()
	}

	done := make(chan struct{})
	var drainWG sync.WaitGroup
	for _, ch := range subs {
		drainWG.Add(1)
		go func(c <-chan GlassBoxEvent) {
			defer drainWG.Done()
			for {
				select {
				case _, ok := <-c:
					if !ok {
						return
					}
				case <-done:
					return
				}
			}
		}(ch)
	}

	var emitWG sync.WaitGroup
	for p := 0; p < producers; p++ {
		emitWG.Add(1)
		go func(p int) {
			defer emitWG.Done()
			for i := 0; i < perProd; i++ {
				evt := GlassBoxEvent{
					Category: AllCategories()[i%len(AllCategories())],
					Summary:  fmt.Sprintf("p%d-%d", p, i),
					TurnID:   i % 3,
				}
				if i%2 == 0 {
					bus.Emit(evt)
				} else {
					bus.EmitImmediate(evt)
				}
			}
		}(p)
	}

	// Concurrent readers/mutators of the same state.
	var otherWG sync.WaitGroup
	otherWG.Add(1)
	go func() {
		defer otherWG.Done()
		for i := 0; i < 100; i++ {
			_ = bus.Stats()
			bus.ClearTurn(i % 3)
			bus.SetVerbose(i%2 == 0)
			_ = bus.Categories()
		}
	}()

	emitWG.Wait()
	otherWG.Wait()
	bus.Flush()
	close(done)
	drainWG.Wait()

	if got := bus.Stats().TotalEmitted; got == 0 {
		t.Fatal("expected a non-zero emit count after the storm")
	}
	bus.Close()
}

func TestSafetyReporter_WhenConcurrentReportAndRead_ShouldNotRace(t *testing.T) {
	t.Parallel()
	r := NewSafetyReporter()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				r.ReportViolation(fmt.Sprintf("rm -rf w%d-%d", w, i), "/tmp", "permitted")
			}
		}(w)
	}
	for rdr := 0; rdr < 4; rdr++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				for _, v := range r.GetRecentViolations(5) {
					_ = r.FormatViolation(&v)
					_ = r.GetViolation(v.ID)
				}
			}
		}()
	}
	wg.Wait()

	if len(r.GetRecentViolations(0)) != r.maxHistory {
		t.Fatalf("expected the ring to be full at %d, got %d", r.maxHistory, len(r.GetRecentViolations(0)))
	}
}

func TestTransparencyManager_WhenConcurrentShardFeed_ShouldNotRace(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(&config.TransparencyConfig{
		Enabled:            true,
		ShardPhases:        true,
		SafetyExplanations: true,
		OperationSummaries: true,
	})

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				id := fmt.Sprintf("shard-%d-%d", w, i)
				tm.StartShard(id, "coder", "task")
				tm.UpdateShardPhase(id, PhaseExecuting, "running")
				tm.EndShard(id, i%7 == 0)
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = tm.GetStatus()
			_ = tm.ShardObserver().FormatExecutionSummary()
		}
	}()
	wg.Wait()

	// 400 executions were started with a 200-entry cap.
	if tracked := len(tm.ShardObserver().executions); tracked > 200 {
		t.Fatalf("execution map grew past its bound: %d entries", tracked)
	}
}

func TestShardObserver_WhenTrackedSetExceedsCap_ShouldPruneTerminalOnly(t *testing.T) {
	t.Parallel()
	obs := NewShardObserver()
	obs.maxTracked = 10

	// 5 executions stay active, 40 finish.
	for i := 0; i < 5; i++ {
		obs.StartExecution(fmt.Sprintf("active-%d", i), "coder", "task")
	}
	for i := 0; i < 40; i++ {
		id := fmt.Sprintf("done-%d", i)
		obs.StartExecution(id, "coder", "task")
		obs.EndExecution(id, false)
	}
	// One more start triggers the final prune pass.
	obs.StartExecution("last", "coder", "task")

	if got := len(obs.executions); got > 11 {
		t.Fatalf("expected pruning to bound the map near 10, got %d", got)
	}
	for i := 0; i < 5; i++ {
		if obs.GetExecution(fmt.Sprintf("active-%d", i)) == nil {
			t.Fatalf("pruning must never evict an active execution (active-%d)", i)
		}
	}
}

// --- NDJSON sink ------------------------------------------------------

func TestNDJSONSink_ShouldWriteOneJSONObjectPerEvent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "events.ndjson")
	sink, err := NewNDJSONFileSink(path)
	if err != nil {
		t.Fatalf("NewNDJSONFileSink: %v", err)
	}

	bus := NewGlassBoxEventBus()
	bus.AddSink(sink)
	bus.Enable()

	bus.EmitImmediate(GlassBoxEvent{Category: CategoryShard, Summary: "spawn", TurnID: 1, Duration: 3 * time.Millisecond})
	bus.EmitImmediate(GlassBoxEvent{Category: CategoryRouting, Summary: "exec_cmd", TurnID: 2})
	bus.Close() // closes sinks

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d: %v", len(lines), lines)
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 0 is not JSON: %v", err)
	}
	if first["category"] != "shard" || first["summary"] != "spawn" {
		t.Fatalf("unexpected first record: %v", first)
	}
	if first["duration_ms"].(float64) != 3 {
		t.Errorf("expected duration_ms=3, got %v", first["duration_ms"])
	}
}

func TestNDJSONSink_WhenTurnFiltered_ShouldExportOnlyThatTurn(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "turn.ndjson")
	sink, err := NewNDJSONFileSink(path)
	if err != nil {
		t.Fatalf("NewNDJSONFileSink: %v", err)
	}
	sink.OnlyTurn(7)

	bus := NewGlassBoxEventBus()
	bus.AddSink(sink)
	bus.Enable()
	bus.EmitImmediate(GlassBoxEvent{Category: CategoryKernel, Summary: "other", TurnID: 6})
	bus.EmitImmediate(GlassBoxEvent{Category: CategoryKernel, Summary: "wanted", TurnID: 7})
	bus.Close()

	lines := readLines(t, path)
	if len(lines) != 1 || !strings.Contains(lines[0], "wanted") {
		t.Fatalf("expected only turn 7 events, got %v", lines)
	}
}

func TestNewGlassBoxEventBus_WhenNDJSONEnvSet_ShouldAttachSink(t *testing.T) {
	// Not parallel: mutates process environment.
	path := filepath.Join(t.TempDir(), "env.ndjson")
	t.Setenv(NDJSONEventEnvVar, path)

	bus := NewGlassBoxEventBus()
	bus.Enable()
	if bus.Stats().SinkCount != 1 {
		t.Fatalf("expected the env sink to be attached, stats=%+v", bus.Stats())
	}
	bus.EmitImmediate(GlassBoxEvent{Category: CategoryControl, Summary: "headless"})
	bus.Close()

	if lines := readLines(t, path); len(lines) != 1 {
		t.Fatalf("expected 1 exported line, got %v", lines)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			lines = append(lines, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return lines
}

// --- Rule glossary audit ----------------------------------------------

// ruleHeadPattern matches a single-line Mangle rule head: `pred(...) :- …`.
var ruleHeadPattern = regexp.MustCompile(`(?m)^([a-z_][a-z0-9_]*)\([^:]*:-`)

// TestRuleGlossary_EveryEntry_ShouldExistInMangleCorpus enforces the audit that
// produced the current glossary. The previous map was keyed by the symbolic
// names in trace_logic.mg's rule_metadata facts, but DerivationNode.RuleName is
// the head predicate — so not a single key ever matched. A glossary that drifts
// off the corpus is indistinguishable from that bug, so it fails the build.
func TestRuleGlossary_EveryEntry_ShouldExistInMangleCorpus(t *testing.T) {
	t.Parallel()

	heads := map[string]bool{}
	root := filepath.Join("..", "core", "defaults")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".mg" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, m := range ruleHeadPattern.FindAllStringSubmatch(string(data), -1) {
			heads[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scanning %s: %v", root, err)
	}
	if len(heads) < 100 {
		t.Fatalf("only found %d rule heads under %s — the scan is broken, not the glossary", len(heads), root)
	}

	var missing []string
	for name := range ruleGlossary {
		if !heads[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("glossary entries are not rule heads in the Mangle corpus: %v", missing)
	}
}
