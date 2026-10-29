package testfacts

import (
	"encoding/json"
	"strings"
)

// testEvent mirrors the test2json TestEvent object (`go doc cmd/test2json`)
// plus the Go 1.26 build-event extension: build-output and build-fail
// actions carry ImportPath instead of Package. Time is ignored; ordering
// comes from verdicts and sorts, not clocks.
type testEvent struct {
	Action      string  `json:"Action"`
	Package     string  `json:"Package"`
	Test        string  `json:"Test"`
	Elapsed     float64 `json:"Elapsed"`
	Output      string  `json:"Output"`
	FailedBuild string  `json:"FailedBuild"`
	ImportPath  string  `json:"ImportPath"`
}

// Test2json action vocabulary, including the Go 1.26 build actions.
const (
	actionStart       = "start"
	actionRun         = "run"
	actionPause       = "pause"
	actionCont        = "cont"
	actionPass        = "pass"
	actionBench       = "bench"
	actionFail        = "fail"
	actionOutput      = "output"
	actionSkip        = "skip"
	actionBuildOut    = "build-output"
	actionBuildFail   = "build-fail"
	noTestFilesMarker = "[no test files]"
)

// decodeLine splits one stream line into either an event or a raw line.
// A line is JSON when it unmarshals into the event shape; anything else --
// a dead runner's stderr, a shell error before test2json started -- is raw
// and kept, never an error. The boolean reports whether the line decoded.
func decodeLine(line string) (testEvent, bool) {
	line = strings.TrimSuffix(line, "\r")
	var ev testEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return testEvent{}, false
	}
	return ev, true
}

// splitLines cuts accumulated output into lines. A lone trailing newline
// leaves no phantom empty line behind it; every other line, blank or not,
// is kept verbatim.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
