package sessionlog

import (
	"testing"
	"time"
)

func TestCheckStall_OneDumpPerStallOnlyWhileATurnIsActive(t *testing.T) {
	var reasons []string
	dump := func(reason string) (string, error) { reasons = append(reasons, reason); return "/tmp/x", nil }
	start := time.Now()

	if checkStall(start.Add(time.Hour), time.Minute, dump) {
		t.Fatal("dumped with no turn active")
	}
	end := BeginActivity()
	defer end()
	stall.last.Store(start.UnixNano())

	if checkStall(start.Add(30*time.Second), time.Minute, dump) {
		t.Fatal("dumped before the threshold")
	}
	if !checkStall(start.Add(2*time.Minute), time.Minute, dump) || len(reasons) != 1 {
		t.Fatalf("no dump past the threshold: %v", reasons)
	}
	if checkStall(start.Add(5*time.Minute), time.Minute, dump) || len(reasons) != 1 {
		t.Fatalf("one stall dumped twice: %v", reasons)
	}
	// Progress, then a new stall: a new dump.
	stall.last.Store(start.Add(6 * time.Minute).UnixNano())
	if !checkStall(start.Add(8*time.Minute), time.Minute, dump) || len(reasons) != 2 {
		t.Fatalf("a second stall was not dumped: %v", reasons)
	}
	end()
	if checkStall(start.Add(20*time.Minute), time.Minute, dump) {
		t.Fatal("dumped after the turn ended")
	}
}
