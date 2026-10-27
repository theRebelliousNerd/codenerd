//go:build !windows

package browser

import (
	"strconv"
	"strings"
	"testing"
)

var _ ProcessQuerier = (*unixProcessQuerier)(nil)

// Chrome rewrites /proc/self/cmdline into one space-joined string (no NUL
// separators). procCmdlineArgs must still recover the arguments, or the
// reconstructed command line quotes the whole string as a single argument,
// extractUserDataDir finds nothing, and every shutdown is safety-refused.
func TestProcCmdlineArgs_ChromeRewrittenCmdline(t *testing.T) {
	joined := "/opt/google/chrome/chrome --disable-breakpad --user-data-dir=/tmp/codenerd/user-data/profile-123 --type=renderer\x00\x00"
	args := procCmdlineArgs([]byte(joined))
	if len(args) != 4 {
		t.Fatalf("args = %v, want 4 split arguments", args)
	}
	var quoted []string
	for _, arg := range args {
		quoted = append(quoted, strconv.Quote(arg))
	}
	if got := extractUserDataDir(strings.Join(quoted, " ")); got != "/tmp/codenerd/user-data/profile-123" {
		t.Fatalf("extractUserDataDir = %q", got)
	}

	nul := "/opt/google/chrome/chrome\x00--user-data-dir=/tmp/a b/profile\x00"
	args = procCmdlineArgs([]byte(nul))
	if len(args) != 2 || args[1] != "--user-data-dir=/tmp/a b/profile" {
		t.Fatalf("NUL-separated args mangled: %v", args)
	}
}

func TestParsePsOutput(t *testing.T) {
	got, err := parsePsOutput(" 10 9 /opt/chrome --user-data-dir=\"/tmp/a b\"\n11 10 /opt/chrome --type=renderer\n")
	if err != nil || len(got) != 2 || got[0].PPID != 9 || got[0].CommandLine != `/opt/chrome --user-data-dir="/tmp/a b"` {
		t.Fatalf("parse = %v, %v", got, err)
	}
	if _, err := parsePsOutput("10 missing chrome"); err == nil {
		t.Fatal("malformed process row accepted")
	}
}
