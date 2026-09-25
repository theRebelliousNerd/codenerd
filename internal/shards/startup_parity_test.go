package shards

import (
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// When each system shard starts is written down twice: in its Go profile
// (StartupMode, which StartSystemShards reads) and in the kernel's
// shard_startup/2 facts (policy/system_config.mg, which the health policy
// reads -- escalation_needed(/system_health, ...) is scoped to /auto shards).
// Nothing kept them together, and they had drifted: mangle_repair started at
// every boot while the kernel had no startup row for it at all, and
// campaign_runner and legislator had none either. A shard the kernel does not
// know starts automatically is one whose silence it cannot escalate.
//
// Standing rule, mechanised: every system profile's startup mode has exactly
// the matching shard_startup row in the booted kernel, and the kernel names no
// system shard the registry does not define.
func TestSystemShardStartupModesAgreeWithTheKernel(t *testing.T) {
	sm := coreshards.NewShardManager()
	RegisterSystemShardProfiles(sm)

	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	rows, err := kernel.Query("shard_startup")
	if err != nil {
		t.Fatalf("query shard_startup: %v", err)
	}
	kernelModes := make(map[string][]string)
	for _, row := range rows {
		if len(row.Args) != 2 {
			continue
		}
		name := strings.TrimPrefix(types.ExtractString(row.Args[0]), "/")
		kernelModes[name] = append(kernelModes[name], strings.TrimPrefix(types.ExtractString(row.Args[1]), "/"))
	}

	goModes := make(map[string]string)
	for _, info := range sm.ListAvailableShards() {
		profile, ok := sm.GetProfile(info.Name)
		if !ok || profile.Type != types.ShardTypeSystem {
			continue
		}
		mode := string(profile.StartupMode)
		if mode == "" {
			mode = string(types.StartupAuto) // StartSystemShards treats an unset mode as auto
		}
		goModes[info.Name] = mode
	}
	if len(goModes) == 0 {
		t.Fatal("no system profiles were registered; this test is comparing nothing")
	}

	names := make([]string, 0, len(goModes))
	for name := range goModes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		got := kernelModes[name]
		if len(got) != 1 || got[0] != goModes[name] {
			t.Errorf("system shard %s starts %q in its Go profile, but the kernel says shard_startup %v; "+
				"add or fix the row in policy/system_config.mg", name, goModes[name], got)
		}
	}
	for name := range kernelModes {
		if _, ok := goModes[name]; !ok {
			t.Errorf("the kernel has shard_startup rows for %s, which no system profile defines", name)
		}
	}
}
