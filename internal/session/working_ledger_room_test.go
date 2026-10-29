package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	nerdconfig "codenerd/internal/config"
)

// appendOnly reports whether every turn of before is the same turn, byte for
// byte, in after: the provider can serve after's prefix from cache.
func appendOnly(before, after []string) bool {
	if len(before) > len(after) {
		return false
	}
	for i := range before {
		if before[i] != after[i] {
			return false
		}
	}
	return true
}

// A loop whose results outgrow the request's input window, under a ceiling
// that never binds, is compacted by the working policy against the window
// and keeps its cache: the budget guard never archives a result, and the
// prefix breaks once an epoch. Measured 2026-10-29 before this: the ledger
// stayed under its 800 KB ceiling while every request past the window had the
// guard archive one more result from its middle, and 19 of 32 such requests
// in one session read the cache only up to that point.
func TestWorkingLedger_AWindowBoundLoopCompactsOnceAnEpoch(t *testing.T) {
	t.Parallel()
	run := func(lowWater int) (breaks, guarded int) {
		l := newLedgerLoopConfigured(t, 1<<20, 1, func(e *Executor) {
			e.config.TokenBudget = 12000
			e.config.Working.LedgerLowWaterPercent = lowWater
		})
		var prev []string
		for n := 1; n <= 40; n++ {
			// A file of its own each round: no read covers another, so what
			// a compaction moves is chosen by age, not by supersession.
			name := fmt.Sprintf("f%02d.go", n)
			if err := os.WriteFile(filepath.Join(l.e.config.WorkspaceRoot, name), []byte("package target\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			sent := l.roundCall("read_file", map[string]any{"path": name}, strings.Repeat(fmt.Sprintf("round-%02d ", n), 250)) // 2250 bytes
			if strings.Contains(requestText(sent), "it does not fit the request") {
				guarded++
			}
			turns := make([]string, len(sent))
			for i, m := range sent {
				turns[i] = fmt.Sprintf("%+v", m)
			}
			if prev != nil && !appendOnly(prev, turns) {
				breaks++
			}
			prev = turns
		}
		return breaks, guarded
	}
	every, _ := run(100)
	epochs, guarded := run(nerdconfig.DefaultWorkingConfig().LedgerLowWaterPercent)
	t.Logf("40 rounds of 2250 bytes in a 12000-token window: %d prefix breaks shedding to the limit, %d shedding to the default low water", every, epochs)
	if guarded > 0 {
		t.Fatalf("the budget guard archived results in %d request(s); the policy's compaction against the window must keep the request inside it", guarded)
	}
	if epochs == 0 {
		t.Fatal("the loop never compacted; the window did not bind")
	}
	if epochs*3 > every {
		t.Fatalf("the default low water broke the prefix %d times against %d shedding to the limit; a compaction must leave rounds of headroom", epochs, every)
	}
}
