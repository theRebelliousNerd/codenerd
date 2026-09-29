package session

import (
	"strings"
	"testing"
)

// summaryShowsBuildFailure is the repair prompt's read of a seed that
// arrived as text. Pinned against a real `go test -json` Summary.

func TestSummaryShowsBuildFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	broken := jsonTestModule(t, map[string]string{
		"calc.go":        "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n",
		"broken_test.go": "package verifyprobe\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { neverWritten() }\n",
	})
	if out := verificationOutput(parseJSONTest(t, runJSONTest(t, broken, "."))); !summaryShowsBuildFailure(out) {
		t.Errorf("a build failure's Summary does not read as one:\n%s", out)
	}
	failing := verificationOutput(parseJSONTest(t, runJSONTest(t, jsonTestModule(t, failingModule()), ".")))
	if summaryShowsBuildFailure(failing) {
		t.Errorf("an ordinary test failure reads as a build failure:\n%s", failing)
	}
	if summaryShowsBuildFailure("") {
		t.Error("empty output must not read as a build failure")
	}
}

func TestVerificationOutput_RepeatFloodIsOneLine(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	dir := jsonTestModule(t, map[string]string{
		"flood_test.go": "package verifyprobe\n\nimport (\n\t\"fmt\"\n\t\"testing\"\n)\n\n" +
			"func TestFlood(t *testing.T) {\n" +
			"\tfor i := 0; i < 2000; i++ { fmt.Println(\"flood-line\") }\n" +
			"\tt.Fatal(\"flood done\")\n}\n",
	})
	stream := runJSONTest(t, dir, ".")
	if len(stream) < 20000 {
		t.Fatalf("the flood fixture printed %d bytes; want a stream big enough to matter", len(stream))
	}
	out := verificationOutput(parseJSONTest(t, stream))
	if n := strings.Count(out, "flood-line"); n != 1 {
		t.Errorf("the flood line appears %d times in %d bytes; want exactly once with a count", n, len(out))
	}
	if !strings.Contains(out, "flood done") {
		t.Errorf("the failure message was lost in the flood:\n%s", out)
	}
	if len(out) >= len(stream)/10 {
		t.Errorf("compact output is %d bytes of a %d-byte stream; want the flood collapsed", len(out), len(stream))
	}
}
