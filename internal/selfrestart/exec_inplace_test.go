package selfrestart

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The real exec replaces the process in place: a child test process restarts
// onto /bin/echo, which prints the arguments it was given -- the child's own
// plus the restore flag.
func TestRestart_ExecsInPlace(t *testing.T) {
	if !ExecSupported {
		t.Skip("no exec in place on this platform")
	}
	if os.Getenv("SELFRESTART_CHILD") == "1" {
		echo, err := IdentityOf("/bin/echo")
		if err != nil {
			t.Skip("no /bin/echo")
		}
		r := NewRestarter(Identity{Path: "/bin/echo", Build: "old"}, t.TempDir(), nil)
		r.BinaryChanged(echo)
		err = r.Restart(State{SessionID: "child-session"}, []string{"echo", "child-args"})
		t.Fatalf("Restart returned: %v", err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRestart_ExecsInPlace$")
	cmd.Env = append(os.Environ(), "SELFRESTART_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "child-args "+RestoreFlag+" ") {
		t.Fatalf("the exec'd process did not receive the restore flag: %q", out)
	}
}
