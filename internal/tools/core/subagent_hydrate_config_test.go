package core

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
)

// The expansion page is the installed observation policy. An absent section
// pages the defaults (60, capped at 200); a set pair is what this call
// returns, including the schema the model is shown.
//
// Not parallel: the installed policy is process-wide.
func TestSubagentExpand_HydrateBoundsFollowInstalledPolicy(t *testing.T) {
	t.Cleanup(func() { config.SetObservationLimits(config.DefaultObservationConfig().Resolve()) })

	config.SetObservationLimits(config.DefaultObservationConfig().Resolve())
	handle := seedBoundTranscript(t, 250)

	out, err := executeSubagentExpand(context.Background(), map[string]any{"handle": handle})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if got := countBoundLines(out); got != 60 {
		t.Fatalf("default page = %d lines, want 60", got)
	}
	out, err = executeSubagentExpand(context.Background(), map[string]any{"handle": handle, "max_lines": 10000})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if got := countBoundLines(out); got != 200 {
		t.Fatalf("cap = %d lines, want 200", got)
	}
	if prop := SubagentExpandTool().Schema.Properties["max_lines"]; prop.Default != 60 ||
		!strings.Contains(prop.Description, "default 60") || !strings.Contains(prop.Description, "hard cap 200") {
		t.Fatalf("schema with the defaults = %#v %q", prop.Default, prop.Description)
	}

	limits := config.DefaultObservationConfig().Resolve()
	limits.SubagentHydrateMaxLines = 17
	limits.SubagentHydrateDefaultLines = 9
	config.SetObservationLimits(limits)

	out, err = executeSubagentExpand(context.Background(), map[string]any{"handle": handle})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if got := countBoundLines(out); got != 9 {
		t.Fatalf("installed default page = %d lines, want 9", got)
	}
	out, err = executeSubagentExpand(context.Background(), map[string]any{"handle": handle, "max_lines": 100})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if got := countBoundLines(out); got != 17 {
		t.Fatalf("installed cap = %d lines, want 17", got)
	}
	out, err = executeSubagentExpand(context.Background(), map[string]any{"handle": handle, "max_lines": 4})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if got := countBoundLines(out); got != 4 {
		t.Fatalf("explicit page under the cap = %d lines, want 4", got)
	}
	if prop := SubagentExpandTool().Schema.Properties["max_lines"]; prop.Default != 9 ||
		!strings.Contains(prop.Description, "default 9") || !strings.Contains(prop.Description, "hard cap 17") {
		t.Fatalf("schema did not follow the install: %#v %q", prop.Default, prop.Description)
	}
}

func seedBoundTranscript(t *testing.T, n int) string {
	t.Helper()
	seq := subagentFixtureSeq.Add(1)
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "bound-line %d %04d\n", seq, i)
	}
	result := observation.SharedSubagents().EncodeReturn(observation.Return{
		Agent:  "coder",
		Output: sb.String(),
	}, observation.ReturnLimits{})
	if result.Handle == "" {
		t.Fatal("the transcript was not retained, so the page cannot be measured")
	}
	return result.Handle
}

func countBoundLines(out string) int {
	return strings.Count(out, "bound-line ")
}
