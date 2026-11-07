package selfrestart

import (
	"errors"
	"os"
	"os/exec"
)

// Handover replaces this process with the program at argv0 run with argv and
// envv, for a long-running command that moves onto a rebuilt binary of itself
// and resumes from its own saved state (no session restore file: the command's
// ledger is the state).
//
// Where exec in place exists (ExecSupported) the process image is replaced:
// same PID, same terminal, and Handover returns only on failure. Elsewhere it
// starts the program with this process's standard streams, waits, and exits
// with the program's status, which is the closest a platform that cannot exec
// comes; it returns only when the program could not be started.
func Handover(argv0 string, argv, envv []string) error {
	if ExecSupported {
		return platformExec(argv0, argv, envv)
	}
	code, err := runChild(argv0, argv, envv)
	if err != nil {
		return err
	}
	os.Exit(code)
	return nil
}

// runChild runs argv0 with argv[1:] on this process's standard streams and
// returns its exit status. An error means the program could not be started or
// did not run to an exit status.
func runChild(argv0 string, argv, envv []string) (int, error) {
	cmd := exec.Command(argv0, argv[min(1, len(argv)):]...)
	cmd.Env = envv
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}
