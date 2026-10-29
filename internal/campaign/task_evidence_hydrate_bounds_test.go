package campaign

import (
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
)

// evidenceProjection's liveness probe reads through subagentHydrateWindow,
// so the cap and the default it passes are the installed observation
// policy: absent is 200/60, and a set pair is what the call site hands
// the codec.
//
// Not parallel: the installed policy is process-wide.
func TestEvidenceHydrateWindow_FollowsInstalledPolicy(t *testing.T) {
	t.Cleanup(func() { config.SetObservationLimits(config.DefaultObservationConfig().Resolve()) })

	config.SetObservationLimits(config.DefaultObservationConfig().Resolve())
	w := subagentHydrateWindow(1)
	if w.Limit != 1 || w.MaxLines != 200 || w.DefaultLines != 60 {
		t.Fatalf("absent policy window = %+v, want limit 1, max 200, default 60", w)
	}

	limits := config.DefaultObservationConfig().Resolve()
	limits.SubagentHydrateMaxLines = 17
	limits.SubagentHydrateDefaultLines = 9
	config.SetObservationLimits(limits)
	w = subagentHydrateWindow(1)
	if w.Limit != 1 || w.MaxLines != 17 || w.DefaultLines != 9 {
		t.Fatalf("installed policy window = %+v, want limit 1, max 17, default 9", w)
	}

	// The probe still redeems a live handle when the page is not the leaf
	// default. A window that the codec rejected would remint on every brief.
	body := strings.Repeat("finding line that stays retained\n", 80)
	proj := evidenceProjection(evidenceRow{path: "docs/findings.md"}, []byte(body))
	if proj.Handle == "" {
		t.Fatal("a long artifact published no handle")
	}
	hydrated, err := observation.SharedSubagents().HydrateReturn(proj.Handle, subagentHydrateWindow(0))
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(hydrated.Lines) != 9 || hydrated.Total < 80 {
		t.Fatalf("page = %d of %d lines, want the installed default of 9", len(hydrated.Lines), hydrated.Total)
	}
}
