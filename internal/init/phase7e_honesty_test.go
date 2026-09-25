package init

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// captureInitStdout runs fn with os.Stdout redirected and returns what it
// printed.
func captureInitStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stdout = saved }()
	fn()
	_ = w.Close()
	os.Stdout = saved
	return <-done
}

// Phase 7e records tool needs and builds nothing. It used to be titled
// "Generating Project-Specific Tools", print "Generated N tools" for N
// recorded needs, file the count under a fake agent "_generated_tools" in the
// KB map, and end init with "Tools are ready to use in .nerd/tools/".
func TestRunPhase7e_RecordsNeedsAndClaimsNoTools(t *testing.T) {
	progress := make(chan InitProgress, 4)
	ini := &Initializer{config: InitConfig{Workspace: t.TempDir(), ProgressChan: progress}}
	result := &InitResult{Success: true, AgentKBs: map[string]int{}}
	profile := ProjectProfile{Name: "probe", Language: "go"}

	out := captureInitStdout(t, func() {
		ini.runPhase7eGenerateTools(context.Background(), newPhaseRunner(ini), result, "", profile)
		ini.printSummary(result, profile)
	})

	if len(result.ToolNeeds) == 0 {
		t.Fatal("a Go project recorded no tool needs")
	}
	if _, ok := result.AgentKBs["_generated_tools"]; ok {
		t.Fatal("the tool-need count is still filed as an agent knowledge base")
	}
	for _, lie := range []string{"Generated", "Generating", "ready to use"} {
		if strings.Contains(out, lie) {
			t.Errorf("phase 7e still says %q about tools it never built:\n%s", lie, out)
		}
	}
	if !strings.Contains(out, "Tool Needs Recorded:") {
		t.Errorf("the summary does not report the recorded needs:\n%s", out)
	}
	select {
	case p := <-progress:
		if strings.Contains(p.Message, "Generating") {
			t.Errorf("progress says %q", p.Message)
		}
	default:
		t.Error("phase 7e sent no progress event")
	}
}
