package campaign

import (
	"strings"
	"testing"
)

// The tests checkpoint reported "3,584 failed (0.00s)" for a stream holding two
// real failures because a plain-text build line broke JSON framing and the
// text heuristic counted every test name containing "Error" as a failure
// (campaign 149c512d, 2026-09-04). Interleaved text must be skipped, not fatal.
func TestParseGoTestJSON_SkipsInterleavedTextAndCountsHonestly(t *testing.T) {
	cr := NewCheckpointRunner(nil, nil, t.TempDir())
	stream := strings.Join([]string{
		`{"Time":"t","Action":"start","Package":"codenerd/internal/a"}`,
		`{"Time":"t","Action":"run","Package":"codenerd/internal/a","Test":"TestErrorPathIsFine"}`,
		`{"Time":"t","Action":"output","Package":"codenerd/internal/a","Test":"TestErrorPathIsFine","Output":"=== RUN   TestErrorPathIsFine\n"}`,
		`{"Time":"t","Action":"pass","Package":"codenerd/internal/a","Test":"TestErrorPathIsFine","Elapsed":0.01}`,
		`{"Time":"t","Action":"run","Package":"codenerd/internal/a","Test":"TestFailedThingReallyFails"}`,
		`{"Time":"t","Action":"output","Package":"codenerd/internal/a","Test":"TestFailedThingReallyFails","Output":"--- FAIL: TestFailedThingReallyFails (0.00s)\n"}`,
		`{"Time":"t","Action":"fail","Package":"codenerd/internal/a","Test":"TestFailedThingReallyFails","Elapsed":0.02}`,
		`{"Time":"t","Action":"fail","Package":"codenerd/internal/a","Elapsed":1.5}`,
		`# codenerd/internal/b`,
		`internal\b\x.go:5:2: undefined: nope`,
		`FAIL	codenerd/internal/b [build failed]`,
		`{"Time":"t","Action":"fail","Package":"codenerd/internal/b","Elapsed":0.001,"FailedBuild":"codenerd/internal/b"}`,
		`{"Time":"t","Action":"start","Package":"codenerd/internal/c"}`,
		`{"Time":"t","Action":"pass","Package":"codenerd/internal/c","Test":"TestOk","Elapsed":0.003}`,
		`{"Time":"t","Action":"pass","Package":"codenerd/internal/c","Elapsed":2.25}`,
	}, "\n")

	passed, failed, duration := cr.parseGoTestJSON(stream)
	if passed != 2 {
		t.Fatalf("passed = %d, want 2", passed)
	}
	// one failing test in a, plus package b which failed to build without running a test;
	// package a's own package-level fail must not be double counted.
	if failed != 2 {
		t.Fatalf("failed = %d, want 2", failed)
	}
	if duration.Seconds() < 3.7 || duration.Seconds() > 3.8 {
		t.Fatalf("duration = %v, want ~3.75s (sum of package wall times)", duration)
	}
}

func TestParseGoTestJSON_NoJSONFallsBackToHeuristic(t *testing.T) {
	cr := NewCheckpointRunner(nil, nil, t.TempDir())
	passed, failed, duration := cr.parseGoTestJSON("--- PASS: TestA (0.00s)\n--- FAIL: TestB (0.00s)\nFAIL\n")
	if passed != 1 || failed < 1 || duration != 0 {
		t.Fatalf("heuristic fallback = (%d, %d, %v), want (1, >=1, 0)", passed, failed, duration)
	}
}
