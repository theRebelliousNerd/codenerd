//go:build !windows

package browser

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/config"
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

func TestIsChromeExecutableName_WrapperAndBinaryNames(t *testing.T) {
	for _, name := range []string{"/opt/google/chrome/chrome", "/usr/bin/google-chrome", "google-chrome-stable", "/x/headless_shell", "/opt/microsoft/msedge/msedge"} {
		if !isChromeExecutableName(name) {
			t.Errorf("%q not recognized as a Chrome executable", name)
		}
	}
	for _, name := range []string{"", "/bin/sleep", "/usr/bin/google-chrome-helper-tool", "chromedriver"} {
		if isChromeExecutableName(name) {
			t.Errorf("%q wrongly recognized as a Chrome executable", name)
		}
	}
}

// The Google Chrome wrapper execs the real binary with argv[0] set to the
// wrapper's own path. The querier must still list that process, or Shutdown
// refuses to reap the browser it launched.
func TestUnixProcessQuerier_FindsWrapperNamedArgv0(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sleep 30", "sh", "--user-data-dir=/tmp/codenerd-argv0-probe")
	cmd.Args[0] = "/usr/bin/google-chrome"
	if err := cmd.Start(); err != nil {
		t.Skipf("start probe process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	q := newProcessQuerier(config.DefaultBrowserReaperConfig())
	processes, err := q.QueryChromeProcesses(context.Background())
	if err != nil {
		t.Fatalf("QueryChromeProcesses: %v", err)
	}
	for _, process := range processes {
		if process.PID == cmd.Process.Pid {
			if !strings.Contains(process.CommandLine, "codenerd-argv0-probe") {
				t.Fatalf("command line lost its profile argument: %q", process.CommandLine)
			}
			return
		}
	}
	t.Fatalf("process with argv[0]=/usr/bin/google-chrome (pid %d) not listed", cmd.Process.Pid)
}
