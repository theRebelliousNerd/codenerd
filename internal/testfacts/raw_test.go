package testfacts

import (
	"strings"
	"testing"
)

// A stream that is entirely non-JSON (a runner that died before test2json
// started) still yields a Result -- raw output plus unknown status -- and
// never an error that loses it.
func TestRawOnlyStream(t *testing.T) {
	stream := "go: cannot load module: network down\n" +
		"panic: runtime error before tests\n"
	res := parseString(t, "", stream)
	if res.Status != StatusUnknown {
		t.Fatalf("Status = %q, want unknown", res.Status)
	}
	if len(res.Packages) != 0 {
		t.Fatalf("packages = %+v, want none", res.Packages)
	}
	if len(res.Raw) != 2 || res.Raw[0] != "go: cannot load module: network down" {
		t.Fatalf("Raw = %q", res.Raw)
	}
	if got := res.Summary(); !strings.Contains(got, "raw lines: 2") {
		t.Errorf("summary = %q, want raw count", got)
	}
}

// Non-JSON stderr mixed into a real event stream is kept as raw output
// with no package while the events still parse around it.
func TestRawMixedWithEvents(t *testing.T) {
	dir := writeModule(t, map[string]string{"ok_test.go": passSrc})
	stream := "unexpected stderr from wrapper\n" + runGoTestJSON(t, dir, ".")
	res := parseString(t, dir, stream)
	if len(res.Raw) != 1 || res.Raw[0] != "unexpected stderr from wrapper" {
		t.Fatalf("Raw = %q", res.Raw)
	}
	if res.Status != StatusPass || len(res.Packages) != 1 {
		t.Fatalf("status = %q packages = %d, want pass/1", res.Status, len(res.Packages))
	}
}

// An empty stream degrades to the empty result, not to an error; a lone
// blank line is still one kept raw line (only the trailing newline is a
// terminator rather than a line).
func TestEmptyStream(t *testing.T) {
	res := parseString(t, "", "")
	if res.Status != StatusUnknown {
		t.Errorf("Status = %q, want unknown", res.Status)
	}
	if len(res.Raw) != 0 || len(res.Packages) != 0 {
		t.Errorf("Raw = %q packages = %d", res.Raw, len(res.Packages))
	}
	if got := res.Summary(); got != "empty result: no events parsed\n" {
		t.Errorf("summary = %q", got)
	}
	blank := parseString(t, "", "\n")
	if len(blank.Raw) != 1 || blank.Raw[0] != "" {
		t.Errorf("blank stream Raw = %q, want one empty line", blank.Raw)
	}
}
