package system

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
)

func TestMockSystemKernelOwnsIngestedFacts(t *testing.T) {
	for _, method := range []string{"load", "assert", "batch"} {
		t.Run(method, func(t *testing.T) {
			m := &MockSystemKernel{}
			input := []core.Fact{{Predicate: "browser_observed_url", Args: []any{"session", "original"}}}
			var err error
			switch method {
			case "load":
				err = m.LoadFacts(input)
			case "assert":
				err = m.Assert(input[0])
			case "batch":
				err = m.AssertBatch(input)
			}
			if err != nil {
				t.Fatal(err)
			}
			input[0].Args[1] = "changed by producer"
			input[0].Predicate = "changed_predicate"
			got, err := m.Query("browser_observed_url")
			if err != nil || len(got) != 1 || got[0].Args[1] != "original" {
				t.Fatalf("producer changed stored evidence: %+v, %v", got, err)
			}
		})
	}
}

func TestMockSystemKernelQueryOwnsItsSnapshot(t *testing.T) {
	m := &MockSystemKernel{}
	if err := m.Assert(core.Fact{Predicate: "browser_observed_url", Args: []any{"session", "original"}}); err != nil {
		t.Fatal(err)
	}
	first, err := m.Query("browser_observed_url")
	if err != nil || len(first) != 1 {
		t.Fatalf("first query: %+v, %v", first, err)
	}
	second, err := m.Query("browser_observed_url")
	if err != nil || len(second) != 1 {
		t.Fatalf("second query: %+v, %v", second, err)
	}
	first[0].Args[1] = "changed by consumer"
	first[0].Predicate = "changed_predicate"
	third, err := m.Query("browser_observed_url")
	if err != nil || len(third) != 1 || third[0].Args[1] != "original" || second[0].Args[1] != "original" {
		t.Fatalf("query snapshots alias each other or storage: second=%+v third=%+v err=%v", second, third, err)
	}
}

func TestMockSystemKernelConcurrentFactProducersAndReaders(t *testing.T) {
	const writers, perWriter = 6, 128
	m := &MockSystemKernel{}
	start, done := make(chan struct{}), make(chan struct{})
	errs := make(chan error, writers+2)
	var wg sync.WaitGroup
	wg.Add(writers + 2)
	for writer := 0; writer < writers; writer++ {
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				fact := core.Fact{Predicate: "browser_fixture", Args: []any{writer, i, "complete"}}
				var err error
				switch writer % 3 {
				case 0:
					err = m.LoadFacts([]core.Fact{fact})
				case 1:
					err = m.Assert(fact)
				case 2:
					err = m.AssertBatch([]core.Fact{fact})
				}
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	for reader := 0; reader < 2; reader++ {
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				facts, err := m.Query("browser_fixture")
				if err != nil {
					errs <- err
					return
				}
				for _, f := range facts {
					if len(f.Args) != 3 || f.Args[2] != "complete" {
						errs <- fmt.Errorf("torn fact snapshot: %+v", f)
						return
					}
				}
			}
		}()
	}
	go func() { wg.Wait(); close(done) }()
	close(start)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent mock kernel operations did not finish")
	}
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	facts, err := m.Query("browser_fixture")
	if err != nil || len(facts) != writers*perWriter {
		t.Fatalf("lost facts: %d, want %d, err=%v", len(facts), writers*perWriter, err)
	}
	seen := map[[2]int]bool{}
	for _, f := range facts {
		id := [2]int{f.Args[0].(int), f.Args[1].(int)}
		if seen[id] {
			t.Fatalf("duplicated fact: %v", id)
		}
		seen[id] = true
	}
}
