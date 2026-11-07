package selfrestart

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The no-exec platforms' Handover runs the program and reports its status; a
// child test process stands in for the rebuilt binary.
func TestRunChild_ReportsExitStatusAndStartFailure(t *testing.T) {
	if os.Getenv("SELFRESTART_HANDOVER_CHILD") == "1" {
		os.Exit(7)
	}
	t.Setenv("SELFRESTART_HANDOVER_CHILD", "1")
	code, err := runChild(os.Args[0], []string{"child", "-test.run=^TestRunChild_ReportsExitStatusAndStartFailure$"}, os.Environ())
	if err != nil || code != 7 {
		t.Fatalf("runChild = %d, %v; want exit status 7", code, err)
	}
	if _, err := runChild("/nonexistent/nerd", []string{"nerd"}, nil); err == nil {
		t.Fatal("a program that cannot start must be an error, not an exit status")
	}
}

// Where exec in place exists, Handover is the process replacement itself: a
// child test process hands over to /bin/echo, which prints the arguments.
func TestHandover_ExecsInPlace(t *testing.T) {
	if !ExecSupported {
		t.Skip("no exec in place on this platform")
	}
	if os.Getenv("SELFRESTART_HANDOVER_EXEC") == "1" {
		err := Handover("/bin/echo", []string{"echo", "resumed", "--waves", "0"}, os.Environ())
		t.Fatalf("Handover returned: %v", err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestHandover_ExecsInPlace$")
	cmd.Env = append(os.Environ(), "SELFRESTART_HANDOVER_EXEC=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "resumed --waves 0") {
		t.Fatalf("the handed-over process did not receive the arguments: %q", out)
	}
}
