//go:build !windows

package browser

import "testing"

var _ ProcessQuerier = (*unixProcessQuerier)(nil)

func TestParsePsOutput(t *testing.T) {
	got, err := parsePsOutput(" 10 9 /opt/chrome --user-data-dir=\"/tmp/a b\"\n11 10 /opt/chrome --type=renderer\n")
	if err != nil || len(got) != 2 || got[0].PPID != 9 || got[0].CommandLine != `/opt/chrome --user-data-dir="/tmp/a b"` {
		t.Fatalf("parse = %v, %v", got, err)
	}
	if _, err := parsePsOutput("10 missing chrome"); err == nil {
		t.Fatal("malformed process row accepted")
	}
}
