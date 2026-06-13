//go:build linux

package tactile

import (
	"context"
	"strings"
	"syscall"
	"testing"
)

func TestLimitedExecutorLinux_CapabilitiesAndCgroups(t *testing.T) {
	e := NewLimitedExecutorLinux(DefaultExecutorConfig())

	caps := e.Capabilities()
	if caps.Name != "limited-linux" {
		t.Errorf("Capabilities().Name=%q, want limited-linux", caps.Name)
	}
	if !caps.SupportsResourceLimits || !caps.SupportsResourceUsage {
		t.Error("limited-linux executor should advertise resource limit + usage support")
	}

	// Cgroup version reporting must agree with UsesCgroups: when cgroups are not
	// usable (typical for an unprivileged CI sandbox) the version is 0.
	if e.UsesCgroups() {
		if v := e.CgroupVersion(); v != 1 && v != 2 {
			t.Errorf("CgroupVersion=%d while UsesCgroups, want 1 or 2", v)
		}
	} else if v := e.CgroupVersion(); v != 0 {
		t.Errorf("CgroupVersion=%d while not using cgroups, want 0", v)
	}
}

func TestLimitedExecutorLinux_ExecuteFallsBackToDirect(t *testing.T) {
	e := NewLimitedExecutorLinux(DefaultExecutorConfig())

	// No Limits set -> Execute uses the DirectExecutor path and runs the command.
	cmd := Command{Binary: "echo", Arguments: []string{"linux-exec-ok"}}
	if err := e.Validate(cmd); err != nil {
		t.Fatalf("Validate(echo): %v", err)
	}
	res, err := e.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute(echo): %v", err)
	}
	if !res.Success || !strings.Contains(res.Stdout, "linux-exec-ok") {
		t.Errorf("Execute result=%+v, want success with echoed output", res)
	}
}

func TestCreateRlimits(t *testing.T) {
	// nil limits yields the common base set without panicking.
	if rl := createRlimits(nil); rl == nil {
		t.Error("createRlimits(nil) should return a (possibly empty) map, not nil")
	}

	limits := &ResourceLimits{MaxProcesses: 64}
	rl := createRlimits(limits)
	got, ok := rl[RLIMIT_NPROC]
	if !ok {
		t.Fatal("createRlimits should set RLIMIT_NPROC when MaxProcesses > 0")
	}
	if got.Cur != 64 || got.Max != 64 {
		t.Errorf("RLIMIT_NPROC=%+v, want Cur=Max=64", got)
	}
	var _ syscall.Rlimit = got // type sanity
}
