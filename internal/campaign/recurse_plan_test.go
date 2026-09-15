package campaign

import (
	"strings"
	"testing"
)

func TestWaveAngles_RotateWithoutOverlap(t *testing.T) {
	seenPrimary := map[RecurseAngle]bool{}
	seenPairs := map[[2]RecurseAngle]bool{}
	var prevP, prevS RecurseAngle
	for wave := 0; wave < 12; wave++ {
		p, s := WaveAngles(wave)
		seenPrimary[p] = true
		seenPairs[[2]RecurseAngle{p, s}] = true
		if wave > 0 && (p == prevP || p == prevS || s == prevP || s == prevS) {
			t.Fatalf("wave %d (%s+%s) shares an angle with wave %d (%s+%s)",
				wave, p, s, wave-1, prevP, prevS)
		}
		prevP, prevS = p, s
	}
	if len(seenPrimary) != len(RecurseAngles()) {
		t.Fatalf("rotation covers %d primaries, want all %d", len(seenPrimary), len(RecurseAngles()))
	}
	if len(seenPairs) != 6 {
		t.Fatalf("rotation has %d distinct pairs, want 6 (no repeats per cycle)", len(seenPairs))
	}
}

func TestParseRecurseAngle(t *testing.T) {
	for _, a := range RecurseAngles() {
		if got, err := ParseRecurseAngle(string(a)); err != nil || got != a {
			t.Fatalf("ParseRecurseAngle(%q) = %q, %v", a, got, err)
		}
	}
	if _, err := ParseRecurseAngle("vibes"); err == nil {
		t.Fatal("unknown angle must fail closed")
	}
}

func TestRecurseConfig_Normalize(t *testing.T) {
	bad := []RecurseConfig{
		{MaxWaves: -1},
		{Angles: []RecurseAngle{"harden", "wire", "review"}},
		{Angles: []RecurseAngle{"vibes"}},
		{StallWaveLimit: -1},
	}
	for i, cfg := range bad {
		if _, err := cfg.Normalize(); err == nil {
			t.Errorf("case %d must fail: %+v", i, cfg)
		}
	}
	cfg, err := RecurseConfig{}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StallWaveLimit != DefaultRecurseStallWaves {
		t.Fatalf("default stall limit = %d, want %d", cfg.StallWaveLimit, DefaultRecurseStallWaves)
	}
}

func TestNewRecurseCampaign_CoversDAGInOrder(t *testing.T) {
	c, err := NewRecurseCampaign(t.TempDir(), RecurseConfig{})
	if err != nil {
		t.Fatalf("NewRecurseCampaign: %v", err)
	}
	nodes, err := TopoOrder(RecurseDAG())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Phases) != len(nodes) {
		t.Fatalf("phases = %d, want %d (whole DAG)", len(c.Phases), len(nodes))
	}
	if c.Type != CampaignTypeRecurse {
		t.Fatalf("type = %s, want /recurse", c.Type)
	}
	if c.RecurseID == "" || c.RecurseWave != 0 {
		t.Fatalf("wave zero must carry an ID and wave 0: id=%q wave=%d", c.RecurseID, c.RecurseWave)
	}
	byPhase := map[string]int{}
	for i, p := range c.Phases {
		byPhase[p.ID] = i
		node := recurseNodeOfPhase(p.Name)
		if node == "" {
			t.Fatalf("phase %q does not name its node", p.Name)
		}
		if len(p.Tasks) != 2 {
			t.Fatalf("phase %q has %d tasks, want 2 (one per angle)", p.Name, len(p.Tasks))
		}
		if len(p.Tasks[1].ContextFrom) != 1 || p.Tasks[1].ContextFrom[0] != p.Tasks[0].ID {
			t.Errorf("phase %q: secondary task must read the primary's result", p.Name)
		}
		if i > 0 && (len(p.Tasks[0].ContextFrom) != 1 || p.Tasks[0].ContextFrom[0] != c.Phases[i-1].Tasks[1].ID) {
			t.Errorf("phase %q: primary task must read the previous phase's finding", p.Name)
		}
	}
	// Hard DAG edges between phases.
	for i, n := range nodes {
		for _, dep := range n.DependsOn {
			depPhase := ""
			for _, p := range c.Phases {
				if recurseNodeOfPhase(p.Name) == dep {
					depPhase = p.ID
				}
			}
			found := false
			for _, d := range c.Phases[i].Dependencies {
				if d.DependsOnPhaseID == depPhase && d.Type == DepHard {
					found = true
				}
			}
			if !found {
				t.Errorf("phase %q missing hard edge on %q", c.Phases[i].Name, dep)
			}
		}
	}
	// Wave 0 is harden+test: the suite gate proves the coverage tasks.
	for _, p := range c.Phases {
		if len(p.Checkpoints) != 1 || p.Checkpoints[0].Type != string(VerifyTestsPass) {
			t.Fatalf("phase %q checkpoint = %+v, want one /tests_pass", p.Name, p.Checkpoints)
		}
	}
}

func TestNewRecurseCampaign_TestWaveGatesOnSuite(t *testing.T) {
	c, err := NewRecurseCampaign(t.TempDir(), RecurseConfig{Angles: []RecurseAngle{"test", "bench"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range c.Phases {
		if len(p.Checkpoints) != 1 || p.Checkpoints[0].Type != string(VerifyTestsPass) {
			t.Fatalf("phase %q checkpoint = %+v, want /tests_pass for a test wave", p.Name, p.Checkpoints)
		}
	}
	if c.TotalTasks != len(c.Phases)*2 {
		t.Fatalf("TotalTasks = %d, want %d", c.TotalTasks, len(c.Phases)*2)
	}
}

func TestSummarizeWave_CountsAndOrdersFailures(t *testing.T) {
	c := &Campaign{Phases: []Phase{
		{Name: "recurse:cli:CLI", Tasks: []Task{
			{Status: TaskCompleted}, {Status: TaskFailed}, {Status: TaskFailed},
		}},
		{Name: "recurse:kernel:Kernel", Tasks: []Task{
			{Status: TaskCompleted}, {Status: TaskFailed},
		}},
		{Name: "recurse:mangle:Mangle", Tasks: []Task{{Status: TaskCompleted}}},
	}}
	f := SummarizeWave(c)
	if f.Completed != 3 || f.Failed != 3 {
		t.Fatalf("counts = %+v, want 3 completed 3 failed", f)
	}
	if len(f.FailedNodes) != 2 || f.FailedNodes[0] != "cli" || f.FailedNodes[1] != "kernel" {
		t.Fatalf("failed nodes = %v, want [cli kernel] (most failures first)", f.FailedNodes)
	}
}

func TestPlanNextWave_RetargetsFailuresAndRotates(t *testing.T) {
	prev, err := NewRecurseCampaign(t.TempDir(), RecurseConfig{})
	if err != nil {
		t.Fatal(err)
	}
	prev.RecurseID = "/recurse_test"
	// Fail everything in the world phase, complete everything else. world
	// has one dependency (store), so retargeting has room to move it.
	for i := range prev.Phases {
		for j := range prev.Phases[i].Tasks {
			prev.Phases[i].Tasks[j].Status = TaskCompleted
			if recurseNodeOfPhase(prev.Phases[i].Name) == "world" {
				prev.Phases[i].Tasks[j].Status = TaskFailed
			}
		}
	}
	next, err := PlanNextWave(t.TempDir(), RecurseConfig{}, prev)
	if err != nil {
		t.Fatalf("PlanNextWave: %v", err)
	}
	if next.RecurseID != prev.RecurseID || next.RecurseWave != 1 {
		t.Fatalf("linkage broken: id=%q wave=%d", next.RecurseID, next.RecurseWave)
	}
	pos := map[string]int{}
	for i, p := range next.Phases {
		pos[recurseNodeOfPhase(p.Name)] = i
	}
	prevPos := -1
	zero, _ := NewRecurseCampaign(t.TempDir(), RecurseConfig{})
	for i, p := range zero.Phases {
		if recurseNodeOfPhase(p.Name) == "world" {
			prevPos = i
		}
	}
	if pos["world"] >= prevPos {
		t.Fatalf("world did not move earlier: wave0=%d wave1=%d", prevPos, pos["world"])
	}
	// The whole retargeted order must still be a valid topo order.
	nodes := RecurseDAG()
	byID := map[string]SubsystemNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	for id, at := range pos {
		for _, dep := range byID[id].DependsOn {
			if pos[dep] >= at {
				t.Fatalf("retarget broke the DAG: %s at %d, %s at %d", dep, pos[dep], id, at)
			}
		}
	}
	if !strings.Contains(next.Goal, "world") || !strings.Contains(next.Goal, "failed") {
		t.Fatalf("goal does not record the retarget: %q", next.Goal)
	}
	// Angles rotated: wave 0 is harden+test, wave 1 must share neither.
	if strings.Contains(next.Title, "harden") || strings.Contains(next.Title, "test") {
		t.Fatalf("wave 1 repeats wave 0 angles: %q", next.Title)
	}
}

func TestPlanNextWave_RequiresPrevWave(t *testing.T) {
	if _, err := PlanNextWave(t.TempDir(), RecurseConfig{}, nil); err == nil {
		t.Fatal("nil prev must fail")
	}
	if _, err := PlanNextWave(t.TempDir(), RecurseConfig{}, &Campaign{ID: "x"}); err == nil {
		t.Fatal("non-recurse prev must fail")
	}
}
