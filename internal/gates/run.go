package gates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/processutil"
)

// Result is one gate run.
type Result struct {
	Gate Gate
	// Node is the node the gate ran for; "" for a workspace-scoped run.
	Node string
	// Argv is the command as run, placeholders expanded.
	Argv []string
	// ExitCode is the process's exit code, or -1 when it did not run to an
	// exit (it could not start, or the context stopped it).
	ExitCode int
	// Output is combined stdout and stderr, bounded by OutputLimit.
	Output string
	// Passed is the gate's verdict: it ran and its exit code is a pass.
	Passed bool
	// Err is why the gate produced no verdict: the program would not start, or
	// the run was cancelled. A Result with Err set is unverified, not failed.
	Err      error
	Duration time.Duration
}

// Unverified reports whether the run produced no verdict.
func (r Result) Unverified() bool { return r.Err != nil }

// OutputLimit bounds the output a Result keeps. Past it the head and the tail
// are kept -- a Go test run names the package at the top and the verdict at
// the bottom -- and the middle is marked as dropped.
var OutputLimit = 1 << 20

// Run runs g in root for node ("" or "." for the root node) and returns its
// verdict. It never returns a pass it did not observe: a program that will not
// start, or a run the context stopped, comes back Unverified.
func Run(ctx context.Context, root string, g Gate, node string) Result {
	res := Result{Gate: g, ExitCode: -1}
	if g.Scope == ScopeNode {
		res.Node = node
	}
	cmd, argv, err := Command(ctx, root, g, node)
	res.Argv = argv
	if err != nil {
		res.Err = err
		return res
	}
	out := &boundedBuffer{limit: OutputLimit}
	cmd.Stdout = out
	cmd.Stderr = out

	start := time.Now()
	err = processutil.Run(cmd)
	res.Duration = time.Since(start)
	res.Output = out.String()

	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		res.Err = fmt.Errorf("stopped: %w", ctx.Err())
	case err == nil:
		res.ExitCode = 0
	case errors.As(err, &exitErr) && exitErr.Exited():
		res.ExitCode = exitErr.ExitCode()
	default:
		res.Err = err
	}
	res.Passed = res.Err == nil && g.Passed(res.ExitCode)
	return res
}

// Command is the process Run starts for g in root for node, unstarted, and
// its argv: one resolver for running a gate and for asking what running it
// would be (a test receipt is reused for a planned check only when it is
// this invocation, processutil.Identity).
func Command(ctx context.Context, root string, g Gate, node string) (*exec.Cmd, []string, error) {
	var argv []string
	if g.Scope == ScopeNode {
		argv = g.ForNode(node)
	} else {
		argv = append([]string(nil), g.Argv...)
	}
	if len(argv) == 0 {
		return nil, argv, errors.New("empty command")
	}
	cmd := build.WorkspaceCommand(ctx, root, argv, g.Env)
	return cmd, argv, nil
}

// boundedBuffer keeps the first and last limit/2 bytes written to it.
type boundedBuffer struct {
	limit   int
	head    bytes.Buffer
	tail    []byte
	dropped int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	half := b.limit / 2
	if room := half - b.head.Len(); room > 0 {
		take := min(room, len(p))
		b.head.Write(p[:take])
		p = p[take:]
	}
	if len(p) == 0 {
		return n, nil
	}
	b.tail = append(b.tail, p...)
	if over := len(b.tail) - (b.limit - half); over > 0 {
		b.dropped += over
		b.tail = append(b.tail[:0], b.tail[over:]...)
	}
	return n, nil
}

func (b *boundedBuffer) String() string {
	if b.dropped == 0 {
		return b.head.String() + string(b.tail)
	}
	return fmt.Sprintf("%s\n... [%d bytes dropped] ...\n%s", b.head.String(), b.dropped, b.tail)
}
