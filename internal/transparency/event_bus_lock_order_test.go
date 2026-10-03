package transparency

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const glassBoxConcurrencyChild = "CODENERD_GLASSBOX_CONCURRENCY_CHILD"

func runGlassBoxConcurrencyChild(test *testing.T) bool {
	test.Helper()
	if os.Getenv(glassBoxConcurrencyChild) == test.Name() {
		return true
	}
	executable, err := os.Executable()
	if err != nil {
		test.Fatal(err)
	}
	watchdog, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(watchdog, executable, "-test.run=^"+test.Name()+"$", "-test.count=1")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, glassBoxConcurrencyChild+"=") && !strings.HasPrefix(entry, NDJSONEventEnvVar+"=") {
			child.Env = append(child.Env, entry)
		}
	}
	child.Env = append(child.Env, glassBoxConcurrencyChild+"="+test.Name())
	output, err := child.CombinedOutput()
	if err != nil {
		test.Fatalf("concurrency child failed: %v (watchdog: %v)\n%s", err, watchdog.Err(), output)
	}
	return false
}

func waitGlassBoxCondition(test *testing.T, description string, condition func() bool) {
	test.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			test.Fatalf("timed out waiting for %s", description)
		}
		runtime.Gosched()
	}
}

func glassBoxWorkerBlocked(symbol string) bool {
	stack := make([]byte, 1<<20)
	length := runtime.Stack(stack, true)
	for _, worker := range strings.Split(string(stack[:length]), "\n\n") {
		if strings.Contains(worker, symbol) && (strings.Contains(worker, "[sync.") || strings.Contains(worker, "[semacquire]")) {
			return true
		}
	}
	return false
}

func waitGlassBoxCompletion(test *testing.T, description string, completed <-chan struct{}) {
	test.Helper()
	select {
	case <-completed:
	case <-time.After(5 * time.Second):
		test.Fatalf("timed out waiting for %s", description)
	}
}

type glassBoxRecordingSink struct {
	mu         sync.Mutex
	events     []GlassBoxEvent
	closed     bool
	closeCalls int
	lateWrite  bool
}

func (sink *glassBoxRecordingSink) Write(event GlassBoxEvent) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.closed {
		sink.lateWrite = true
	}
	sink.events = append(sink.events, event)
}

func (sink *glassBoxRecordingSink) Close() error {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.closeCalls++
	sink.closed = true
	return nil
}

func TestGlassBoxEventBusLockOrder_WriterPendingBufferedFlush(test *testing.T) {
	if !runGlassBoxConcurrencyChild(test) {
		return
	}
	for _, mutation := range []string{"categories", "subscribe", "unsubscribe"} {
		test.Run(mutation, func(subtest *testing.T) {
			bus := NewGlassBoxEventBus()
			bus.batchWindow = time.Hour
			bus.SetCategories([]GlassBoxCategory{CategoryKernel})
			permanent := bus.Subscribe()
			var temporary <-chan GlassBoxEvent
			if mutation == "unsubscribe" {
				temporary = bus.Subscribe()
			}
			sink := &glassBoxRecordingSink{}
			bus.AddSink(sink)
			bus.Enable()
			bus.Emit(GlassBoxEvent{Category: CategoryKernel, Summary: "first"})
			bus.Emit(GlassBoxEvent{Category: CategoryPerception, Summary: "filtered"})
			bus.Emit(GlassBoxEvent{Category: CategoryKernel, Summary: "second"})
			bus.bufferMu.Lock()
			bus.mu.RLock()
			statsCompleted := make(chan struct{})
			go func() {
				_ = bus.Stats()
				close(statsCompleted)
			}()
			waitGlassBoxCondition(subtest, "Stats blocked on the actual buffer lock", func() bool {
				return glassBoxWorkerBlocked("(*GlassBoxEventBus).Stats(")
			})
			writerCompleted := make(chan struct{})
			go func() {
				switch mutation {
				case "categories":
					bus.SetCategories([]GlassBoxCategory{CategoryKernel})
				case "subscribe":
					temporary = bus.Subscribe()
				case "unsubscribe":
					bus.Unsubscribe(temporary)
				}
				close(writerCompleted)
			}()
			waitGlassBoxCondition(subtest, "writer pending behind the held bus reader", func() bool {
				if bus.mu.TryRLock() {
					bus.mu.RUnlock()
					return false
				}
				return true
			})
			flushCompleted := make(chan struct{})
			go func() {
				bus.flushLocked()
				bus.bufferMu.Unlock()
				close(flushCompleted)
			}()
			waitGlassBoxCondition(subtest, "buffered flush blocked behind the pending writer", func() bool {
				return glassBoxWorkerBlocked("(*GlassBoxEventBus).flushLocked(")
			})
			publicFlushCompleted := make(chan struct{})
			go func() {
				bus.Flush()
				close(publicFlushCompleted)
			}()
			waitGlassBoxCondition(subtest, "public Flush blocked on the actual buffer lock", func() bool {
				return glassBoxWorkerBlocked("(*GlassBoxEventBus).Flush(")
			})
			bus.mu.RUnlock()
			waitGlassBoxCompletion(subtest, "writer", writerCompleted)
			waitGlassBoxCompletion(subtest, "buffered flush", flushCompleted)
			waitGlassBoxCompletion(subtest, "Stats", statsCompleted)
			waitGlassBoxCompletion(subtest, "public Flush", publicFlushCompleted)
			stats := bus.Stats()
			expectedSubscribers := 1
			if mutation == "subscribe" {
				expectedSubscribers = 2
			}
			if stats.TotalEmitted != 2 || stats.Delivered != uint64(2*expectedSubscribers) || stats.Dropped != 0 || stats.BufferedEvents != 0 || stats.SubscriberCount != expectedSubscribers || stats.CategoryCount != 1 {
				subtest.Fatalf("buffered writer schedule lost delivery or counters: %+v", stats)
			}
			for _, expected := range []string{"first", "second"} {
				select {
				case event := <-permanent:
					if event.Summary != expected || event.Category != CategoryKernel {
						subtest.Fatalf("ordered filtered delivery = %+v, want %s", event, expected)
					}
				default:
					subtest.Fatal("buffered event missing")
				}
			}
			bus.Close()
			if len(sink.events) != 2 || sink.closeCalls != 1 || sink.lateWrite {
				subtest.Fatalf("sink events/close/late = %d/%d/%v", len(sink.events), sink.closeCalls, sink.lateWrite)
			}
		})
	}
}

func TestGlassBoxEventBusLockOrder_CloseJoinsStartedTimer(test *testing.T) {
	if !runGlassBoxConcurrencyChild(test) {
		return
	}
	bus := NewGlassBoxEventBus()
	bus.batchWindow = time.Hour
	subscriber := bus.Subscribe()
	bus.Enable()
	bus.Emit(GlassBoxEvent{Category: CategoryKernel, Summary: "timer tail"})
	bus.bufferMu.Lock()
	if !bus.flushTimer.Reset(0) {
		test.Fatal("fixture timer expired before synchronized reset")
	}
	waitGlassBoxCondition(test, "started timer blocked on buffer lock", func() bool {
		return glassBoxWorkerBlocked("(*GlassBoxEventBus).startFlushTimerLocked.func1(")
	})
	closeCompleted := make(chan struct{})
	go func() {
		bus.Close()
		close(closeCompleted)
	}()
	waitGlassBoxCondition(test, "Close blocked on buffer lock", func() bool {
		return glassBoxWorkerBlocked("(*GlassBoxEventBus).Close(")
	})
	bus.bufferMu.Unlock()
	waitGlassBoxCompletion(test, "Close and its started timer", closeCompleted)
	bus.flushWorkers.Wait()
	stats := bus.Stats()
	if stats.Enabled || stats.TotalEmitted != 1 || stats.Delivered != 1 || stats.Dropped != 0 || stats.BufferedEvents != 0 || bus.flushTimer != nil {
		test.Fatalf("timer shutdown lost its tail: %+v", stats)
	}
	if event, open := <-subscriber; !open || event.Summary != "timer tail" {
		test.Fatalf("timer tail delivery = %+v/%v", event, open)
	}
	if _, open := <-subscriber; open {
		test.Fatal("Close left subscriber open")
	}
}

func TestGlassBoxEventBusLockOrder_CloseStopsEmptyBufferTimer(test *testing.T) {
	if !runGlassBoxConcurrencyChild(test) {
		return
	}
	bus := NewGlassBoxEventBus()
	bus.batchWindow = time.Hour
	bus.Enable()
	bus.Emit(GlassBoxEvent{Category: CategoryKernel, TurnID: 7})
	bus.ClearTurn(7)
	bus.Close()
	bus.flushWorkers.Wait()
	if bus.flushTimer != nil || bus.Stats().BufferedEvents != 0 {
		test.Fatal("Close retained an owned timer after ClearTurn emptied the buffer")
	}
}

func TestGlassBoxEventBusLockOrder_ConcurrentLifecycleConservesDelivery(test *testing.T) {
	if !runGlassBoxConcurrencyChild(test) {
		return
	}
	for round := 0; round < 24; round++ {
		bus := NewGlassBoxEventBus()
		bus.batchWindow = time.Millisecond
		bus.SetCategories([]GlassBoxCategory{CategoryKernel})
		permanent := bus.Subscribe()
		sink := &glassBoxRecordingSink{}
		bus.AddSink(sink)
		bus.Enable()
		bus.Emit(GlassBoxEvent{Category: CategoryKernel, Summary: "accepted before close"})
		start := make(chan struct{})
		continueOperations := make(chan struct{})
		var continueOnce sync.Once
		var ready sync.WaitGroup
		var workers sync.WaitGroup
		var temporaryDeliveries atomic.Uint64
		operations := []func(int){
			func(step int) {
				bus.Emit(GlassBoxEvent{Category: CategoryKernel, Summary: fmt.Sprintf("buffered-%d", step)})
			},
			func(step int) {
				bus.EmitImmediate(GlassBoxEvent{Category: CategoryKernel, Summary: fmt.Sprintf("immediate-%d", step)})
			},
			func(step int) { _ = bus.Stats() },
			func(step int) {
				bus.SetCategories([]GlassBoxCategory{CategoryKernel})
				bus.ToggleCategory(CategoryPerception)
				bus.SetVerbose(step%2 == 0)
				_ = bus.Categories()
			},
			func(step int) {
				subscriber := bus.Subscribe()
				bus.Unsubscribe(subscriber)
				for range subscriber {
					temporaryDeliveries.Add(1)
				}
			},
			func(step int) { bus.Flush() },
		}
		ready.Add(len(operations))
		for _, operation := range operations {
			workers.Add(1)
			go func(operation func(int)) {
				defer workers.Done()
				<-start
				operation(0)
				ready.Done()
				<-continueOperations
				for step := 1; step < 64; step++ {
					operation(step)
				}
			}(operation)
		}
		for closer := 0; closer < 3; closer++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				ready.Wait()
				continueOnce.Do(func() { close(continueOperations) })
				bus.Close()
			}()
		}
		close(start)
		workers.Wait()
		bus.Close()
		stats := bus.Stats()
		var permanentDeliveries uint64
		identities := make(map[uint64]bool)
		for event := range permanent {
			if identities[event.ID] || event.Category != CategoryKernel {
				test.Fatalf("round %d duplicate or wrongly filtered event: %+v", round, event)
			}
			identities[event.ID] = true
			permanentDeliveries++
		}
		if stats.Enabled || stats.TotalEmitted < 3 || stats.TotalEmitted != permanentDeliveries || stats.Delivered != permanentDeliveries+temporaryDeliveries.Load() || stats.Dropped != 0 || stats.BufferedEvents != 0 || stats.SubscriberCount != 0 || stats.SinkCount != 0 {
			test.Fatalf("round %d lifecycle conservation failed: %+v, permanent=%d temporary=%d", round, stats, permanentDeliveries, temporaryDeliveries.Load())
		}
		if uint64(len(sink.events)) != stats.TotalEmitted || sink.closeCalls != 1 || sink.lateWrite {
			test.Fatalf("round %d sink lifecycle failed: events=%d closes=%d late=%v", round, len(sink.events), sink.closeCalls, sink.lateWrite)
		}
		bus.Enable()
		bus.Emit(GlassBoxEvent{Category: CategoryKernel, Summary: "after close"})
		bus.EmitImmediate(GlassBoxEvent{Category: CategoryKernel, Summary: "after close"})
		bus.Flush()
		lateSubscriber := bus.Subscribe()
		if _, open := <-lateSubscriber; open {
			test.Fatal("subscription after Close remained live")
		}
		lateSink := &glassBoxRecordingSink{}
		bus.AddSink(lateSink)
		if lateSink.closeCalls != 1 || bus.Stats().TotalEmitted != stats.TotalEmitted || bus.IsEnabled() {
			test.Fatal("closed bus admitted new events or owned resources")
		}
	}
}
