package campaign

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/autopoiesis"
)

// TestThunderdomeRefusalReasons is the regression test for a safety gate that
// could not fail. runThunderdomeForTool returned (true, nil) unconditionally
// while RequireThunderdome defaulted to true, so every pregenerated tool was
// recorded as PassedThunderdome without a single attack being fired.
//
// The property under test is that "no verdict" refuses. A tool the arena never
// touched must not be adopted by a campaign just because generation succeeded.
func TestThunderdomeRefusalReasons(t *testing.T) {
	cases := []struct {
		name       string
		result     *autopoiesis.LoopResult
		wantRefuse bool
		wantSubstr string
	}{
		{
			name:       "nil result refuses",
			result:     nil,
			wantRefuse: true,
			wantSubstr: "verdict unavailable",
		},
		{
			name:       "arena never ran refuses",
			result:     &autopoiesis.LoopResult{Success: true, ToolName: "t"},
			wantRefuse: true,
			wantSubstr: "did not run",
		},
		{
			name: "generation success does not imply a pass",
			// The exact shape of the old bug: the loop succeeded, so the stub
			// said "passed", but ThunderdomeRan is false.
			result:     &autopoiesis.LoopResult{Success: true, ToolName: "t", ThunderdomeSurvived: true},
			wantRefuse: true,
			wantSubstr: "did not run",
		},
		{
			name:       "arena ran and killed the tool refuses",
			result:     &autopoiesis.LoopResult{ToolName: "t", ThunderdomeRan: true, ThunderdomeSurvived: false, ThunderdomeAttacks: 7},
			wantRefuse: true,
			wantSubstr: "defeated",
		},
		{
			name:       "arena ran and tool survived passes",
			result:     &autopoiesis.LoopResult{ToolName: "t", ThunderdomeRan: true, ThunderdomeSurvived: true, ThunderdomeAttacks: 7},
			wantRefuse: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := thunderdomeRefusalReasons(context.Background(), tc.result)
			if tc.wantRefuse && len(got) == 0 {
				t.Fatalf("expected a refusal, got a pass")
			}
			if !tc.wantRefuse && len(got) != 0 {
				t.Fatalf("expected a pass, got refusal: %v", got)
			}
			if tc.wantSubstr != "" && !strings.Contains(strings.Join(got, " "), tc.wantSubstr) {
				t.Fatalf("reason %q does not mention %q", got, tc.wantSubstr)
			}
		})
	}
}

// TestThunderdomeRefusalReasons_CancelledContext keeps the cancellation path the
// old stub had: a cancelled campaign must not hand back a pass.
func TestThunderdomeRefusalReasons_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := thunderdomeRefusalReasons(ctx, &autopoiesis.LoopResult{
		ThunderdomeRan: true, ThunderdomeSurvived: true, ThunderdomeAttacks: 3,
	})
	if len(got) == 0 {
		t.Fatalf("cancelled context must refuse even a surviving tool")
	}
	if !strings.Contains(got[0], "cancelled") {
		t.Fatalf("expected a cancellation reason, got %q", got[0])
	}
}

// TestDefaultPregeneratorConfig_RequiresThunderdome pins the default that made
// the old stub dangerous: the flag is on, so the gate must be real.
func TestDefaultPregeneratorConfig_RequiresThunderdome(t *testing.T) {
	if !DefaultPregeneratorConfig().RequireThunderdome {
		t.Fatal("RequireThunderdome must default to true; the adversarial gate is not opt-in")
	}
}
